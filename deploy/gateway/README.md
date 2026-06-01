# Gateway Container

Run commands from this README directory; the first command enters the project root:
The image uses `Asia/Shanghai` as its timezone so file logs match the host's local time.

```bash
cd ../..
```

# 删除日志
```bash
rm -rf deploy/gateway/log/*
```

## Build Image
```bash
docker build -f deploy/gateway/Dockerfile -t server.gateway:dev .
docker images | grep server.gateway
```

## Remove Image
```bash
docker rmi server.gateway:dev
```

# gateway.1

## Run Container
```bash
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.1 \
  -p 10101:10101 \
  -p 20101:20101 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.1.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  server.gateway:dev
```

## Stop Container
```bash
docker stop server.gateway.1
```

## Start Container
```bash
docker start server.gateway.1
```

## Remove Container
```bash
docker rm server.gateway.1
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20101 list gateway.GatewayService
grpcurl -plaintext 192.168.71.123:20101 describe gateway.GatewayService
```

# gateway.2

## Run Container
```bash
mkdir -p deploy/gateway/log
PROJECT_ROOT="$(pwd -W)"

MSYS_NO_PATHCONV=1 docker run -d --name server.gateway.2 \
  -p 10102:10102 \
  -p 20102:20102 \
  -v "$PROJECT_ROOT/deploy/gateway/gateway.2.yaml:/app/config/gateway.yaml" \
  -v "$PROJECT_ROOT/deploy/gateway/log:/app/log" \
  server.gateway:dev
```

## Stop Container
```bash
docker stop server.gateway.2
```

## Start Container
```bash
docker start server.gateway.2
```

## Remove Container
```bash
docker rm server.gateway.2
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20102 list gateway.GatewayService
grpcurl -plaintext 192.168.71.123:20102 describe gateway.GatewayService
```
