# Install Script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加一个 `install.sh`，根据当前系统平台从 `binary-download-service` 自动下载并安装 `node_exporter` 与 `node-push-exporter`

**Architecture:** 在仓库根目录增加一个 Bash 安装脚本。脚本通过 `GET /api/files` 获取候选文件，结合 API 元数据和文件名回退解析匹配当前 `os/arch`，再下载 `/download/{filename}` 并提取目标二进制到指定目录。

**Tech Stack:** Bash, curl, python3, tar

---

### Task 1: 锁定安装脚本核心行为

**Files:**
- Create: `install_test.sh`

- [ ] **Step 1: 写平台识别和文件筛选测试**
- [ ] **Step 2: 运行测试，确认在 `install.sh` 缺失时失败**

### Task 2: 实现安装脚本

**Files:**
- Create: `install.sh`

- [ ] **Step 1: 实现平台识别和参数解析**
- [ ] **Step 2: 实现 `/api/files` 查询和候选文件选择**
- [ ] **Step 3: 实现下载、解压、安装和权限设置**

### Task 3: 验证

**Files:**
- Modify: `install.sh`

- [ ] **Step 1: 运行 `bash install_test.sh`**
- [ ] **Step 2: 运行 `bash -n install.sh`**
