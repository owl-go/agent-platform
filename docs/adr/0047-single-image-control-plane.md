---
status: superseded by ADR-0048
---

# 单镜像整合部署

> 本决策中控制服务与 Runtime 共用镜像的部分已由 [ADR-0048](0048-separate-control-and-runtime-images.md) 取代；以下保留原始决策记录。

2026-10-18 用户要求前后端及基础服务合并为一个镜像，并在首次启动创建 `admin / admin123`，发布 ARM64/AMD64。新增可选整合部署，在一个受信任控制容器内按依赖顺序运行 Web 反向代理、API、Worker、Keycloak、PostgreSQL 和 MinIO；任一关键进程退出则终止整套进程，由 Docker 重启。一个 `/data` 卷保存数据库、对象、Workspace 与随机系统密钥；已有账号密码绝不被重启覆盖。弱默认密码只用于环回地址的首次本地安装，公开 HTTPS 部署必须明确覆盖初始密码。

任务不在控制进程中直接执行：继续用同一镜像的非 root、只读、受限隔离容器；只把本次任务目录交给 Runtime，不挂载 Docker socket 或整卷。控制 Worker 通过受限 Docker CLI 路径映射把 `/data` 转成宿主真实卷路径，映射拒绝路径穿越与未明确挂载的来源；每个卷的稳定 Namespace 隔离 Warm Container。默认整合入口显式采用 ADR-0046 的本地模式，不得把它宣传为生产隔离验收；显式生产模式仍要求 runsc、精确 Registry RepoDigest 和既有宿主 Egress 策略，缺一即拒绝启动。原生产分离部署保留。

浏览器的公开 OIDC 配置在启动时生成，允许一个前端构建复用于不同域名与端口。API 可显式把此 Issuer 的 discovery/JWKS 运输改到同容器 loopback，不改变公开 Issuer、Audience、签名、有效期或重定向校验；其他目标地址不被改写。该方案以共享升级、故障和备份边界及较大镜像，换取单容器部署便利；不引入新的领域实体，也不把部署状态放进 CONTEXT.md。
