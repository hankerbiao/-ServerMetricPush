package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"node-push-exporter/src/config"
	"node-push-exporter/src/controlplane"
	"node-push-exporter/src/pusher"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

const defaultConfigPath = "./config.yml"

type nodeExporterProcess struct {
	cmd *exec.Cmd
}

type controlPlaneRuntimeState struct {
	mu                sync.Mutex
	lastPushAt        time.Time
	lastPushSuccessAt time.Time
	lastPushErrorAt   time.Time
	pushFailCount     int
	lastError         string
	nodeExporterUp    bool
}

func newControlPlaneRuntimeState() *controlPlaneRuntimeState {
	return &controlPlaneRuntimeState{nodeExporterUp: true}
}

func (s *controlPlaneRuntimeState) recordFailure(err error, nodeExporterUp bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.lastPushAt = now
	s.lastPushErrorAt = now
	s.pushFailCount++
	s.lastError = err.Error()
	s.nodeExporterUp = nodeExporterUp
}

func (s *controlPlaneRuntimeState) RecordFetchFailure(err error) {
	s.recordFailure(err, false)
}

func (s *controlPlaneRuntimeState) RecordPushFailure(err error) {
	s.recordFailure(err, true)
}

func (s *controlPlaneRuntimeState) RecordPushSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.lastPushAt = now
	s.lastPushSuccessAt = now
	s.pushFailCount = 0
	s.lastError = ""
	s.nodeExporterUp = true
}

func (s *controlPlaneRuntimeState) Snapshot(agentID string) controlplane.HeartbeatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "online"
	if !s.nodeExporterUp || s.pushFailCount > 0 || s.lastError != "" {
		status = "degraded"
	}

	return controlplane.HeartbeatRequest{
		AgentID:           agentID,
		Status:            status,
		LastError:         s.lastError,
		LastPushAt:        cloneTimePointer(s.lastPushAt),
		LastPushSuccessAt: cloneTimePointer(s.lastPushSuccessAt),
		LastPushErrorAt:   cloneTimePointer(s.lastPushErrorAt),
		PushFailCount:     s.pushFailCount,
		NodeExporterUp:    s.nodeExporterUp,
	}
}

