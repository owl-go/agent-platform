# 专家资源包与领队协作契约

状态：当前实现；验收命令和环境边界记录于[执行记录](../tickets/expert-package-team-refactor-execution.md)。产品依据为[已批准规格](../product/expert-package-team-refactor.md)和 [ADR-0046](../adr/0046-portable-experts-and-lead-coordinated-teams.md)。本文件不宣称已部署或通过真实模型质量验收。

## 定义与 authoring

专家使用展示名称、Introduction、一份权威 Markdown Guidance、可选头像、最多三条开场提示及资源绑定。简介和开场提示不注入角色指令。当前 API、编辑器和创建 Skill 使用 `guidance`；旧结构化输入字段保留已弃用的协议编号以通过兼容检查，非空旧字段写入被明确拒绝；旧输出编号不填充。历史读取和迁移适配保留。新保存的定义不持有模型、Runtime 或自动标签。

专家团仅由 Administrator 创建、导入、复制和维护；其所有者才能维护，自定义团队对所有已认证 User 可读。目录原件不可修改。成员是团队拥有的独立定义，稳定 ID 和手工责任 labels 随重排保持；源专家编辑或删除不会改变副本。领队必须明确指定，旧管理员团队没有领队时不能开始新协作。

会话式预览复用相同 profile 校验。预览支持所有者读取和带版本的修订，确认专家/专家团与创建记录在同一 PostgreSQL 事务中提交；并发或丢失响应后的再次确认复用同一资源。团队建议、确认及重放均检查 Administrator 权限。保存不新建会话。上传头像通过认证 profile 写入及资源版本 CAS，实际字节校验复用头像格式和像素边界；读取沿用资源可见权限。头像字节随不可变定义保存，不开放任意对象读取。

## ZIP schema 1

入口 `.plugin/plugin.json`；`agents/*.md` 保存角色指引，`skills/<key>/` 保存包内技能，`avatars/` 保存头像。清单严格拒绝未知、重复或尾随 JSON 字段。

```json
{"schema_version":1,"id":"example.reviewer","version":"1.0.0","kind":"expert","expert":{"name":"Reviewer","introduction":"Review evidence","guidance_file":"agents/reviewer.md","starter_prompts":["Review this proposal"],"bundled_skills":["skills/review"],"connectors":[{"source":"example-tools","kind":"mcp","version":"1.0.0"}]}}
```

团队清单的 `team` 包含 name、introduction、core_capability、lead_member_id 和 members；成员声明 id、name、labels 和独立 expert profile。头像使用 `avatar_file`，不能与图标字段同时声明；包不接受 SVG 或账号、凭据、授权及模型设置字段。

ZIP 压缩和展开体积各不超过 100 MiB，最多 4000 项。拒绝绝对/绕过路径、大小写冲突、重复条目、链接和特殊文件；所有文件必须被声明引用。技能复用 Skill Store 校验和规范化，保留可执行位，不注册到全球技能目录。头像不超过 2 MiB、单边不超过 4096 像素、总像素不超过 16 Mi。

稳定身份、语义版本和规范化内容摘要确定包修订。相同三者重导入幂等，同版本异内容冲突；升级需更高版本及当前资源版本 CAS，可绑定目标资源防止误升级。另命名副本具有独立安装身份。导出先检查资源权限，再读取被冻结的技能内容与连接器依赖；不含作者凭据或 Provider URL。当前未编辑的已导入修订保持原清单身份；编辑后的导出使用自定义身份和资源修订版本。

目录加载冻结并校验完整 Catalog 后再进入初始化事务，沿用默认资源锁和账本；专家与团队共用 `experts/`。包内对象通过逻辑 Key、Size、SHA-256 校验存储。用户导入失败或幂等重放清理本次新写的对象；受管理目录对象按摘要复用。

## 执行 schema 3

新专家团响应冻结领队、成员定义、执行用户的 Personal Settings 和资源版本。单专家不增加协调调用。历史 schema 0/2 保留原顺序语义及私有重试；新 schema 3 的协调器位于现有 Executor 之上，Worker 仍只依赖 Adapter `Describe`/`Execute`，不使用 CLI 原生子代理或内部 MCP 调度协议。

领队完成响应必须是严格的 `delegate` 或 `complete` JSON 控制动作；只接受冻结 roster 中的成员。每个真实调用使用独立 UUID、上下文、Workspace、凭证目录及原生状态；最多 20 次真实调用，最多 3 位成员同时执行，最多 7200 秒主动执行。没有模型调用的排队任务不计次数。重复委派仍有独立 task/invocation 身份。

成员结果作为有界支持材料交给领队，不进入正式回答。平台同时从成员调用前后的 Workspace 生成增改删摘要，使用该调用的完整凭据集合脱敏路径；每个任务最多 20 条，每条路径最多 200 字节，截断会明确标记。每个失败的原任务最多修复一次，修复调用仍计入总数；必需任务未解决就失败。并行文件修改基于同一基线校验合并；完整目录与文件互换可合并，竞争的后代路径仍产生冲突。冲突保留贡献任务和路径，必须由领队明确选择贡献者或确认自己的修复。仅最终成功可提交 Workflow Workspace；取消或失败丢弃临时目录与原生状态。

连接器依赖解析到执行 User 自己的 active Installation、精确版本和 Authorization，CLI 同时沿用精确 Bundle × Runtime Digest Conformance。缺少依赖或失效授权明确阻止选择/执行；包导入不授予权限。成员仅收到自己冻结的默认资源和 composer 的显式添加/排除。

每次实际调用在 User/Department 账户锁下独立准入和结算。可选响应 Credit 准入预算在新响应冻结，未设置或零表示不限。已有调用的实际结算可以超过估算或预算，之后阻止新调用；失败/取消中已报告用量仍结算，无用量且无完成响应的调用释放预留。终态事实和账本同事务，UUID 防止重复结算。

主动计时只有全部执行中的调用都等待审批时暂停；有一个仍运行便继续计时。审批沿用 15 分钟有效期、所有者、命令摘要、账号身份和单次消费绑定。取消关闭排队/执行中的任务和待处理审批，不允许晚到事件或成功提交。新策略 Worker 中断后明确失败，保留已持久化用量和安全操作事实；用户决定重试，不自动重放不确定外部操作。

任务面板从持久状态和事件展示冻结 roster、领队、真实调用、必需/修复、排队与终态、可展开成员结果、文件冲突和总消费。查询与流接口使用相同安全投影，隐藏控制 JSON、内部模型与费率、Token、凭据、原始 reasoning 和 tool payload。

## 迁移与恢复

新增 migration 75–87，历史迁移不修改。旧结构化文字按确定标题无损转换，旧响应、Run 快照和账本不改写。migration 78 直接删除普通 User 私人团队定义，清除 mutable Session/Workflow/draft 选择，保留所有者历史响应和原快照重试；身份退休记录只保存团队/用户 ID，用于拒绝陈旧选择重新绑定。不得公开或转移旧私人团队。

数据库备份恢复能够恢复被删除的定义；应用版本回退不能撤销删除，旧程序也不能恢复新策略执行。前向修复须保留迁移账本和不可变修订，修复新定义/绑定，不重写历史或消费。任何线上删除前应通过已有发布流程准备数据库备份；本地 PostgreSQL 演练不是生产备份或部署证据。
