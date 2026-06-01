# Cache Container

本文件中的命令可在仓库任意子目录执行，第一行会自动进入 Git 仓库根目录。

镜像内时区为 `Asia/Shanghai`，容器日志时间与宿主机本地时间保持一致。

## 准备目录

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/cache/log
PROJECT_ROOT="$(pwd -W)"
```

## 清理日志

```bash
cd "$(git rev-parse --show-toplevel)"
rm -rf deploy/cache/log/*
```

# 方式一：本地手动构建镜像

本方式使用本机源码构建镜像：

```text
server.cache:dev
```

## 构建本地镜像

```bash
cd "$(git rev-parse --show-toplevel)"
docker build -f deploy/cache/Dockerfile -t server.cache:dev .
docker images | grep server.cache
```

## 删除本地镜像

```bash
docker rmi server.cache:dev
```

## 启动 cache.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/cache/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.1 \
  -p 20301:20301 \
  -v "$PROJECT_ROOT/deploy/cache/cache.1.yaml:/app/config/cache.yaml" \
  -v "$PROJECT_ROOT/deploy/cache/log:/app/log" \
  server.cache:dev
```

## 启动 cache.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/cache/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.2 \
  -p 20302:20302 \
  -v "$PROJECT_ROOT/deploy/cache/cache.2.yaml:/app/config/cache.yaml" \
  -v "$PROJECT_ROOT/deploy/cache/log:/app/log" \
  server.cache:dev
```

# 方式二：从 GHCR 获取镜像

本方式使用 GitHub Actions 已生成并推送的镜像：

```text
ghcr.io/75912001/server/cache:main
```

## 拉取镜像

```bash
docker pull ghcr.io/75912001/server/cache:main
```

## 启动 cache.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/cache/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.1 \
  -p 20301:20301 \
  -v "$PROJECT_ROOT/deploy/cache/cache.1.yaml:/app/config/cache.yaml" \
  -v "$PROJECT_ROOT/deploy/cache/log:/app/log" \
  ghcr.io/75912001/server/cache:main
```

## 启动 cache.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/cache/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.2 \
  -p 20302:20302 \
  -v "$PROJECT_ROOT/deploy/cache/cache.2.yaml:/app/config/cache.yaml" \
  -v "$PROJECT_ROOT/deploy/cache/log:/app/log" \
  ghcr.io/75912001/server/cache:main
```

# 容器管理

## 停止 cache.1

```bash
docker stop server.cache.1
```

## 停止 cache.2

```bash
docker stop server.cache.2
```

## 启动已停止的 cache.1

```bash
docker start server.cache.1
```

## 启动已停止的 cache.2

```bash
docker start server.cache.2
```

## 删除 cache.1

```bash
docker rm server.cache.1
```

## 删除 cache.2

```bash
docker rm server.cache.2
```

# 验证

## 查看容器

```bash
docker ps --filter name=server.cache
```

## 查看日志

```bash
docker logs server.cache.1
docker logs server.cache.2
```

## 查看 cache.1 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20301 list cache.CacheService
grpcurl -plaintext 192.168.71.123:20301 describe cache.CacheService
```

## 查看 cache.2 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20302 list cache.CacheService
grpcurl -plaintext 192.168.71.123:20302 describe cache.CacheService
```
