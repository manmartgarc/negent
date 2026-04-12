package sync

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type trackingWriteCloser struct {
	buf      []byte
	closeErr error
	closed   bool
}

func (w *trackingWriteCloser) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *trackingWriteCloser) Close() error {
	w.closed = true
	return w.closeErr
}

func TestWriteAndClose_ReturnsCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	dst := &trackingWriteCloser{closeErr: closeErr}

	err := writeAndClose(dst, func(w io.Writer) error {
		return copyToWriter(strings.NewReader("hello"), w)
	})
	if !errors.Is(err, closeErr) {
		t.Fatalf("writeAndClose error = %v, want %v", err, closeErr)
	}
	if string(dst.buf) != "hello" {
		t.Fatalf("written content = %q, want %q", string(dst.buf), "hello")
	}
}

func TestWriteAndClose_ClosesOnWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	dst := &trackingWriteCloser{}

	err := writeAndClose(dst, func(io.Writer) error {
		return writeErr
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("writeAndClose error = %v, want %v", err, writeErr)
	}
	if !dst.closed {
		t.Fatal("writeAndClose should close the writer on error")
	}
}

func TestWriteFileAtomic_PreservesDestinationOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	writeErr := errors.New("write failed")

	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeFileAtomic(path, func(dst io.Writer) error {
		if _, err := dst.Write([]byte("replacement")); err != nil {
			return err
		}
		return writeErr
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("writeFileAtomic error = %v, want %v", err, writeErr)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("destination content = %q, want original content", string(got))
	}

	matches, err := filepath.Glob(path + ".*.negent.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files should be removed, found %v", matches)
	}
}

func TestWriteFileAtomic_PreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")

	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(path, func(dst io.Writer) error {
		_, err := dst.Write([]byte("replacement"))
		return err
	}); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("file mode = %o, want %o", got, 0o755)
	}
}
