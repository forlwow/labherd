# 部署手册

## 1. 总览

| 组件 | 部署位置 | 方式 | 来源 |
|---|---|---|---|
| Controller（含网页） | 一台所有 GPU 服务器都能访问的机器 | Docker Compose | `ghcr.io/forlwow/labherd-controller:<版本>` |
| Agent | 每台 GPU 服务器 | systemd，由 Ansible 批量安装 | GitHub Releases 中的 `labherd-agent-linux-amd64` |

Agent 不放进容器：它要读取宿主机的 `/proc`、磁盘挂载和 `nvidia-smi`，容器化反而需要大量特权配置。选型理由见 [ADR 0006](../adr/0006-release-and-deploy.md)。

端口：

| 端口 | 方向 | 用途 |
|---|---|---|
| 9090/tcp | Agent → Controller | gRPC 双向流 |
| 8080/tcp | 浏览器 → Controller | 网页与 HTTP 接口 |

> M1 没有登录和传输加密，只能部署在实验室内网。

## 2. 发版

版本号规则见 `conventions.md` 7.10。在 main 的最新提交上打 tag：

```bash
git switch main && git pull
git tag v0.1.0
git push origin v0.1.0
```

`release` 工作流会自动：

1. 构建静态链接的 Agent 与 Controller（前端已嵌入），核对版本号与 tag 一致；
2. 创建 GitHub Release，上传二进制和 `sha256sums.txt`；
3. 推送镜像 `ghcr.io/forlwow/labherd-controller:v0.1.0`，正式版本同时更新 `latest`（带 `-` 的预发布版本如 `v0.2.0-rc.1` 不更新）。

在 Actions 页面确认 `release` 运行成功即可。

> **首次发版后**：ghcr 新建的镜像包默认是私有的。到 GitHub 个人主页 → Packages → `labherd-controller` → Package settings，把可见性改为 Public，否则服务器拉取镜像需要登录。

## 3. 部署 Controller

前提：机器上已安装 Docker（含 compose 插件）。

```bash
# 只需要 deploy/compose 目录，可以只拷这两个文件
mkdir -p ~/labherd && cd ~/labherd
cp <仓库>/deploy/compose/compose.yaml .
cp <仓库>/deploy/compose/.env.example .env
vim .env                 # 把 LABHERD_VERSION 改为要部署的版本
docker compose up -d
docker compose logs -f   # 看到 listening 日志即启动成功
```

浏览器打开 `http://<Controller IP>:8080`。

Compose 使用宿主机网络，以便 Controller 看到 Agent 的真实源 IP。如果机器上有防火墙，放行 9090 与 8080。

**拉不到镜像时**（校园网访问 ghcr 慢或中断）：在能访问的机器上中转。

```bash
docker pull ghcr.io/forlwow/labherd-controller:v0.1.0
docker save ghcr.io/forlwow/labherd-controller:v0.1.0 | gzip > controller.tar.gz
# 拷到 Controller 机器后
docker load < controller.tar.gz
docker compose up -d
```

## 4. 部署 Agent

在任意一台能 SSH 到所有 GPU 服务器的机器（如开发机）上运行 Ansible。发布包只在这台机器上下载一次，服务器本身不需要访问 GitHub。

前提：

- 控制机：`pipx install ansible-core`（或 `pip install --user ansible-core`）；
- GPU 服务器：可以 SSH 登录、有 sudo 权限、使用 systemd；不需要安装任何其他东西。

```bash
cd deploy/ansible
cp inventory.example.ini inventory.ini
vim inventory.ini        # 填写服务器 IP、SSH 账号和 Controller 地址
ansible gpu_nodes -m ping                                   # 先确认都能连上
ansible-playbook agent.yml -e labherd_version=v0.1.0 -K     # -K 询问 sudo 密码，免密 sudo 可省略
```

Playbook 做的事情：创建系统用户 `labherd`，安装 `/usr/local/bin/labherd-agent`，生成 `/etc/labherd/agent.env`，安装并启动 `labherd-agent.service`。重复运行是安全的，只有内容变化时才会重启 Agent。

