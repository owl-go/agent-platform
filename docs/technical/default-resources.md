# 默认资源分发与初始化

状态：随服务二进制分发完整无凭据资源，并通过 API/Worker 现有启动入口自动初始化。本地测试证据与生产执行证据分别记录；初始化不代表外部供应商、模型引擎或当前 Linux Worker 已通过验收。

## 分发内容

`backend/internal/defaultresources/assets/manifest.json` 是本次分发的权威清单，包含稳定资源 Key、独立版本、归档路径及归档 SHA-256；Catalog 版本为 `1.0.0`。资源来源为当前平台已有 Platform Experts/Skills 和有效 Connector Publications 的只读导出。只白名单保留定义内容，不导出账号、User Installation/Authorization、模型连接、凭据或现有 Conformance 数据。

- 8 个 Expert：旅行计划与行程规划师、UI设计师、产品管理专家、亚马逊广告投放执行专家、亚马逊listing 文案优化大师、跨境电商专家、跨境运营专家、AI 工作流架构师。保留图标、简介、完整结构化指导以及旅行专家的 Skill 绑定；原部署 UUID 转为稳定 Skill Key。
- 9 个附加 Skill：旅行规划与行程安排、前端设计、grill-me、腾讯会议、腾讯问卷、PPT 一键生成大师、PDF图片文字提取、周报生成助手、PDF 文档处理。ZIP 保留所引用的脚本和参考文档；个人下载来源 `_skillhub_meta.json` 不分发，机器路径使用通用占位符。
- 3 个创建 Skill：继续复用 `internal/systemskills` 的 `Create Skill`、`Create Expert`、`Create Connector`，共 12 个 Skill。

| Connector source | 模式 | 包版本 |
|---|---|---|
| feishu | CLI | 1.0.96 |
| dingtalk | CLI | 1.0.64 |
| wecom | CLI | 1.4.2 |
| notion | CLI | 0.23.15 |
| teambition | CLI | 0.3.7 |
| modao | CLI | 0.1.1 |
| picset-ai | CLI | 0.1.0 |
| kling-ai | MCP | 0.1.0 |
| linear | MCP | 1.0.0 |
| pixso | MCP | 1.0.0 |
| camscanner | CLI | 1.1.8 |
| ai-hive | MCP | 0.3.0 |
| caoliao | MCP | 1.0.0 |
| github | CLI | 0.1.0 |
| openboost | CLI | 0.1.0 |
| xiaoe | MCP | 0.1.0 |
| moka-hr | CLI | 0.1.0 |
| tianyancha | MCP | 1.0.0 |
| pkulaw | MCP | 1.0.0 |
| mindbye | MCP | 1.0.0 |

这些定义和归档不是对全部供应商能力的新增验收声明。CLI 包包含固定可执行 Bundle，MCP 包保留供应商 Endpoint/占位凭据及伴随 Skill；包内现有第三方许可和参考资料随归档保留。当前分发 ZIP 合计约 107 MiB，会增加 Git 下载与服务二进制大小。

## 启动与升级

API/Worker 先完成账号 Bootstrap，再调用 `EnsureDefaultResources`；资源归属为唯一 Bootstrap Administrator。初始化事务使用同一个 PostgreSQL transaction advisory lock，避免两个进程、重复启动和多实例同时建立副本。Migration `000072_default_resource_seeds.sql` 只追加初始化账本，既有 Migration 不修改。

Skill 继续使用既有 ZIP 安全规范化/资源验证；Connector 使用公共 Package Parser，包括不可变 CLI Bundle 校验；Expert 使用既有 Domain Input 校验。对象写入先校验 Size 和 SHA-256，数据库只保存逻辑 Object Key。对象键基于内容摘要；数据库提交失败可能留下没有引用的不可变对象，重试复用同一内容，不把失败记录标为已完成。

默认 Expert/Skill 原版使用稳定 `system_key` 与现有 `system_managed` 权限，无法编辑或删除；新需求以另名自定义资源表达。发布者改变内容须增加对应资源版本；同一版本出现不同内容会失败并阻止启动。更新保留 ID、递增资源修订，历史执行快照不回写。初始化不能把 Catalog 版本号视作已经验证的运行能力。

首次遇到同名 Bootstrap Administrator 自建资源时记录为非受管理资源：不修改、不复制、不在以后升级时接管。User-private 资源不参与同名查找，也不被修改。默认 Expert 引用被保留的管理员 Skill 时，后续该 Skill 若消失会明确报错，不静默绑定悬空 ID。

Connector Publication 已存在但不属于当前默认初始化时，保留其活动修订与状态；同 source/version 的私人包摘要冲突也不提升为平台发布。受管理 Connector 升级只替换活动修订，保留管理员显式 disabled 状态，并用 publication version 防止并发覆盖。

## 可用性与授权

9 个 MCP 发布定义为 available，仍须遵守既有用户安装、配置、测试与授权边界；外部服务可用性另验。11 个 CLI 发布初始状态为 disabled，不生成用户安装、授权、审批或 Conformance 记录。

CLI 发布服务在结构校验之外查询当前数据库的 exact Bundle SHA-256 × Runtime RepoDigest Conformance。没有真实通过记录就拒绝发布。各 `scripts/connectors/<source>/README.md` 与发布脚本描述当前 source build → Worker Linux + runsc 验证 → Stage/Publish 路径；从 `main_temp` 部署候选版本后在受控主机执行，使用本安装的私有配置及证据目录。不能把原部署的证据行、Runtime 可用状态或用户凭据复制过来使包可用。

变更默认包时同步归档 SHA-256 和资源版本，验证全量归档及 Expert 绑定，再执行 PostgreSQL 初始化/并发/升级/冲突/失败测试和后端门禁。当前安装的供应商授权、Linux + gVisor、精确 Runtime Digest 及全新服务器部署仍需独立证据。
