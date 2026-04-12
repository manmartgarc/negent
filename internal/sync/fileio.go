package sync

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const defaultAtomicFilePerm = 0o644

func copyToWriter(src io.Reader, dst io.Writer) error {
	_, err := io.Copy(dst, src)
	return err
}

func writeAndClose(dst io.WriteCloser, write func(io.Writer) error) error {
	if err := write(dst); err != nil {
		_ = dst.Close()
		return err
	}

	return dst.Close()
}

func writeBytesAtomic(path string, data []byte) error {
	return writeFileAtomic(path, func(dst io.Writer) error {
		_, err := dst.Write(data)
		return err
	})
}

// writeFileAtomic writes path through a unique temp file, so write failures
// cannot leave a partially-written destination behind.
func writeFileAtomic(path string, write func(dst io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	perm, err := targetFilePerm(path)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.negent.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()

	if err := writeAndClose(f, write); err != nil {
		return cleanupTempFile(tmp, err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return cleanupTempFile(tmp, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return cleanupTempFile(tmp, err)
	}

	return nil
}

func cleanupTempFile(path string, err error) error {
	removeErr := os.Remove(path)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return fmt.Errorf("%w (cleanup failed: %v)", err, removeErr)
	}
	return err
}

func targetFilePerm(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().Perm(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return defaultAtomicFilePerm, nil
	}
	return 0, err
}
