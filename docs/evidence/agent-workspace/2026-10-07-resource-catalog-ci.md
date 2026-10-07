# 资源目录合并检查：2026-10-07

[PR #69](https://github.com/owl-go/agent-platform/pull/69) 首次 CI 的后端常规测试成功，race 阶段因 GORM Repository 包超过默认 10 分钟而失败。运行记录为 `37555215940`；超时发生于 `TestDefaultCatalogFreshInstallDoesNotAuthorizeOrVerifyCLI`，堆栈位于目录组包的 ZIP 压缩，没有报告数据竞争。公共事务夹具原先每次加载完整资源目录后只保留 3 个定义，全量安装用例还为数据库准备、计数和初始化重复加载完整目录。

修复仅修改测试夹具：事务测试从仓库复制旅行 Skill、旅行 Expert 和 ai-hive MCP 包，通过真实目录加载器校验；数据库夹具单独建立，不加载资源。全量用例仍加载并安装所有 20 个 Connector、9+3 个 Skill 和 8 个 Expert，复用已校验 Catalog，继续断言没有用户安装/授权、伪造 Conformance 或可用的未验证 CLI。公开入口的目录发现、新增资源、重复启动、稳定绑定和旧脚本 Publication 保留继续由真实目录集成测试覆盖。生产加载、包身份、初始化事务与去重规则没有改动。

本机验证：

- 一次性 `postgres:17-alpine` + tmpfs，设置 `WORKSPACE_TEST_POSTGRES_DSN` 后运行 `go -C backend test -count=1 ./internal/data/workspace/gormrepo -run '^(TestDefaultResource|TestDefaultCatalog|TestDefaultConnector|TestDefaultSkill|TestLocalResourceDirectory)' -v`：8 项通过，包耗时 18.482s。
- 同一组真实 PostgreSQL 测试加 `-race`：8 项通过，包耗时 203.616s；全量安装用例 196.53s。测试容器已清理。
- `make test`、`make build`、目标包普通测试、`gofmt`、`git diff --check`、暂存后空白检查和 `cmp -s AGENTS.md CLAUDE.md` 通过。未配置 DSN 的普通全量测试中的 Skip 不算数据库验证。

本次不发布服务或改变线上配置。远端合并检查结果以 PR 的当前提交状态为准。
