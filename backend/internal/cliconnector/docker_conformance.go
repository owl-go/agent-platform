package cliconnector

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DockerConformanceConfig struct {
	DockerCommand string
	Runtime       string
	RuntimeImages map[string]string
	UID, GID      int
	Timeout       time.Duration
}

type DockerConformance struct {
	config DockerConformanceConfig
	run    DockerCommand
}

func NewDockerConformance(config DockerConformanceConfig, run DockerCommand) (*DockerConformance, error) {
	if config.DockerCommand == "" || config.Runtime != "runsc" || config.UID <= 0 || config.GID <= 0 || config.Timeout <= 0 || config.Timeout > 5*time.Minute || len(config.RuntimeImages) == 0 {
		return nil, errors.New("invalid CLI Runtime Conformance configuration")
	}
	for digest, image := range config.RuntimeImages {
		if !runtimeDigest.MatchString(digest) || !imageRepoDigest.MatchString(image) || !strings.HasSuffix(image, "@"+digest) {
			return nil, errors.New("CLI Runtime Conformance requires matching pinned Runtime images")
		}
	}
	if run == nil {
		run = func(ctx context.Context, command string, arguments ...string) error {
			output, err := exec.CommandContext(ctx, command, arguments...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}
	}
	return &DockerConformance{config: config, run: run}, nil
}

func (suite *DockerConformance) Test(ctx context.Context, bundle []byte, runtimeDigest string, definition Definition) error {
	image, ok := suite.config.RuntimeImages[runtimeDigest]
	if !ok {
		return errors.New("Runtime Digest is not configured for CLI Conformance")
	}
	root, err := os.MkdirTemp("", "agent-cli-conformance-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err := extractBundle(bundle, root); err != nil {
		return err
	}
	if err := os.Chown(root, suite.config.UID, suite.config.GID); err != nil {
		return fmt.Errorf("set CLI Conformance bundle ownership: %w", err)
	}
	executable := "/opt/agent-connector/node_modules/.bin/" + definition.Executable
	execution, cancel := context.WithTimeout(ctx, suite.config.Timeout)
	defer cancel()
	arguments := []string{
		"run", "--rm", "--runtime", suite.config.Runtime,
		"--user", fmt.Sprintf("%d:%d", suite.config.UID, suite.config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", "none", "--memory", strconv.FormatInt(512<<20, 10), "--cpus", "1", "--pids-limit", "128",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=67108864",
		"--mount", "type=bind,src=" + root + ",dst=/opt/agent-connector,readonly=true",
		"--entrypoint", executable,
		"--label", "agent-platform.managed=true", "--label", "agent-platform.workload=cli-conformance",
		image, "--help",
	}
	if err := suite.run(execution, suite.config.DockerCommand, arguments...); err != nil {
		return fmt.Errorf("run CLI Conformance on %s: %w", runtimeDigest, err)
	}
	return nil
}

func extractBundle(bundle []byte, destination string) error {
	if len(bundle) == 0 || len(bundle) > maxBundleSize {
		return errors.New("invalid CLI bundle size")
	}
	compressed, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return fmt.Errorf("open CLI bundle: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var total int64
	type pendingSymlink struct{ target, link string }
	var symlinks []pendingSymlink
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read CLI bundle: %w", err)
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return errors.New("CLI bundle contains an unsafe path")
		}
		target := filepath.Join(destination, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += header.Size
			if header.Size < 0 || total > maxBundleSize {
				return errors.New("expanded CLI bundle is too large")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			link := filepath.Clean(filepath.FromSlash(header.Linkname))
			resolved := filepath.Clean(filepath.Join(filepath.Dir(name), link))
			if filepath.IsAbs(link) || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
				return errors.New("CLI bundle contains an unsafe symlink")
			}
			symlinks = append(symlinks, pendingSymlink{target: target, link: link})
		default:
			return errors.New("CLI bundle contains an unsupported entry")
		}
	}
	// Materialize links only after regular files so later archive entries cannot
	// traverse a previously created symlink outside the extraction root.
	for _, symlink := range symlinks {
		if err := os.MkdirAll(filepath.Dir(symlink.target), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(symlink.link, symlink.target); err != nil {
			return err
		}
	}
	return nil
}

var _ Conformance = (*DockerConformance)(nil)
