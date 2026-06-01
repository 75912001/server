# Cache Container

Run commands from this README directory; the first command enters the project root:
The image uses `Asia/Shanghai` as its timezone so file logs match the host's local time.

```bash
cd "$(git rev-parse --show-toplevel)"
```

# 删除日志
```bash
cd "$(git rev-parse --show-toplevel)"
rm -rf deploy/cache/log/*
```

## Build Image
```bash
cd "$(git rev-parse --show-toplevel)"
docker build -f deploy/cache/Dockerfile -t server.cache:dev .
docker images | grep server.cache
```

## Remove Image
```bash
docker rmi server.cache:dev
```

# cache.1

## Run Container
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

## Stop Container
```bash
docker stop server.cache.1
```

## Start Container
```bash
docker start server.cache.1
```

## Remove Container
```bash
docker rm server.cache.1
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20301 list cache.CacheService
grpcurl -plaintext 192.168.71.123:20301 describe cache.CacheService
```

# cache.2

## Run Container
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

## Stop Container
```bash
docker stop server.cache.2
```

## Start Container
```bash
docker start server.cache.2
```

## Remove Container
```bash
docker rm server.cache.2
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20302 list cache.CacheService
grpcurl -plaintext 192.168.71.123:20302 describe cache.CacheService
```
