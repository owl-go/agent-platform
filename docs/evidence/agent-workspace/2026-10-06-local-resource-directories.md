# 本地资源目录分发：2026-10-06

将默认资源从后端嵌入 ZIP 迁移到顶层 `connectors/<source>/package/`、`skills/<name>/`、`experts/<name>/expert.json`。保留 20 个 Connector Package、9 个业务 Skill、8 个 Expert，三个创建 Skill 继续由既有 systemskills 分发。线上只读清点确认有效 Publication 和平台目录具有相同资源数量；没有导出 User Installation/Authorization 或凭据。

迁移时用真实 Connector Package Parser 和 Skill ZIP 规范化器逐项对照旧归档，37 个资源的内容、版本、稳定 Key 和专家绑定全部一致。随后移除旧后端 ZIP，保留规范化身份摘要 fixture，防止布局调整改变原版本。导入的第三方文档和资源保留字节、许可和换行；Git attributes 对这些包数据关闭文本改写和空白修整检查，目录 metadata 和新增实现仍按普通文本检查。

加载规则：根 `resources.json` 只记录 Catalog 版本，三个目录中的新子目录自动发现。连接器的 `package/` 将正式包与构建源码隔离；`scripts/connectors/` 不参与自动安装。先冻结、校验全部定义和包，再调用现有 Bootstrap Administrator 初始化事务。复用 PostgreSQL advisory lock、稳定 Key、source/version/checksum、内容寻址 Object Key 和既有初始化账本；没有新增或修改 Migration。

服务 Dockerfile 校验目录后将它们复制到 `/app/resources`，API/Worker 通过 `AGENT_WORKSPACE_RESOURCE_ROOT` 自动加载。`make test` / `make build` 包含 `make resources-check`；部署范围检测包含目录变化，因此新增目录会触发服务镜像更新。目录删除不删除现有数据库资源。已有管理员自建资源、旧脚本发布记录、disabled 状态、私人版本冲突和历史快照保留。

已执行：

- 迁移前旧 ZIP × 新目录等价测试：通过，20 Connector、9 Skill、8 Expert 全部比较；执行两次后才移除归档。
- `go -C backend test -count=1 ./internal/defaultresources ./internal/data/workspace/gormrepo`：通过。目录单测覆盖独立发现、脚本副本隔离、冻结字节、未知/保留 Key、重复 Key/名称、版本、JSON 未知字段、完整 Expert、未知 Skill 绑定、source 别名、双模式包、metadata/package 根和资源符号链接、取消及显式根路径错误。
- 一次性 `postgres:17-alpine` + tmpfs，设置测试进程 `WORKSPACE_TEST_POSTGRES_DSN` 后运行 `go -C backend test -count=1 ./internal/data/workspace/gormrepo -run '^(TestDefaultResource|TestDefaultCatalog|TestDefaultConnector|TestDefaultSkill|TestLocalResourceDirectory)' -v`：8 项通过，79.172s。覆盖新安装全目录、并发启动、重复/升级、无授权或伪造 Conformance、私有资源保护、同名保留、对象写失败回滚，以及在不修改中心清单的情况下新增 Skill/Expert 并解析稳定 Key 绑定。旧脚本 Publication 的 Revision、disabled 状态与 Version 全部保留，Connector Revision/Publication 数量各为 1。测试数据库和容器已清理。
- `make deploy-test`：23 个部署测试、13 个安装测试通过，Shell 语法通过；新增/删除目录的发布范围检测覆盖三种资源。
- `make test`、`make build`：通过，自动目录校验报告 20 Connector、9 Skill、8 Expert +3 内置 Skill。全量 Go 门禁未配置数据库 DSN，其中 Skip 不作为数据库集成通过；上面的隔离数据库测试是真实执行结果。
- `git diff --check`、暂存后的 `git diff --cached --check` 和 `cmp -s AGENTS.md CLAUDE.md`：通过。没有 Web 实现改动。

目录初始化安装的是平台定义。新环境 CLI 保持 disabled，仍需 exact bundle SHA-256 × 当前 Runtime RepoDigest 的真实 Conformance 才能开放；MCP 使用者仍需各自配置或授权。本变更不新增供应商业务调用、完整 Runtime Production Conformance 或全新 Ubuntu 安装验收证据。
