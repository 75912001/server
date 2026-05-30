# Online Container

Run commands from the project root:
The image uses `Asia/Shanghai` as its timezone so file logs match the host's local time.

```bash
cd /d/src/github.com/server
```

## Build Image
```bash
docker build -f deploy/online/Dockerfile -t server.online:dev .
docker images | grep server.online
```

## Remove Image
```bash
docker rmi server.online:dev
```

# online.1

## Run Container
```bash
mkdir -p /d/src/github.com/server/deploy/online/log

MSYS_NO_PATHCONV=1 docker run -d --name server.online.1 \
  -p 20201:20201 \
  -v "D:/src/github.com/server/deploy/online/online.1.yaml:/app/config/online.yaml" \
  -v "D:/src/github.com/server/deploy/online/log:/app/log" \
  server.online:dev
```

## Stop Container
```bash
docker stop server.online.1
```

## Start Container
```bash
docker start server.online.1
```

## Remove Container
```bash
docker rm server.online.1
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20201 list online.OnlineService
grpcurl -plaintext 192.168.71.123:20201 describe online.OnlineService
```

# online.2

## Run Container
```bash
mkdir -p /d/src/github.com/server/deploy/online/log

MSYS_NO_PATHCONV=1 docker run -d --name server.online.2 \
  -p 20202:20202 \
  -v "D:/src/github.com/server/deploy/online/online.2.yaml:/app/config/online.yaml" \
  -v "D:/src/github.com/server/deploy/online/log:/app/log" \
  server.online:dev
```

## Stop Container
```bash
docker stop server.online.2
```

## Start Container
```bash
docker start server.online.2
```

## Remove Container
```bash
docker rm server.online.2
```

## Inspect gRPC Service
```bash
grpcurl -plaintext 192.168.71.123:20202 list online.OnlineService
grpcurl -plaintext 192.168.71.123:20202 describe online.OnlineService
```
