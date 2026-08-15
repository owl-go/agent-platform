package processharness

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

type Stream string

const (
	StreamStdout Stream = "stdout"
	StreamStderr Stream = "stderr"
)

type Spec struct {
	Command        []string
	Dir            string
	Env            []string
	Stdin          io.Reader
	InlineLimit    int64
	MaxOutputBytes int64
	MaxLineBytes   int64
	GracePeriod    time.Duration
	Observer       OutputObserver
}

type OutputObserver interface {
	Observe(ctx context.Context, stream Stream, data []byte) error
}

type Output struct {
	Stream Stream
	Reader io.Reader
	Size   int64
	UTF8   bool
	Inline bool
}

type OutputSink interface {
	Store(ctx context.Context, output Output) error
}

type Result struct {
	ExitCode    int
	StdoutBytes int64
	StderrBytes int64
}

var (
	ErrOutputLimit = errors.New("process output limit exceeded")
	ErrLineLimit   = errors.New("process line limit exceeded")
)

func Run(ctx context.Context, spec Spec, sink OutputSink) (Result, error) {
	if len(spec.Command) == 0 {
		return Result{}, fmt.Errorf("process command is required")
	}

	tempDir, err := os.MkdirTemp("", "agent-runtime-output-*")
	if err != nil {
		return Result{}, fmt.Errorf("create output spool: %w", err)
	}
	defer os.RemoveAll(tempDir)

	stdout, err := os.CreateTemp(tempDir, "stdout-*")
	if err != nil {
		return Result{}, fmt.Errorf("create stdout spool: %w", err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(tempDir, "stderr-*")
	if err != nil {
		return Result{}, fmt.Errorf("create stderr spool: %w", err)
	}
	defer stderr.Close()

	cmd := exec.Command(spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	cmd.Stdin = spec.Stdin
	limit := newOutputLimit(spec.MaxOutputBytes, spec.MaxLineBytes, spec.Observer != nil)
	cmd.Stdout = limit.writer(ctx, StreamStdout, stdout, spec.Observer)
	cmd.Stderr = limit.writer(ctx, StreamStderr, stderr, spec.Observer)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start process: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var processErr error
	var runErr error
	select {
	case processErr = <-waitCh:
		if limit.exceeded() {
			runErr = limit.violation()
		} else {
			runErr = processErr
		}
	case <-ctx.Done():
		processErr = terminateProcessGroup(cmd.Process.Pid, waitCh, spec.GracePeriod)
		runErr = ctx.Err()
	case <-limit.reached():
		processErr = terminateProcessGroup(cmd.Process.Pid, waitCh, spec.GracePeriod)
		runErr = limit.violation()
	}

	result := Result{ExitCode: exitCode(processErr)}
	stdoutInfo, stdoutStatErr := stdout.Stat()
	stderrInfo, stderrStatErr := stderr.Stat()
	if stdoutStatErr != nil || stderrStatErr != nil {
		return result, errors.Join(stdoutStatErr, stderrStatErr)
	}
	result.StdoutBytes = stdoutInfo.Size()
	result.StderrBytes = stderrInfo.Size()
	if spec.MaxOutputBytes > 0 && result.StdoutBytes+result.StderrBytes > spec.MaxOutputBytes {
		return result, fmt.Errorf("%w: stdout=%d stderr=%d max=%d", ErrOutputLimit, result.StdoutBytes, result.StderrBytes, spec.MaxOutputBytes)
	}
	inlineLimit := spec.InlineLimit
	if inlineLimit == 0 {
		inlineLimit = 64 * 1024
	}
	storeCtx := context.WithoutCancel(ctx)
	_, err = publishFile(storeCtx, sink, StreamStdout, stdout, inlineLimit)
	if err != nil {
		return result, err
	}
	_, err = publishFile(storeCtx, sink, StreamStderr, stderr, inlineLimit)
	if err != nil {
		return result, err
	}
	return result, runErr
}

func terminateProcessGroup(pid int, waitCh <-chan error, gracePeriod time.Duration) error {
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return errors.Join(err, <-waitCh)
	}

	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		return err
	case <-timer.C:
		killErr := syscall.Kill(-pid, syscall.SIGKILL)
		if errors.Is(killErr, syscall.ESRCH) {
			killErr = nil
		}
		return errors.Join(killErr, <-waitCh)
	}
}

func publishFile(ctx context.Context, sink OutputSink, stream Stream, file *os.File, inlineLimit int64) (int64, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	validUTF8, err := validUTF8File(file)
	if err != nil {
		return 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if err := sink.Store(ctx, Output{
		Stream: stream,
		Reader: file,
		Size:   info.Size(),
		UTF8:   validUTF8,
		Inline: validUTF8 && info.Size() <= inlineLimit,
	}); err != nil {
		return info.Size(), err
	}
	return info.Size(), nil
}

func validUTF8File(file *os.File) (bool, error) {
	reader := bufio.NewReader(file)
	for {
		value, size, err := reader.ReadRune()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if value == utf8.RuneError && size == 1 {
			return false, nil
		}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

type outputLimit struct {
	mu           sync.Mutex
	max          int64
	maxLine      int64
	written      int64
	violationErr error
	once         sync.Once
	ch           chan struct{}
	active       bool
}

func newOutputLimit(max, maxLine int64, hasObserver bool) *outputLimit {
	return &outputLimit{max: max, maxLine: maxLine, ch: make(chan struct{}), active: max > 0 || maxLine > 0 || hasObserver}
}

func (l *outputLimit) writer(ctx context.Context, stream Stream, destination io.Writer, observer OutputObserver) io.Writer {
	if !l.active {
		return destination
	}
	return &cappedWriter{ctx: ctx, stream: stream, destination: destination, observer: observer, limit: l}
}

func (l *outputLimit) reached() <-chan struct{} {
	if !l.active {
		return nil
	}
	return l.ch
}

func (l *outputLimit) exceeded() bool {
	select {
	case <-l.ch:
		return true
	default:
		return false
	}
}

func (l *outputLimit) violation() error {
	return l.violationErr
}

func (l *outputLimit) signal(err error) {
	l.once.Do(func() {
		l.violationErr = err
		close(l.ch)
	})
}

type cappedWriter struct {
	ctx         context.Context
	stream      Stream
	destination io.Writer
	observer    OutputObserver
	limit       *outputLimit
	lineBytes   int64
}

func (w *cappedWriter) Write(data []byte) (int, error) {
	return w.limit.write(w, data)
}

func (l *outputLimit) write(writer *cappedWriter, data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if writer.observer != nil {
		if err := writer.observer.Observe(writer.ctx, writer.stream, data); err != nil {
			l.signal(err)
			return 0, err
		}
	}

	if l.maxLine > 0 {
		lineBytes := writer.lineBytes
		for _, value := range data {
			if value == '\n' {
				lineBytes = 0
				continue
			}
			lineBytes++
			if lineBytes > l.maxLine {
				l.signal(ErrLineLimit)
				return 0, ErrLineLimit
			}
		}
		writer.lineBytes = lineBytes
	}
	if l.max > 0 {
		remaining := l.max - l.written
		if remaining <= 0 {
			l.signal(ErrOutputLimit)
			return 0, ErrOutputLimit
		}
		if int64(len(data)) > remaining {
			data = data[:remaining]
			l.signal(ErrOutputLimit)
		}
	}
	written, err := writer.destination.Write(data)
	l.written += int64(written)
	if err != nil {
		return written, err
	}
	if written < len(data) {
		return written, io.ErrShortWrite
	}
	if l.exceeded() {
		return written, ErrOutputLimit
	}
	return written, nil
}
