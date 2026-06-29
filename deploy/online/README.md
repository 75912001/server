# Online Container

本文件中的命令可在仓库任意子目录执行，第一行会自动进入 Git 仓库根目录。

镜像内时区为 `Asia/Shanghai`，容器日志时间与宿主机本地时间保持一致。

online 启动时会先加载共享游戏配置. 镜像构建会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`. server 只校验服务端消费字段和跨表引用; 角色展示字段, 客户端 PNG, `.tpsheet` 和 frame 资源仍由 sa.desktop 校验.

## 准备目录

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/online/log
PROJECT_ROOT="$(pwd -W)"
```

## 清理日志

```bash
cd "$(git rev-parse --show-toplevel)"
rm -rf deploy/online/log/*
```

# 方式一：本地手动构建镜像

本方式使用本机源码构建镜像：

```text
server.online:dev
```

## 构建本地镜像

```bash
cd "$(git rev-parse --show-toplevel)"
docker build -f deploy/online/Dockerfile -t server.online:dev .
docker images | grep server.online
```

## 启动 online.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/online/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.online.1 \
  -p 20201:20201 \
  -v "$PROJECT_ROOT/deploy/online/online.1.yaml:/app/config/online.yaml" \
  -v "$PROJECT_ROOT/deploy/online/log:/app/log" \
  server.online:dev
```

`/app/config` 中的共享游戏配置来自镜像内置文件. 只挂载 `online.yaml` 和日志目录即可.

## 启动 online.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/online/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.online.2 \
  -p 20202:20202 \
  -v "$PROJECT_ROOT/deploy/online/online.2.yaml:/app/config/online.yaml" \
  -v "$PROJECT_ROOT/deploy/online/log:/app/log" \
  server.online:dev
```

`/app/config` 中的共享游戏配置来自镜像内置文件. 只挂载 `online.yaml` 和日志目录即可.

# 方式二：从 GHCR 获取镜像

本方式使用 GitHub Actions 已生成并推送的镜像：

```text
ghcr.io/75912001/server/online:main
```

## 拉取镜像

```bash
docker pull ghcr.io/75912001/server/online:main
```

## 启动 online.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/online/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.online.1 \
  -p 20201:20201 \
  -v "$PROJECT_ROOT/deploy/online/online.1.yaml:/app/config/online.yaml" \
  -v "$PROJECT_ROOT/deploy/online/log:/app/log" \
  ghcr.io/75912001/server/online:main
```

## 启动 online.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/online/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.online.2 \
  -p 20202:20202 \
  -v "$PROJECT_ROOT/deploy/online/online.2.yaml:/app/config/online.yaml" \
  -v "$PROJECT_ROOT/deploy/online/log:/app/log" \
  ghcr.io/75912001/server/online:main
```

# 容器管理

## 停止 online.1

```bash
docker stop server.online.1
```

## 停止 online.2

```bash
docker stop server.online.2
```

## 启动已停止的 online.1

```bash
docker start server.online.1
```

## 启动已停止的 online.2

```bash
docker start server.online.2
```

## 删除 online.1

```bash
docker rm server.online.1
```

## 删除 online.2

```bash
docker rm server.online.2
```

## 删除本地镜像

```bash
docker rmi server.online:dev
```

# 验证

## 查看容器

```bash
docker ps --filter name=server.online
```

## 查看日志

```bash
docker logs server.online.1
docker logs server.online.2
```

如果日志出现 `load game config failed`, 先检查镜像是否按当前仓库构建, 以及 `deploy/online/*.yaml` 中的 `custom.gameConfigDir` 是否指向 `/app/config`。

## 查看 online.1 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20201 list online.OnlineService
grpcurl -plaintext 192.168.71.123:20201 describe online.OnlineService
```

## 查看 online.2 gRPC 服务

```bash
grpcurl -plaintext 192.168.71.123:20202 list online.OnlineService
grpcurl -plaintext 192.168.71.123:20202 describe online.OnlineService
```
