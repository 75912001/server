# Gateway Container

本文件中的命令可在仓库任意子目录执行，第一行会自动进入 Git 仓库根目录。

镜像内时区为 `Asia/Shanghai`，容器日志时间与宿主机本地时间保持一致。

## 准备目录

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"
```

## 清理日志

```bash
cd "$(git rev-parse --show-toplevel)"
rm -rf deploy/gateway/log/*
```

# 方式一：本地手动构建镜像

本方式使用本机源码构建镜像：

```text
server.gateway:dev
```

## 构建本地镜像

```bash
cd "$(git rev-parse --show-toplevel)"
docker build -f deploy/gateway/Dockerfile -t server.gateway:dev .
docker images | grep server.gateway
```

## 删除本地镜像

```bash
docker rmi server.gateway:dev
```

## 启动 gateway.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.1 \
  -p 10101:10101 \
  -p 20101:20101 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.1.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  server.gateway:dev
```

## 启动 gateway.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.2 \
  -p 10102:10102 \
  -p 20102:20102 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.2.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  server.gateway:dev
```

# 方式二：从 GHCR 获取镜像

本方式使用 GitHub Actions 已生成并推送的镜像：

```text
ghcr.io/75912001/server/gateway:main
```

## 拉取镜像

```bash
docker pull ghcr.io/75912001/server/gateway:main
```

## 启动 gateway.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.1 \
  -p 10101:10101 \
  -p 20101:20101 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.1.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  ghcr.io/75912001/server/gateway:main
```

## 启动 gateway.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.2 \
  -p 10102:10102 \
  -p 20102:20102 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.2.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  ghcr.io/75912001/server/gateway:main
```

# 容器管理

## 停止 gateway.1

```bash
docker stop server.gateway.1
```

## 停止 gateway.2

```bash
docker stop server.gateway.2
```

## 启动已停止的 gateway.1

```bash
docker start server.gateway.1
```

## 启动已停止的 gateway.2

```bash
docker start server.gateway.2
```

## 删除 gateway.1

```bash
docker rm server.gateway.1
```

## 删除 gateway.2

```bash
docker rm server.gateway.2
```

# 验证

## 查看容器

```bash
docker ps --filter name=server.gateway
```

## 查看日志

```bash
docker logs server.gateway.1
docker logs server.gateway.2
```

## 查看 gateway.1 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20101 list gateway.GatewayService
grpcurl -plaintext 192.168.71.123:20101 describe gateway.GatewayService
```

## 查看 gateway.2 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20102 list gateway.GatewayService
grpcurl -plaintext 192.168.71.123:20102 describe gateway.GatewayService
```
