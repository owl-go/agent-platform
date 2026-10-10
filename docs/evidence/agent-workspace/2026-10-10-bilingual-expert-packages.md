# 专家包双语展示文案（2026-10-10）

用户明确要求 `plugin.json` 中的名称、简介、常用提示提供中文和英文。沿用 `codex/expert-team-refactor`；本次只扩充资源包展示元数据，页面/API 默认字段的语言选择行为沿用已有合同。

## 行为与实现

八个随仓专家升级为 1.3.0，原默认中文 name/introduction/starter_prompts 保持；profile 新增 translations.zh-CN 和 translations.en，各有名称、简介及三条按位置对应的提示。专家团和成员 profile 同样支持。旧单语言包可省略该块，仍可解析。双语名称/简介、提示长度和对应数量经严格解析器检查；中文块必须与默认值一致。翻译没有注入 Guidance，不恢复标签或分类。

目录初始化在同一事务中保存托管定义及规范化原包，复用既有 expert_package_imports 表和导出路径。原件导出保留双语清单；原包冲突时整体回滚，非托管定义继续受保护。未修改的用户导入修订沿用原归档；编辑后新自定义包仍只导出该新定义当前持有的默认语言内容。未添加 Migration 或 Proto 字段。

个人 expert-creator 的指引、模板、专家/团队初始化骨架和内置 Create Expert 的 ZIP 指引均同步。八个 ZIP 使用真实包解析器校验、记录和打包，交付目录 `/Users/frank/.codex/artifacts/expert-packages/2026-10-10-bilingual/`。

## 验证

- G-GO：目标包 expertpackage、systemskills、defaultresources、gormrepo 测试、完整 make test 和 make build 通过。
- G-DB：通过 SSH 隧道连接现有 PostgreSQL，为测试临时创建仅有 LOGIN/CREATEDB 权限的角色。每项测试使用新建隔离数据库和合成数据，不读取生产用户内容。实跑 TestManagedDirectoryExportsRetainBilingualSourcePackage、TestDefaultResourcesConcurrentInstallAndUpgrade、TestDefaultExpertPackagesInitializeAndUpgradeBothTypesAtomically 均通过，94.467 秒；另补测源归档摘要冲突与回滚。
- 首次测试封装中的角色清理脚本误替换 SQL 关键词，产生 SyntaxError；Go 测试本身通过。随后修正脚本、移除临时角色并确认无其所属数据库残留。补测的独立数据库和临时角色也已清理。
- 个人技能 quick_validate.py 成功，八个包均含双语字段，Agent/Team 初始化均生成双语占位；占位内容仍必须完成才能打包。
- G-DOC：git diff --check、AGENTS.md/CLAUDE.md 镜像检查通过。

本次双语改动没有改变前端、Runtime 或 Sandbox。没有宣称已完成界面自动语言切换、真人认证导入或模型生成质量验证。

补测源归档摘要冲突/回滚用例实际通过，29.205 秒。所有目标数据库测试均使用合成数据。

## 发布与线上核验

功能提交为 `0cd90f8`，经 `main_temp` 集成并保留并行的保存错误与 PostgreSQL 错误原因修复。`make deploy` 发布不可变来源 `d47860a70acae094ef62e0b9e500a090b30e9e35`，发布编号 `app-20261010T104003Z-00b9bd74`；服务器当前来源目录为 `src.release-app-20261010T104003Z-00b9bd74`。

发布后通过只读核验脚本检查八个托管专家的当前源归档：包版本均为 1.3.0，资源修订均为 2，全部原 UUID 保持不变；解压后的双语清单逐项与本地交付包一致，规范化内容摘要也一致。内置创建技能摘要为 `dc5a62595c70ca5cd8dd290e6a2bd85b822f53c0446caefef106c3a1171d07f6`，与本地更新后的技能一致。

API 与 Worker 均为 healthy，公共 HTTPS 健康、就绪、OIDC 及静态资源核验通过；Migration 台账仍为 102 项。最终核验脚本位于 `/Users/frank/.codex/artifacts/expert-packages/2026-10-10-bilingual/verify-production.py`。这些检查没有替代真人认证后的上传操作或浏览器语言交互验收。
