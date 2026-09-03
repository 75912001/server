# Online Container

本文件中的命令可在仓库任意子目录执行，第一行会自动进入 Git 仓库根目录。

镜像内时区为 `Asia/Shanghai`，容器日志时间与宿主机本地时间保持一致。

online 启动时会先加载共享游戏配置. 镜像构建会把仓库 `config/` 整体复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`. `scene/*.yaml` 根节点不含格式版本字段; 单人或组队 PVE 自动遇敌从 `scene/*.yaml -> enemy.group.yaml` 读取敌人条目, 分别引用 `pet.yaml` 和 `ai.yaml`, 每张地图直接读取平铺的 `encounter.enabled` 和 `encounter.enemyGroups`, 再按权重选择敌人组. 敌人组直接引用宠物模板, 数量和等级来自敌人组, 属性来自宠物模板, 战斗技能、权重和目标策略由每个敌人的 `battleAI` 引用提供. `pet.yaml` 不再包含 AI 引用; 旧 `enemies[].skill` 和 AI 分离权重字段会导致启动失败, 发布时须同时更新代码和整套配置. server 只校验服务端消费字段和跨表引用; 角色展示字段、宠物展示字段、客户端 PNG、`.tpsheet` 和 frame 资源仍由 sa.desktop 校验.

任务系统还要求镜像内包含`task.yaml`和`reward.yaml`. 两表与道具、敌群引用在启动时校验, 缺失或错误时直接启动失败. Godot任务编辑器只修改仓库源文件, 不更新已运行容器; 修改运行任务后需要按原流程重新构建镜像并重启实例. 配置目录仍由现有`custom.gameConfigDir`指定, 不增加部署参数.

## gRPC 消息大小

`online.1.yaml` 和 `online.2.yaml` 将 `grpc.maxReceiveMessageBytes`、`grpc.maxSendMessageBytes` 都设为 `67108864`. Online gRPC 服务端和生成客户端的单条消息收发上限均为 64MiB.

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