### 变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `labherd_version` | 是* | 要部署的 Release 版本，如 `v0.1.0` |
| `labherd_controller_addr` | 是 | Controller 的 gRPC 地址，如 `10.0.0.10:9090` |
| `labherd_agent_args` | 否 | 其余命令行参数，如 `-advertise-ip=10.0.0.11 -log-level=debug`，可按主机覆盖 |
| `labherd_agent_binary` | 否 | 本地二进制的绝对路径；设置后不下载 Release，用于测试未发布的构建或控制机无法访问 GitHub 的情况 |
| `labherd_release_base` | 否 | 发布包下载地址前缀，默认 `https://github.com/forlwow/labherd/releases/download`；可改为内网镜像，目录结构需相同（`<前缀>/<版本>/<文件名>`） |

\* 设置了 `labherd_agent_binary` 时可以不填。

### 控制机访问 GitHub 慢或不通

下载超时 120 秒，最多尝试 3 次。仍然失败时，任选一种：

```bash
# 1. 走代理
https_proxy=http://<代理地址>:<端口> ansible-playbook agent.yml -e labherd_version=v0.1.0 -K

# 2. 在能访问 GitHub 的机器上下载并传到控制机，校验后用本地文件部署
gh release download v0.1.0 -R forlwow/labherd -p labherd-agent-linux-amd64 -p sha256sums.txt -D /tmp/labherd
scp /tmp/labherd/* <控制机>:~/labherd-release/
# 在控制机上
cd ~/labherd-release && sha256sum -c --ignore-missing sha256sums.txt
ansible-playbook agent.yml -e labherd_agent_binary=$HOME/labherd-release/labherd-agent-linux-amd64 -K
```

### 只部署部分服务器

```bash
ansible-playbook agent.yml -e labherd_version=v0.1.0 --limit 10.0.0.11   # 单台
ansible-playbook agent.yml -e labherd_version=v0.1.0 --check --diff      # 只预览改动
```

### 测试未发布的构建

```bash
CGO_ENABLED=0 make release
ansible-playbook agent.yml -e labherd_agent_binary=$PWD/../../bin/labherd-agent --limit 10.0.0.11
```

### 手动安装单台（无 Ansible）

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin labherd
sudo install -m 0755 labherd-agent-linux-amd64 /usr/local/bin/labherd-agent
sudo install -d /etc/labherd
sudo install -m 0644 deploy/systemd/agent.env.example /etc/labherd/agent.env
sudo vim /etc/labherd/agent.env       # 填写 LABHERD_CONTROLLER_ADDR
sudo install -m 0644 deploy/systemd/labherd-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now labherd-agent
```

## 5. 升级

1. 按第 2 节发布新版本。
2. Controller：修改 `.env` 中的 `LABHERD_VERSION`，执行 `docker compose pull && docker compose up -d`。
3. Agent：`ansible-playbook agent.yml -e labherd_version=<新版本>`。

Agent 与 Controller 的升级顺序无要求：Controller 重启期间 Agent 会不断重连，恢复后自动上线。

**回滚**：用旧版本号重复第 2、3 步。

## 6. 检查与日志

```bash
# Controller
docker compose ps
docker compose logs --tail 100 -f

# Agent（在 GPU 服务器上）
systemctl status labherd-agent
journalctl -u labherd-agent -f
/usr/local/bin/labherd-agent -version

# 批量查看所有 Agent 的状态与版本
ansible gpu_nodes -a "systemctl is-active labherd-agent"
ansible gpu_nodes -a "/usr/local/bin/labherd-agent -version"
```

## 7. 卸载 Agent

```bash
ansible gpu_nodes -b -m shell -a "systemctl disable --now labherd-agent; \
  rm -f /etc/systemd/system/labherd-agent.service /usr/local/bin/labherd-agent; \
  rm -rf /etc/labherd; systemctl daemon-reload; userdel labherd"
```
