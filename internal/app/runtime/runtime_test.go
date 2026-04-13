package runtime

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"flux/pkg/logger"
)

func TestNewDisablesConsoleOutputWhenRequested(t *testing.T) {
	tempDir := t.TempDir()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer

	defer func() {
		os.Stdout = oldStdout
		_ = reader.Close()
	}()

	rt, err := New(Options{
		Version:           "test",
		DataDir:           filepath.Join(tempDir, "data"),
		DisableConsoleLog: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		_ = rt.Close()
	}()

	logger.Info("should stay out of stdout")

	_ = writer.Close()

	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	if output.Len() != 0 {
		t.Fatalf("expected no stdout log output, got %q", output.String())
	}
}
