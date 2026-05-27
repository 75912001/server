# Cache Container

Run commands from the project root:
The image uses `Asia/Shanghai` as its timezone so file logs match the host's local time.

```bash
cd /d/src/github.com/server
```

## Build Image
```bash
docker build -f deploy/cache/Dockerfile -t server.cache:dev .
docker images | grep server.cache
```

# cache.1

## Run Container
```bash
mkdir -p /d/src/github.com/server/deploy/cache/log

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.1 \
  -p 20301:20301 \
  -v "D:/src/github.com/server/deploy/cache/cache.1.yaml:/app/config/cache.yaml" \
  -v "D:/src/github.com/server/deploy/cache/log:/app/log" \
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
mkdir -p /d/src/github.com/server/deploy/cache/log

MSYS_NO_PATHCONV=1 docker run -d --name server.cache.2 \
  -p 20302:20302 \
  -v "D:/src/github.com/server/deploy/cache/cache.2.yaml:/app/config/cache.yaml" \
  -v "D:/src/github.com/server/deploy/cache/log:/app/log" \
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