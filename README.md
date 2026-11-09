# labherd

LabHerd：实验室服务器资源监控与调度系统。

## 功能

## 架构

## 快速开始

本地试运行：

```bash
make release                      # 构建带前端页面的程序
./bin/labherd-controller          # 网页：http://localhost:8080
./bin/labherd-agent               # 默认连接 localhost:9090
```

部署到实验室服务器（Controller 用 Docker Compose，Agent 用 Ansible 批量安装）见 [部署手册](docs/operations/deploy.md)。

## 文档

见 [docs/](docs/README.md)。
