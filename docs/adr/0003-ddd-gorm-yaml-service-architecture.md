---
status: superseded by ADR-0004
---

# 服务端采用 DDD 分层、GORM 与 YAML 配置

服务端按限界上下文组织业务能力，并在每个上下文内区分 Domain、Application 和 Infrastructure。领域层拥有聚合、不变量、状态转换和 Repository 端口；应用层编排用例；基础设施层通过 GORM 实现 PostgreSQL Repository。服务进程从经过严格校验的 YAML 文件读取配置，部署环境变量只用于替换 YAML 中的 Secret 占位符。

这个结构让 Run、Attempt、Run Lease 和 Run Event 的业务规则不再散落在 SQL 与进程入口中，也避免 HTTP、Worker 和 PostgreSQL 成为领域模型的依赖。代价是为简单 CRUD 增加显式映射，并且需要在 Repository 中谨慎维护聚合事务边界。

## 影响

- 新业务代码先确定所属限界上下文，再依赖该上下文的 Domain 与 Application 接口。
- Domain 不依赖 GORM、HTTP、配置文件或外部 SDK；GORM Model 不能作为 API DTO 或领域实体复用。
- 跨聚合写入由应用服务编排；一个聚合的状态变更、Attempt、Run Lease 和 Run Event 必须在同一数据库事务中提交。
- PostgreSQL 特有的 `FOR UPDATE SKIP LOCKED` 等并发语义允许封装在 GORM Repository 内，但不能泄漏到领域层。
- YAML 是 API 与 Worker 的权威配置格式；未知字段、缺失必填项和非法时长必须导致启动失败。
