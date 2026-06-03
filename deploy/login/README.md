# Login Container

本文件中的命令可在仓库任意子目录执行，第一行会自动进入 Git 仓库根目录。
镜像内时区为 `Asia/Shanghai`，容器日志时间与宿主机本地时间保持一致。

## 准备目录

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/login/log
PROJECT_ROOT="$(pwd -W)"
```

## 清理日志

```bash
cd "$(git rev-parse --show-toplevel)"
rm -rf deploy/login/log/*
```

# 方式一：本地手动构建镜像

本方式使用本机源码构建镜像：

```text
server.login:dev
```

## 构建本地镜像

```bash
cd "$(git rev-parse --show-toplevel)"
docker build -f deploy/login/Dockerfile -t server.login:dev .
docker images | grep server.login
```

## 删除本地镜像

```bash
docker rmi server.login:dev
```

## 启动 login.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/login/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.login.1 \
  -p 30401:30401 \
  -v "$PROJECT_ROOT/deploy/login/login.1.yaml:/app/config/login.yaml" \
  -v "$PROJECT_ROOT/deploy/login/log:/app/log" \
  server.login:dev
```

## 启动 login.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/login/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.login.2 \
  -p 30402:30402 \
  -v "$PROJECT_ROOT/deploy/login/login.2.yaml:/app/config/login.yaml" \
  -v "$PROJECT_ROOT/deploy/login/log:/app/log" \
  server.login:dev
```

# 方式二：从 GHCR 获取镜像

本方式使用 GitHub Actions 已生成并推送的镜像：

```text
ghcr.io/75912001/server/login:main
```

## 拉取镜像

```bash
docker pull ghcr.io/75912001/server/login:main
```

## 启动 login.1

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/login/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.login.1 \
  -p 30401:30401 \
  -v "$PROJECT_ROOT/deploy/login/login.1.yaml:/app/config/login.yaml" \
  -v "$PROJECT_ROOT/deploy/login/log:/app/log" \
  ghcr.io/75912001/server/login:main
```

## 启动 login.2

```bash
cd "$(git rev-parse --show-toplevel)"
mkdir -p deploy/login/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.login.2 \
  -p 30402:30402 \
  -v "$PROJECT_ROOT/deploy/login/login.2.yaml:/app/config/login.yaml" \
  -v "$PROJECT_ROOT/deploy/login/log:/app/log" \
  ghcr.io/75912001/server/login:main
```

# 容器管理

## 停止 login.1

```bash
docker stop server.login.1
```

## 停止 login.2

```bash
docker stop server.login.2
```

## 启动已停止的 login.1

```bash
docker start server.login.1
```

## 启动已停止的 login.2

```bash
docker start server.login.2
```

## 删除 login.1

```bash
docker rm server.login.1
```

## 删除 login.2

```bash
docker rm server.login.2
```

# 验证

## 查看容器

```bash
docker ps --filter name=server.login
```

## 查看日志

```bash
docker logs server.login.1
docker logs server.login.2
```

## 请求 login.1 token

```bash
curl -X POST http://192.168.71.123:30401/api/login/token \
  -H 'Content-Type: application/json' \
  -d '{"account":"test-account","token":"test-token"}'
```

## 请求 login.1 session

```bash
curl -X POST http://192.168.71.123:30401/api/login/session \
  -H 'Content-Type: application/json' \
  -d '{"account":"test-account","token":"test-token"}'
```

## 请求 login.2 token

```bash
curl -X POST http://192.168.71.123:30402/api/login/token \
  -H 'Content-Type: application/json' \
  -d '{"account":"test-account","token":"test-token"}'
```

## 请求 login.2 session

```bash
curl -X POST http://192.168.71.123:30402/api/login/session \
  -H 'Content-Type: application/json' \
  -d '{"account":"test-account","token":"test-token"}'
```
