# 服务拓扑

本文记录本项目当前开发环境的服务关系。具体部署、地址、网络访问规则和验证命令以 `deploy/` 下各组件文档为准。

## 组件

```text
client.simulator
  -> gateway
      -> online
          -> cache
              -> Redis Cluster

gateway / online / cache
  -> etcd Cluster
```

职责：

- `gateway`：客户端接入、连接管理、用户请求转发。
- `online`：在线状态、用户登录流程、跨 gateway/online 协调。
- `cache`：用户记录、用户 session、token 等缓存数据访问。
- `Redis Cluster`：cache 后端数据存储。
- `etcd Cluster`：服务注册发现。
- `client.simulator`：本地客户端模拟器。

## 配置入口

服务注册发现、对外注册地址、Docker 网络访问规则、启动命令和验证命令分别查看对应组件文档：

- cache：`deploy/cache/README.md`
- Redis Cluster：`deploy/redis.cluster/README.md`
- etcd Cluster：`deploy/etcd.cluster/README.md`

`docs/topology.md` 只作为全局拓扑索引，不维护单组件的部署细节。
