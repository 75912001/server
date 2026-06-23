# server 项目规则

本项目继承用户级 `AGENTS.md`, 下列规则仅描述 `server` 项目特有约束.

# 文档同步约定

- 在全局文档同步规则基础上, 本项目所有代码修改都必须同步更新对应服务目录下的 `README.md` 设计文档, 保证设计文档与当前代码实现一致.
- 服务目录下的 `README.md` 至少应覆盖服务设计、架构、核心能力、数据流、接口、注意事项和待改进项; 涉及行为、依赖、配置、部署或接口变化时, 需要同步补充或修正对应章节.
- 任何涉及 `login`、`gateway`、`online`、`cache` 行为、接口、配置、部署或测试流程的改动, 都必须检查对应目录下的 `README.md`、`TEST.md`、`deploy/*/README.md`、`bin/*.yaml.template` 和相关 `deploy/*.yaml`, 并同步更新过期内容.

# 代码风格

- 生成或修改业务代码时, 优先检查并使用 `github.com/75912001/xlib` 中已有模块, 避免重复实现通用能力.
- 常见优先复用模块包括但不限于 `xlib/map`、`xlib/timer`、`xlib/actor`、`xlib/control`、`xlib/etcd`、`xlib/grpc`、`xlib/log`、`xlib/runtime`.
- 仅当 `xlib` 没有合适能力、标准库实现更直接, 或当前场景需要特殊处理时, 才新增本地实现.
- 新增本地通用实现时, 需要简要说明未使用 `xlib` 的原因.

# 部署约定

- 部署约定基于项目内 `deploy/` 目录.
