package conformanceartifact

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Metadata struct {
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Files   int    `json:"files"`
	Entries int    `json:"entries"`
}

func Archive(source string, destination io.Writer) (Metadata, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return Metadata{}, fmt.Errorf("resolve archive source: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Metadata{}, fmt.Errorf("inspect archive source: %w", err)
	}
	if !info.IsDir() {
		return Metadata{}, fmt.Errorf("archive source must be a directory")
	}

	digest := sha256.New()
	counter := &countingWriter{next: io.MultiWriter(destination, digest)}
	writer := tar.NewWriter(counter)
	metadata := Metadata{}
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		link := ""
		if !info.IsDir() && !info.Mode().IsRegular() && entry.Type()&os.ModeSymlink == 0 {
			return fmt.Errorf("archive entry %q has unsupported file type", relative)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
			if err := validateSymlink(relative, link); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		metadata.Entries++
		if !info.Mode().IsRegular() {
			if entry.Type()&os.ModeSymlink != 0 {
				metadata.Files++
			}
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("archive file %q: %w", relative, errors.Join(copyErr, closeErr))
		}
		metadata.Files++
		return nil
	})
	closeErr := writer.Close()
	if walkErr != nil || closeErr != nil {
		return Metadata{}, fmt.Errorf("create archive: %w", errors.Join(walkErr, closeErr))
	}
	metadata.Size = counter.count
	metadata.SHA256 = hex.EncodeToString(digest.Sum(nil))
	return metadata, nil
}

func Restore(source io.Reader, target string) error {
	root, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve restore target: %w", err)
	}
	if err := requireEmptyDirectory(root); err != nil {
		return err
	}
	reader := tar.NewReader(source)
	seen := make(map[string]struct{})
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		relative, err := cleanArchivePath(header.Name)
		if err != nil {
			return err
		}
		if _, duplicate := seen[relative]; duplicate {
			return fmt.Errorf("archive contains duplicate path %q", relative)
		}
		seen[relative] = struct{}{}
		path := filepath.Join(root, filepath.FromSlash(relative))
		mode := os.FileMode(header.Mode) & os.ModePerm
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, mode); err != nil {
				return fmt.Errorf("restore directory %q: %w", relative, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("restore file %q: %w", relative, err)
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil || closeErr != nil {
				return fmt.Errorf("restore file %q: %w", relative, errors.Join(copyErr, closeErr))
			}
		case tar.TypeSymlink:
			if err := validateSymlink(relative, header.Linkname); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, path); err != nil {
				return fmt.Errorf("restore symlink %q: %w", relative, err)
			}
		default:
			return fmt.Errorf("archive entry %q has unsupported type %d", relative, header.Typeflag)
		}
	}
}

func validateSymlink(relative, target string) error {
	if target == "" || filepath.IsAbs(target) {
		return fmt.Errorf("symlink %q has unsafe target", relative)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(relative), target))
	if resolved == ".." || strings.HasPrefix(resolved, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("symlink %q escapes archive root", relative)
	}
	return nil
}

func cleanArchivePath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\x00') || filepath.IsAbs(name) {
		return "", fmt.Errorf("archive contains unsafe path %q", name)
	}
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive contains unsafe path %q", name)
	}
	return filepath.ToSlash(cleaned), nil
}

func requireEmptyDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0o700)
	}
	if err != nil {
		return fmt.Errorf("inspect restore target: %w", err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("restore target must be empty")
	}
	return nil
}

type countingWriter struct {
	next  io.Writer
	count int64
}

func (w *countingWriter) Write(value []byte) (int, error) {
	count, err := w.next.Write(value)
	w.count += int64(count)
	return count, err
}
