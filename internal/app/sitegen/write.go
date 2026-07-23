package sitegen

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type stagingWriter struct {
	outputDir string
	staging   string
}

func newStagingWriter(outputDir string) (*stagingWriter, error) {
	parent := filepath.Dir(outputDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("create output parent directory: %w", err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(outputDir)+"-")
	if err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}
	return &stagingWriter{outputDir: outputDir, staging: staging}, nil
}

func (w *stagingWriter) WriteFile(path string, data []byte) error {
	clean, err := safeOutputPath(path)
	if err != nil {
		return err
	}
	filename := filepath.Join(w.staging, clean)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", clean, err)
	}
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write output file %s: %w", clean, err)
	}
	return nil
}

func (w *stagingWriter) CopyEmbeddedFile(source fs.FS, sourcePath, targetPath string) error {
	data, err := fs.ReadFile(source, sourcePath)
	if err != nil {
		return fmt.Errorf("read embedded asset %s: %w", sourcePath, err)
	}
	return w.WriteFile(targetPath, data)
}

func (w *stagingWriter) Commit() error {
	var backup string
	if _, err := os.Stat(w.outputDir); err == nil {
		var createErr error
		backup, createErr = os.MkdirTemp(filepath.Dir(w.outputDir), "."+filepath.Base(w.outputDir)+"-previous-")
		if createErr != nil {
			return fmt.Errorf("reserve output backup path: %w", createErr)
		}
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("prepare output backup path: %w", err)
		}
		if err := os.Rename(w.outputDir, backup); err != nil {
			return fmt.Errorf("back up current output: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect current output: %w", err)
	}
	if err := os.Rename(w.staging, w.outputDir); err != nil {
		if backup != "" {
			_ = os.Rename(backup, w.outputDir)
		}
		return fmt.Errorf("activate generated output: %w", err)
	}
	w.staging = ""
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove output backup: %w", err)
		}
	}
	return nil
}

func (w *stagingWriter) Abort() {
	if w.staging != "" {
		_ = os.RemoveAll(w.staging)
	}
}

func safeOutputPath(path string) (string, error) {
	clean := filepath.Clean(path)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe static output path %q", path)
	}
	return clean, nil
}
