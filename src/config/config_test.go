package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ParsesCompleteConfig(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `
pushgateway.url=http://pushgateway:9091
pushgateway.job=node-prod
pushgateway.instance=host-01
pushgateway.interval=30
pushgateway.timeout=15
node_exporter.path=/usr/local/bin/node_exporter
node_exporter.port=9200
node_exporter.metrics_url=http://127.0.0.1:9200/metrics
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Pushgateway.URL != "http://pushgateway:9091" {
		t.Fatalf("Pushgateway.URL = %q, want %q", cfg.Pushgateway.URL, "http://pushgateway:9091")
	}
	if cfg.Pushgateway.Job != "node-prod" {
		t.Fatalf("Pushgateway.Job = %q, want %q", cfg.Pushgateway.Job, "node-prod")
	}
	if cfg.Pushgateway.Instance != "host-01" {
		t.Fatalf("Pushgateway.Instance = %q, want %q", cfg.Pushgateway.Instance, "host-01")
	}
	if cfg.Pushgateway.Interval != 30 {
		t.Fatalf("Pushgateway.Interval = %d, want %d", cfg.Pushgateway.Interval, 30)
	}
	if cfg.Pushgateway.Timeout != 15 {
		t.Fatalf("Pushgateway.Timeout = %d, want %d", cfg.Pushgateway.Timeout, 15)
	}
	if cfg.NodeExporter.Path != "/usr/local/bin/node_exporter" {
		t.Fatalf("NodeExporter.Path = %q, want %q", cfg.NodeExporter.Path, "/usr/local/bin/node_exporter")
	}
	if cfg.NodeExporter.Port != 9200 {
		t.Fatalf("NodeExporter.Port = %d, want %d", cfg.NodeExporter.Port, 9200)
	}
	if cfg.NodeExporter.MetricsURL != "http://127.0.0.1:9200/metrics" {
		t.Fatalf("NodeExporter.MetricsURL = %q, want %q", cfg.NodeExporter.MetricsURL, "http://127.0.0.1:9200/metrics")
	}
}

func TestLoad_ReturnsErrorWhenRequiredFieldMissing(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `
pushgateway.url=http://pushgateway:9091
pushgateway.job=node-prod
pushgateway.interval=30
pushgateway.timeout=15
node_exporter.path=/usr/local/bin/node_exporter
node_exporter.port=9200
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want missing required field error")
	}
	if !strings.Contains(err.Error(), "node_exporter.metrics_url") {
		t.Fatalf("Load() error = %q, want missing node_exporter.metrics_url", err)
	}
}

func TestLoad_ParsesControlPlaneConfigWhenPresent(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `
pushgateway.url=http://pushgateway:9091
pushgateway.job=node-prod
pushgateway.instance=host-01
pushgateway.interval=30
pushgateway.timeout=15
node_exporter.path=/usr/local/bin/node_exporter
node_exporter.port=9200
node_exporter.metrics_url=http://127.0.0.1:9200/metrics
control_plane.url=http://control-plane:8080
control_plane.heartbeat_interval=30
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ControlPlane.URL != "http://control-plane:8080" {
		t.Fatalf("ControlPlane.URL = %q, want %q", cfg.ControlPlane.URL, "http://control-plane:8080")
	}
	if cfg.ControlPlane.HeartbeatInterval != 30 {
		t.Fatalf("ControlPlane.HeartbeatInterval = %d, want %d", cfg.ControlPlane.HeartbeatInterval, 30)
	}
}

func TestLoad_ReturnsErrorWhenControlPlaneConfigIncomplete(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `
pushgateway.url=http://pushgateway:9091
pushgateway.job=node-prod
pushgateway.instance=host-01
pushgateway.interval=30
pushgateway.timeout=15
node_exporter.path=/usr/local/bin/node_exporter
node_exporter.port=9200
node_exporter.metrics_url=http://127.0.0.1:9200/metrics
control_plane.url=http://control-plane:8080
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want incomplete control plane config error")
	}
	if !strings.Contains(err.Error(), "control_plane.heartbeat_interval") {
		t.Fatalf("Load() error = %q, want control_plane heartbeat validation error", err)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	return path
}
