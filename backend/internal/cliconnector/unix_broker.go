package cliconnector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type UnixBrokerServer struct {
	path      string
	directory string
	listener  *net.UnixListener
	cancel    context.CancelFunc
	done      chan error
	closeOnce sync.Once
	closeErr  error
}

func StartUnixBroker(ctx context.Context, broker *Broker, socketPath string, uid, gid int) (*UnixBrokerServer, error) {
	directory := filepath.Dir(socketPath)
	if broker == nil || !filepath.IsAbs(socketPath) || directory == "/" || filepath.Base(socketPath) != "cli-broker.sock" || strings.ContainsAny(socketPath, ",\x00\r\n") || uid < 0 || gid < 0 {
		return nil, errors.New("invalid CLI broker socket configuration")
	}
	if err := prepareBrokerDirectory(directory, uid, gid); err != nil {
		return nil, err
	}
	if err := removeStaleBrokerSocket(socketPath); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		return nil, errors.New("CLI broker directory must contain only its reserved socket")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on CLI broker socket: %w", err)
	}
	cleanupListener := func(cause error) (*UnixBrokerServer, error) {
		_ = listener.Close()
		_ = os.Chmod(directory, 0o750)
		_ = os.Remove(socketPath)
		_ = os.Remove(directory)
		return nil, cause
	}
	if err := os.Chown(socketPath, uid, gid); err != nil {
		return cleanupListener(fmt.Errorf("set CLI broker socket owner: %w", err))
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		return cleanupListener(fmt.Errorf("protect CLI broker socket: %w", err))
	}
	// The Runtime only needs connect(2); a read-only bind mount prevents it
	// from replacing the server-owned socket path.
	if err := os.Chmod(directory, 0o550); err != nil {
		return cleanupListener(fmt.Errorf("protect CLI broker directory: %w", err))
	}
	serverCtx, cancel := context.WithCancel(ctx)
	server := &UnixBrokerServer{path: socketPath, directory: directory, listener: listener, cancel: cancel, done: make(chan error, 1)}
	go func() { server.done <- broker.Serve(serverCtx, listener) }()
	return server, nil
}

func (server *UnixBrokerServer) Close() error {
	if server == nil {
		return nil
	}
	server.closeOnce.Do(func() {
		server.cancel()
		_ = server.listener.Close()
		serveErr := <-server.done
		if err := os.Chmod(server.directory, 0o750); err != nil && !errors.Is(err, os.ErrNotExist) {
			server.closeErr = errors.Join(server.closeErr, fmt.Errorf("unlock CLI broker directory: %w", err))
		}
		if err := os.Remove(server.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			server.closeErr = errors.Join(server.closeErr, fmt.Errorf("remove CLI broker socket: %w", err))
		}
		if err := os.Remove(server.directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			server.closeErr = errors.Join(server.closeErr, fmt.Errorf("remove CLI broker directory: %w", err))
		}
		server.closeErr = errors.Join(server.closeErr, serveErr)
	})
	return server.closeErr
}

func prepareBrokerDirectory(directory string, uid, gid int) error {
	if err := os.Mkdir(directory, 0o750); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create CLI broker directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("CLI broker directory must be a real directory")
	}
	if err := os.Chown(directory, uid, gid); err != nil {
		return fmt.Errorf("set CLI broker directory owner: %w", err)
	}
	return os.Chmod(directory, 0o750)
}

func removeStaleBrokerSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect CLI broker socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return errors.New("reserved CLI broker path is not a socket")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale CLI broker socket: %w", err)
	}
	return nil
}
