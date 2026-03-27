package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	if defaultConfigPath != "./config.yml" {
		t.Fatalf("defaultConfigPath = %q, want %q", defaultConfigPath, "./config.yml")
	}
}

func TestResolveNodeExporterPath_UsesCurrentDirectoryBinary(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	localBinary := filepath.Join(tempDir, "node_exporter")
	if err := os.WriteFile(localBinary, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	resolvedPath, err := resolveNodeExporterPath("node_exporter")
	if err != nil {
		t.Fatalf("resolveNodeExporterPath() error = %v", err)
	}

	resolvedInfo, err := os.Stat(resolvedPath)
	if err != nil {
		t.Fatalf("os.Stat(resolvedPath) error = %v", err)
	}
	localInfo, err := os.Stat(localBinary)
	if err != nil {
		t.Fatalf("os.Stat(localBinary) error = %v", err)
	}
	if !os.SameFile(resolvedInfo, localInfo) {
		t.Fatalf("resolveNodeExporterPath() = %q, want same file as %q", resolvedPath, localBinary)
	}
}

func TestStartNodeExporter_ReturnsErrorWhenProcessExitsImmediately(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	}()

	localBinary := filepath.Join(tempDir, "node_exporter")
	script := "#!/bin/sh\n" +
		"echo 'mock node_exporter failed to start' >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(localBinary, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	_, err = startNodeExporter("node_exporter", 9100)
	if err == nil {
		t.Fatal("startNodeExporter() error = nil, want startup failure")
	}
	if !strings.Contains(err.Error(), "mock node_exporter failed to start") {
		t.Fatalf("startNodeExporter() error = %q, want stderr output", err)
	}
}

func TestEffectivePushInstance_PrefersConfiguredValue(t *testing.T) {
	if got := effectivePushInstance("custom-instance"); got != "custom-instance" {
		t.Fatalf("effectivePushInstance() = %q, want %q", got, "custom-instance")
	}
}

func TestEffectivePushInstance_FallsBackWhenEmpty(t *testing.T) {
	got := effectivePushInstance("")
	if got == "" {
		t.Fatal("effectivePushInstance() = empty, want detected ip or hostname")
	}
}
