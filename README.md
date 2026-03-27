# node-push-exporter

启动本机 `node_exporter`，抓取 `/metrics`，再把指标推到 Prometheus Pushgateway。

## 运行方式

本地调试时，程序默认读取当前目录下的 `./config.yml`。

```bash
go build -o node-push-exporter ./src
./node-push-exporter
```

也可以显式指定配置文件：

```bash
go run ./src -config ./config.yml
```

## 配置文件

配置格式是 `key=value`，默认示例就在仓库根目录 [config.yml](/Users/libiao/Documents/push_node/config.yml)。

```ini
# Pushgateway 设置
pushgateway.url=http://localhost:9091
pushgateway.job=node
# 留空时会自动使用本机 IP，Prometheus 可据此区分不同服务器
pushgateway.instance=
pushgateway.interval=60
pushgateway.timeout=10

# node_exporter 设置
node_exporter.path=node_exporter
node_exporter.port=9100
node_exporter.metrics_url=http://localhost:9100/metrics

# 主服务设置（可选）
control_plane.url=http://127.0.0.1:8080
control_plane.heartbeat_interval=30
```

必填项缺失时，程序会在启动阶段直接报错退出。
`pushgateway.instance` 留空时，程序会自动回退为本机 IPv4；如果拿不到 IPv4，则回退为 hostname。
如果未配置 `control_plane.url`，主动注册功能会自动关闭，不影响原有 Pushgateway 推送。

## 本地调试

当前目录放好这两个文件即可：

- `./node-push-exporter`
- `./node_exporter`

启动后会先确认 `node_exporter` 的 `/metrics` 已就绪，再开始首次推送。`node_exporter.path=node_exporter` 时，会按下面顺序找可执行文件：

1. 当前目录下的 `./node_exporter`
2. `PATH` 里的 `node_exporter`
3. 常见安装路径

## systemd

如果用 systemd 部署，建议显式指定系统配置路径：

```bash
sudo mkdir -p /etc/node-push-exporter
sudo cp ./config.yml /etc/node-push-exporter/config.yaml
sudo cp systemd/node-push-exporter.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable node-push-exporter
sudo systemctl start node-push-exporter
```

服务文件默认使用：

```bash
--config /etc/node-push-exporter/config.yaml
```

## 主服务节点状态

`binary-download-service` 现在也承担主服务能力，用来接收 `node-push-exporter` 的注册和心跳。节点状态页入口：

```bash
http://<binary-download-service>/agents
```

页面会展示当前节点在线状态、最近心跳、Pushgateway 推送结果、最近错误，以及对应的 node_exporter 指标地址。

## 常用命令

```bash
go test ./...
./build.sh
./node-push-exporter --version
curl http://localhost:9100/metrics
sudo journalctl -u node-push-exporter -f
sudo systemctl status node-push-exporter
```

## 发布包

执行：

```bash
./build.sh
```

会在 `./releases` 目录下生成这几个发布包：

- `node-push-exporter-linux-amd64.tar.gz`
- `node-push-exporter-linux-arm64.tar.gz`
- `node-push-exporter-linux-armv7.tar.gz`

每个压缩包都包含：

- `node-push-exporter`
- `config.yml`
- `README.md`

其他机器可以按自身架构下载对应的 `.tar.gz`，解压后直接修改 `config.yml` 并运行。

## 故障排查

`node_exporter` 启动失败时，程序现在会在启动阶段直接报错，不会再误报“进程已启动”。优先检查：

- `./node_exporter` 是否存在并有执行权限
- `node_exporter.port` 对应端口是否被占用
- `pushgateway.url` 是否可达

如果是 systemd 环境，先看：

```bash
journalctl -u node-push-exporter -n 100
```

## 许可证

MIT
