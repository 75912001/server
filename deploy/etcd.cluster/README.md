# ETCD 集群部署

本文档记录 etcd 集群的从零创建、地址配置、跨 host 访问和验证步骤。

所有命令默认在当前 README 所在目录执行，也就是 `deploy/etcd.cluster`。如果你已经在这个目录，不需要再进入任何子目录：

```bash
pwd
```

部署形态：

- Docker Compose 项目：`etcd-cluster`
- 镜像：`quay.io/coreos/etcd:v3.6.6`
- 节点：`etcd1`、`etcd2`、`etcd3`
- 客户端端口：`2379`、`22379`、`32379`
- Peer 端口：`2380`、`22380`、`32380`
- 对外客户端地址：`192.168.71.123:2379`、`192.168.71.123:22379`、`192.168.71.123:32379`
- 数据目录：`data/`
- 自动压缩：`periodic`，保留最近 `1h` 历史版本
- Backend 配额：`8589934592` bytes
- 最大请求大小：`10485760` bytes

## 从零创建集群

停止并移除当前 etcd 容器和网络：

```bash
docker compose down
```

清理本地运行数据：

```bash
rm -rf data
```

启动 etcd 集群：

```bash
docker compose up -d
```

确认容器已启动：

```bash
docker ps --filter name=etcd
```

## 停止与重启

停止：

```bash
docker compose down
```

重启已有集群：

```bash
docker compose up -d
```

如果只是重启已有集群，不要清理 `data/`。

## 地址配置

本机访问可以使用：

```text
127.0.0.1:2379
127.0.0.1:22379
127.0.0.1:32379
```

跨 host 访问必须使用服务配置中的对外地址：

```text
192.168.71.123:2379
192.168.71.123:22379
192.168.71.123:32379
```

如果宿主机 IP 变化，需要同步修改：

- `docker-compose.yml` 中的 `--advertise-client-urls`
- `../cache/cache.1.yaml` 和 `../cache/cache.2.yaml` 中的 `etcd.endpoints`
- `../gateway/gateway.1.yaml` 和 `../gateway/gateway.2.yaml` 中的 `etcd.endpoints`
- `../online/online.1.yaml` 和 `../online/online.2.yaml` 中的 `etcd.endpoints`
- 其他业务服务配置中的 `etcd.endpoints`

## 服务配置

当前部署配置使用以下 etcd endpoints：

```yaml
etcd:
  endpoints:
    - 192.168.71.123:2379
    - 192.168.71.123:22379
    - 192.168.71.123:32379
```

跨 host 部署时，业务服务配置中的 `etcd.endpoints` 必须写 etcd 对外可达地址，不能写 `127.0.0.1`。

## 跨 host 访问检查

需要确认以下端口对客户端所在 host 放通：

- etcd 客户端端口：`2379`、`22379`、`32379`
- etcd peer 端口：`2380`、`22380`、`32380`

业务服务只需要访问客户端端口；peer 端口用于 etcd 节点间通信。当前 3 个节点运行在同一 Docker 网络内，但端口仍映射到宿主机，便于检查和后续扩展。

## 验证命令

查看 Compose 来源：

```bash
docker compose ls --format table
```

查看容器挂载：

```bash
docker inspect etcd1 --format "{{range .Mounts}}{{.Source}} -> {{.Destination}}{{println}}{{end}}"
```

查看集群健康：

```bash
docker exec etcd1 etcdctl --endpoints=http://etcd1:2379,http://etcd2:2379,http://etcd3:2379 endpoint health --write-out=table
```

查看节点状态：

```bash
docker exec etcd1 etcdctl --endpoints=http://etcd1:2379,http://etcd2:2379,http://etcd3:2379 endpoint status --write-out=table
```

查看业务服务注册数据：

```bash
docker exec etcd1 etcdctl --endpoints=http://etcd1:2379,http://etcd2:2379,http://etcd3:2379 get // --prefix
```

## 常见问题

### 启动时报容器名、网络名或端口冲突

说明已有 etcd 实例仍在运行，或端口仍被占用。先检查：

```bash
docker ps --filter name=etcd
docker compose ls
```

确认冲突实例已停止后，再在 `deploy/etcd.cluster` 目录执行启动命令。

### 服务发现异常

检查：

- etcd 容器是否运行
- `endpoint health` 是否全部为 `true`
- 业务服务配置中的 `etcd.endpoints` 是否为 `192.168.71.123:2379`、`192.168.71.123:22379`、`192.168.71.123:32379`
- 防火墙是否放通客户端端口
- gateway、online、cache 是否已重新注册到 `/project/server/` 前缀
