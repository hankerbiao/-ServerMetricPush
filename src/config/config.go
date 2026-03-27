package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Pushgateway  PushgatewayConfig
	NodeExporter NodeExporterConfig
	ControlPlane ControlPlaneConfig
}

type PushgatewayConfig struct {
	URL      string
	Job      string
	Instance string
	Interval int
	Timeout  int
}

type NodeExporterConfig struct {
	Path       string
	Port       int
	MetricsURL string
}

type ControlPlaneConfig struct {
	URL               string
	HeartbeatInterval int
}

func (c ControlPlaneConfig) Enabled() bool {
	return c.URL != ""
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "pushgateway.url":
			cfg.Pushgateway.URL = value
		case "pushgateway.job":
			cfg.Pushgateway.Job = value
		case "pushgateway.instance":
			cfg.Pushgateway.Instance = value
		case "pushgateway.interval":
			i, err := parseInt(key, value, index)
			if err != nil {
				return nil, err
			}
			cfg.Pushgateway.Interval = i
		case "pushgateway.timeout":
			i, err := parseInt(key, value, index)
			if err != nil {
				return nil, err
			}
			cfg.Pushgateway.Timeout = i
		case "node_exporter.path":
			cfg.NodeExporter.Path = value
		case "node_exporter.port":
			i, err := parseInt(key, value, index)
			if err != nil {
				return nil, err
			}
			cfg.NodeExporter.Port = i
		case "node_exporter.metrics_url":
			cfg.NodeExporter.MetricsURL = value
		case "control_plane.url":
			cfg.ControlPlane.URL = value
		case "control_plane.heartbeat_interval":
			i, err := parseInt(key, value, index)
			if err != nil {
				return nil, err
			}
			cfg.ControlPlane.HeartbeatInterval = i
		}
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {
	requiredFields := []struct {
		name  string
		valid bool
	}{
		{name: "pushgateway.url", valid: cfg.Pushgateway.URL != ""},
		{name: "pushgateway.job", valid: cfg.Pushgateway.Job != ""},
		{name: "pushgateway.interval", valid: cfg.Pushgateway.Interval > 0},
		{name: "pushgateway.timeout", valid: cfg.Pushgateway.Timeout > 0},
		{name: "node_exporter.path", valid: cfg.NodeExporter.Path != ""},
		{name: "node_exporter.port", valid: cfg.NodeExporter.Port > 0},
		{name: "node_exporter.metrics_url", valid: cfg.NodeExporter.MetricsURL != ""},
	}

	for _, field := range requiredFields {
		if !field.valid {
			return fmt.Errorf("缺少必填配置项: %s", field.name)
		}
	}

	if cfg.ControlPlane.Enabled() && cfg.ControlPlane.HeartbeatInterval <= 0 {
		return fmt.Errorf("缺少必填配置项: control_plane.heartbeat_interval")
	}
	if !cfg.ControlPlane.Enabled() && cfg.ControlPlane.HeartbeatInterval > 0 {
		return fmt.Errorf("缺少必填配置项: control_plane.url")
	}

	return nil
}

func parseInt(key, value string, index int) (int, error) {
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("配置项 %s 的值无效(第%d行): %w", key, index+1, err)
	}
	return i, nil
}
