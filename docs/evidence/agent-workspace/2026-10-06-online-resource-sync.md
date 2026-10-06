# 线上资源目录同步：2026-10-06

对线上 `https://47-237-108-63.sslip.io` 做只读导出与完整内容核对：20 个活动 Connector Publication、8 个 Platform Expert、12 个 Platform Skill。资源清单、线上包摘要、目录包摘要和文件树摘要保存在[核对清单](2026-10-06-online-resource-sync.json)。没有导出私人资源、账号、密钥、用户安装/授权或 Conformance 记录。

- 20 个连接器位于 `connectors/<source>/package/`。读取并校验全部活动包的数据库 SHA-256，再比较全部文件名、内容和可执行位；CLI bundle、图标和伴随 Skill 均已保留。19 个包与目录包摘要一致，飞书仅有一处参考文档中的机器路径已改为 `<LOCAL_DATA_ROOT>`。
- 9 个业务 Skill 位于 `skills/<name>/`。脚本、模板和参考文件全部保留；8 份个人下载元数据 `_skillhub_meta.json` 不分发，PPT Skill 的 4 个参考文件使用通用机器路径。除这些明确处理的内容外，文件逐字节一致，可执行位一致。
- 3 个内置创建 Skill 继续位于 `backend/internal/systemskills/systemskills.go`，其 SKILL.md 与线上逐字节一致。线上归档与本地生成 ZIP 的封装差异不代表文档内容差异；不在业务目录再建立同名副本。
- 8 个专家位于 `experts/<name>/expert.json`，逐字段比较名称、图标、背景、简介、核心能力、流程、输出标准和注意事项，全部一致。旅行专家的线上 Skill ID 已对应为稳定 Skill Key；其余专家没有 Skill 或 Connector 绑定，未推断新增绑定。

本次从线上实际包补齐核对依据，确认已有目录没有遗漏或功能内容差异，因此保留既有版本和包身份。`scripts/connectors/` 仍只存放构建/发布工具，不参与目录扫描；重复启动沿用现有去重机制。

验证：`make resources-check`、`go -C backend test -count=1 ./internal/defaultresources`、`git diff --check` 和 `cmp -s AGENTS.md CLAUDE.md` 通过。只读导出完成后，线上仍为 20 个 Publication、35 个 Revision、12 个 Platform Skill、8 个 Platform Expert；Publication 指纹与导出前一致。本次仅补充核对清单和文档，没有服务发布、数据库写入、外部账号业务调用或新的 Runtime/首次安装验收。