func main() {
	configPath := flag.String("config", defaultConfigPath, "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("node-push-exporter version %s (构建时间: %s)\n", version, buildTime)
		fmt.Printf("使用 node_exporter 采集指标\n")
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("启动 node-push-exporter 版本 %s", version)
	log.Printf("Pushgateway地址: %s", cfg.Pushgateway.URL)
	log.Printf("任务名称: %s, 推送间隔: %d秒", cfg.Pushgateway.Job, cfg.Pushgateway.Interval)

	pushInstance := effectivePushInstance(cfg.Pushgateway.Instance)
	if pushInstance != "" {
		log.Printf("Pushgateway实例标识: %s", pushInstance)
	}

	exporter, err := startNodeExporter(cfg.NodeExporter.Path, cfg.NodeExporter.Port)
	if err != nil {
		log.Fatalf("启动 node_exporter 失败: %v", err)
	}
	defer exporter.Stop()

	log.Printf("node_exporter 已启动，地址: %s", cfg.NodeExporter.MetricsURL)

	pusherClient := pusher.NewPusher(
		cfg.Pushgateway.URL,
		pusher.WithJob(cfg.Pushgateway.Job),
		pusher.WithInstance(pushInstance),
		pusher.WithTimeout(time.Duration(cfg.Pushgateway.Timeout)*time.Second),
	)

	ticker := time.NewTicker(time.Duration(cfg.Pushgateway.Interval) * time.Second)
	defer ticker.Stop()

	runtimeState := newControlPlaneRuntimeState()
	var controlPlaneClient *controlplane.Client
	var registerRequest controlplane.RegisterRequest
	var heartbeatCh <-chan time.Time
	registered := false
	if cfg.ControlPlane.Enabled() {
		controlPlaneClient = controlplane.NewClient(
			cfg.ControlPlane.URL,
			time.Duration(cfg.Pushgateway.Timeout)*time.Second,
		)
		registerRequest = buildRegisterRequest(cfg)
		if err := controlPlaneClient.Register(registerRequest); err != nil {
			log.Printf("控制面注册失败，后续会继续重试: %v", err)
		} else {
			registered = true
			log.Printf("控制面注册成功，节点ID: %s", registerRequest.AgentID)
		}

		heartbeatTicker := time.NewTicker(time.Duration(cfg.ControlPlane.HeartbeatInterval) * time.Second)
		defer heartbeatTicker.Stop()
		heartbeatCh = heartbeatTicker.C
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	time.Sleep(2 * time.Second)

	if err := fetchAndPush(cfg.NodeExporter.MetricsURL, pusherClient, runtimeState); err != nil {
		log.Printf("首次推送失败: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := fetchAndPush(cfg.NodeExporter.MetricsURL, pusherClient, runtimeState); err != nil {
				log.Printf("推送失败: %v", err)
			}
		case <-heartbeatCh:
			if controlPlaneClient == nil {
				continue
			}

			if !registered {
				if err := controlPlaneClient.Register(registerRequest); err != nil {
					log.Printf("控制面注册重试失败: %v", err)
					continue
				}
				registered = true
				log.Printf("控制面注册成功，节点ID: %s", registerRequest.AgentID)
			}

			if err := controlPlaneClient.Heartbeat(runtimeState.Snapshot(registerRequest.AgentID)); err != nil {
				if apiErr, ok := err.(*controlplane.APIError); ok && apiErr.StatusCode == http.StatusNotFound {
					registered = false
					log.Printf("控制面心跳返回节点不存在，下次周期重新注册: %v", err)
					continue
				}
				log.Printf("控制面心跳失败: %v", err)
				continue
			}
		case sig := <-sigChan:
			log.Printf("收到信号 %v, 正在关闭...", sig)
			return
		}
	}
}
func startNodeExporter(executablePath string, port int) (*nodeExporterProcess, error) {
	resolvedPath, err := resolveNodeExporterPath(executablePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(resolvedPath,
		fmt.Sprintf("--web.listen-address=:%d", port),
		"--web.telemetry-path=/metrics",
		"--collector.cpu",
		"--collector.meminfo",
		"--collector.diskstats",
		"--collector.netdev",
		"--collector.filesystem",
		"--collector.loadavg",
		"--collector.stat",
		"--collector.time",
		"--collector.uname",
	)

	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 node_exporter 失败: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	if err := waitForNodeExporterReady(port, waitCh, &stderr); err != nil {
		return nil, err
	}

	log.Printf("node_exporter 进程已启动，PID: %d", cmd.Process.Pid)

	return &nodeExporterProcess{
		cmd: cmd,
	}, nil
}

func resolveNodeExporterPath(executablePath string) (string, error) {
	if strings.Contains(executablePath, string(os.PathSeparator)) {
		if _, err := os.Stat(executablePath); err == nil {
			return executablePath, nil
		}
	}

	localPath := filepath.Join(".", executablePath)
	if _, err := os.Stat(localPath); err == nil {
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return "", fmt.Errorf("解析当前目录下的 node_exporter 路径失败: %w", err)
		}
		return absPath, nil
	}

	if resolvedPath, err := exec.LookPath(executablePath); err == nil {
		return resolvedPath, nil
	}

	possiblePaths := []string{
		"/usr/local/bin/node_exporter",
		"/usr/bin/node_exporter",
		"/opt/node_exporter/node_exporter",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到 node_exporter，请先安装: https://github.com/prometheus/node_exporter")
}

func waitForNodeExporterReady(port int, waitCh <-chan error, stderr *bytes.Buffer) error {
	deadline := time.Now().Add(3 * time.Second)
	metricsURL := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
	client := &http.Client{Timeout: 200 * time.Millisecond}

	for time.Now().Before(deadline) {
		select {
		case err := <-waitCh:
			return formatNodeExporterStartError("node_exporter 启动后立即退出", err, stderr)
		default:
		}

		resp, err := client.Get(metricsURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	select {
	case err := <-waitCh:
		return formatNodeExporterStartError("node_exporter 启动后立即退出", err, stderr)
	default:
	}

	stderrOutput := strings.TrimSpace(stderr.String())
	if stderrOutput != "" {
		return fmt.Errorf("node_exporter 启动超时，%s 内未能在端口 %d 提供 /metrics，stderr: %s", 3*time.Second, port, stderrOutput)
	}

	return fmt.Errorf("node_exporter 启动超时，%s 内未能在端口 %d 提供 /metrics", 3*time.Second, port)
}

func formatNodeExporterStartError(prefix string, err error, stderr *bytes.Buffer) error {
	stderrOutput := strings.TrimSpace(stderr.String())
	if stderrOutput != "" {
		return fmt.Errorf("%s: %w, stderr: %s", prefix, err, stderrOutput)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func (p *nodeExporterProcess) Stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		log.Printf("停止 node_exporter 进程，PID: %d", p.cmd.Process.Pid)
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
}
func fetchAndPush(metricsURL string, pusher *pusher.Pusher, runtimeState *controlPlaneRuntimeState) error {
	metrics, err := fetchMetrics(metricsURL)
	if err != nil {
		if runtimeState != nil {
			runtimeState.RecordFetchFailure(err)
		}
		return fmt.Errorf("获取指标失败: %w", err)
	}

	// 推送到 Pushgateway
	if err := pusher.Push([]byte(metrics)); err != nil {
		if runtimeState != nil {
			runtimeState.RecordPushFailure(err)
		}
		return fmt.Errorf("推送失败: %w", err)
	}

	if runtimeState != nil {
		runtimeState.RecordPushSuccess()
	}
	log.Printf("指标推送成功，来源: %s", metricsURL)
	return nil
}
func fetchMetrics(url string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	metrics := string(body)
	if strings.TrimSpace(metrics) == "" {
		return "", fmt.Errorf("指标数据为空")
	}

	return metrics, nil
}

func buildRegisterRequest(cfg *config.Config) controlplane.RegisterRequest {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown-host"
	}

	return controlplane.RegisterRequest{
		AgentID:                buildAgentID(hostname),
		Hostname:               hostname,
		Version:                version,
		OS:                     runtime.GOOS,
		Arch:                   runtime.GOARCH,
		IP:                     detectNodeIP(),
		PushgatewayURL:         cfg.Pushgateway.URL,
		PushIntervalSeconds:    cfg.Pushgateway.Interval,
		NodeExporterPort:       cfg.NodeExporter.Port,
		NodeExporterMetricsURL: cfg.NodeExporter.MetricsURL,
		StartedAt:              time.Now().UTC(),
	}
}

func effectivePushInstance(configured string) string {
	if configured != "" {
		return configured
	}
	if ip := detectNodeIP(); ip != "" {
		return ip
	}
	hostname, _ := os.Hostname()
	return hostname
}

func buildAgentID(hostname string) string {
	seed := readMachineID()
	if seed == "" {
		seed = hostname
	}
	sum := sha256.Sum256([]byte("node-push-exporter:" + seed))
	return hex.EncodeToString(sum[:16])
}

func readMachineID() string {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func detectNodeIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip := extractIP(addr)
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}

func cloneTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

func extractIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}
