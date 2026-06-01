# Redis 集群部署

本文档记录 Redis Cluster 的从零创建、地址配置、跨 host 访问和验证步骤。

所有命令默认在当前 README 所在目录执行，也就是 `deploy/redis.cluster`。如果你已经在这个目录，不需要再进入任何子目录：

```bash
pwd
```

部署形态：

- Docker Compose 项目：`redis-cluster`
- 容器名：`redis-cluster`
- 镜像：`redis:7`
- Redis 密码：`111111`
- Redis 客户端端口：`7000-7005`
- Redis Cluster 总线端口：`17000-17005`
- 节点对外宣布地址：`192.168.71.123:7000-7005`
- 拓扑：3 主 3 从
- 集群健康目标：`cluster_state:ok`，`cluster_slots_ok:16384`，`cluster_known_nodes:6`

注意：不要同时启动两套 Redis Cluster。相同容器名和端口会发生冲突。

## 从零创建集群

停止并移除当前 Redis 容器和网络：

```bash
docker compose down
```

清理 6 个节点的本地运行数据：

```bash
rm -rf 7000/data 7001/data 7002/data 7003/data 7004/data 7005/data
```

启动 Redis 容器：

```bash
docker compose up -d
```

确认容器已启动：

```bash
docker ps --filter name=redis-cluster
```

创建 3 主 3 从集群：

```bash
docker exec redis-cluster redis-cli -a 111111 --no-auth-warning --cluster create 127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 127.0.0.1:7003 127.0.0.1:7004 127.0.0.1:7005 --cluster-replicas 1 --cluster-yes
```

创建完成后验证集群状态：

```bash
docker exec redis-cluster redis-cli -a 111111 --no-auth-warning -p 7000 cluster info
```

预期至少包含：

```text
cluster_state:ok
cluster_slots_assigned:16384
cluster_slots_ok:16384
cluster_known_nodes:6
```

查看节点拓扑：

```bash
docker exec redis-cluster redis-cli -a 111111 --no-auth-warning -p 7000 cluster nodes
```

写入和读取测试：

```bash
docker exec redis-cluster redis-cli -a 111111 --no-auth-warning -p 7000 -c set test-key hello
docker exec redis-cluster redis-cli -a 111111 --no-auth-warning -p 7001 -c get test-key
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

如果只是重启已有集群，不要清理 `7000-7005/data`，也不要重复执行 `--cluster create`。

## 地址配置

本机访问可以使用：

```text
127.0.0.1:7000
127.0.0.1:7001
127.0.0.1:7002
127.0.0.1:7003
127.0.0.1:7004
127.0.0.1:7005
```

跨 host 访问必须使用 Redis 节点对外可达的地址。当前配置使用：

```text
192.168.71.123:7000
192.168.71.123:7001
192.168.71.123:7002
192.168.71.123:7003
192.168.71.123:7004
192.168.71.123:7005
```

Redis Cluster 客户端会根据节点返回的 cluster slots / moved 信息访问其他节点。因此跨 host 场景下，`redis.conf` 中的 `cluster-announce-ip` 必须是客户端可访问的宿主机 IP，不能使用 `127.0.0.1`。

如宿主机 IP 变化，需要同步修改：

- `deploy/redis.cluster/7000-7005/redis.conf` 中的 `cluster-announce-ip`
- `deploy/cache/cache.1.yaml` 和 `deploy/cache/cache.2.yaml` 中的 `redis.addrs`

## Cache 服务配置

当前 cache 部署配置已指向 Redis 对外地址：

```yaml
redis:
  - name: cache
    addrs:
      - 192.168.71.123:7000
      - 192.168.71.123:7001
      - 192.168.71.123:7002
      - 192.168.71.123:7003
      - 192.168.71.123:7004
      - 192.168.71.123:7005
    password: "111111"
```

跨 host 部署时，cache 配置中的 `redis.addrs` 必须写 Redis 节点对外可达地址，不能写 `127.0.0.1`。

## 跨 host 访问检查

需要确认以下端口对客户端所在 host 放通：

- Redis 客户端端口：`7000-7005`
- Redis Cluster 总线端口：`17000-17005`

如果只放通 `7000-7005`，部分集群通信或故障转移相关行为可能异常。建议防火墙、安全组和 Docker 端口映射都同时覆盖这两组端口。

## 常见问题

### 启动时报容器名或端口冲突

说明当前已有 Redis Cluster 容器或端口占用。先检查：

```bash
docker ps --filter name=redis-cluster
docker compose ls
```

确认冲突实例已停止后，再在 `deploy/redis.cluster` 目录执行启动命令。

### 外部机器连接后出现 MOVED 到不可达地址

检查 `cluster nodes` 输出中的地址。如果返回 `127.0.0.1` 或容器内 IP，外部机器会无法跟随 Redis Cluster 重定向。需要把 6 个节点配置中的 `cluster-announce-ip` 改为宿主机对外可达 IP，然后重建或修正集群节点配置。

### cache 服务启动时 Redis Ping 失败

检查：

- Redis 容器是否运行
- `cluster_state` 是否为 `ok`
- cache 配置中的 `redis.addrs` 是否为 Redis 对外可达地址
- Redis 密码是否为 `111111`
- 防火墙是否放通 `7000-7005` 和 `17000-17005`
