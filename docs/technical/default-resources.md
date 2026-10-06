# 默认资源分发与初始化

状态：从仓库目录分发完整无凭据资源，通过 API/Worker 现有启动入口自动初始化。目录原版沿用现有默认资源权限；初始化不代表外部供应商、模型引擎或当前 Linux Worker 已通过验收。

## 分发内容

正式资源的权威来源为顶层 `connectors/`、`skills/`、`experts/`。根 `resources.json` 仅记录 Catalog 版本 `1.0.0`，不列举资源；新增子目录自动发现，无需修改中心清单。当前内容来自已有 Platform Experts/Skills 和有效 Connector Publications 的无凭据导出，只保留定义，不分发账号、User Installation/Authorization、模型连接、凭据或原环境 Conformance 记录。

| 目录 | 内容 | 稳定身份 |
|---|---|---|
| `connectors/<source>/package/` | metadata、图标、恰好一个模式 manifest、伴随 Skills、CLI bundle | source + package version + 规范化 SHA-256 |
| `skills/<name>/` | SKILL.md、资源、resource.json | resource.json 的 key / version |
| `experts/<name>/expert.json` | 完整结构化指导、图标、Skill Keys | key / version |

纯包目录也支持直接放在 `connectors/<source>/`；存在 `package/` 时仅该子目录进入安装包，旁边的构建工具不会混入。`scripts/connectors/` 是构建、测试与发布工具，**不参与扫描**。构建并验证真实包后更新正式目录；同 source 别名目录拒绝导入，避免将脚本副本或测试包重复加入目录。

- 8 个 Expert：旅行计划与行程规划师、UI设计师、产品管理专家、亚马逊广告投放执行专家、亚马逊listing 文案优化大师、跨境电商专家、跨境运营专家、AI 工作流架构师。保留图标、简介、完整结构化指导以及旅行专家的 Skill 绑定；原部署 UUID 转为稳定 Skill Key。
- 9 个附加 Skill：旅行规划与行程安排、前端设计、grill-me、腾讯会议、腾讯问卷、PPT 一键生成大师、PDF图片文字提取、周报生成助手、PDF 文档处理。目录保留所引用的脚本和参考文档；个人下载来源 `_skillhub_meta.json` 不分发，机器路径使用通用占位符。
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

CLI 包包含固定可执行 Bundle，MCP 包保留供应商 Endpoint/变量引用及伴随 Skill；现有第三方许可和参考资料保留。原后端嵌入 ZIP 已移除，业务目录随服务镜像分发，不再膨胀服务二进制。目录迁移前逐项比较了 37 个资源的规范化内容，原版本身份摘要保存在 `backend/internal/defaultresources/testdata/migration-identities.json`，避免布局变化生成不同修订。

## 启动与升级

`make install` 和 `make deploy` 的构建门禁自动执行 `make resources-check`；Docker 构建再校验并将三个目录复制到镜像 `/app/resources`，默认 `AGENT_WORKSPACE_RESOURCE_ROOT=/app/resources`。目录文件由非 root 服务用户读取，不在运行时执行构建或下载。修改任何目录会触发 API/Worker 镜像更新；本地 Go 运行从当前目录向上发现 `resources.json`，指定根路径后错误时不会回退另一份资源。

API/Worker 先完成账号 Bootstrap，再调用 `EnsureDefaultResources`；资源归属为唯一 Bootstrap Administrator。加载先冻结并校验全部定义、归档字节和绑定，错误不进入数据库写入。初始化事务使用同一个 PostgreSQL transaction advisory lock，避免两个进程、重复启动和多实例同时建立副本。继续使用 Migration `000072_default_resource_seeds.sql` 的账本，既有 Migration 不修改。

目录组包拒绝符号链接、特殊文件、越界路径和超限内容；JSON 严格拒绝未知字段。Skill 继续使用既有 ZIP 安全规范化/资源验证，名称从 SKILL.md 的 display_name 读取；Connector 使用公共 Package Parser，包括不可变 CLI Bundle 校验；Expert 使用既有 Domain Input 校验。对象写入先校验 Size 和 SHA-256，数据库只保存逻辑 Object Key。对象键基于内容摘要；数据库提交失败可能留下没有引用的不可变对象，重试复用同一内容，不把失败记录标为已完成。

组包文件权限规范化为 0644 或 0755，仅保留 Git 管理的可执行位；Git archive 的 tar.umask、工作区 umask 和镜像 COPY 的读写位差异不改变 Skill 摘要。

默认 Expert/Skill 原版使用稳定 `system_key` 与现有 `system_managed` 权限，无法编辑或删除；新需求以另名自定义资源表达。发布者改变内容须增加对应资源版本；同一版本出现不同内容会失败并阻止启动。更新保留 ID、递增资源修订，历史执行快照不回写。初始化不能把 Catalog 版本号视作已经验证的运行能力。

首次遇到同名 Bootstrap Administrator 自建资源时记录为非受管理资源：不修改、不复制、不在以后升级时接管。User-private 资源不参与同名查找，也不被修改。默认 Expert 引用被保留的管理员 Skill 时，后续该 Skill 若消失会明确报错，不静默绑定悬空 ID。

Connector Publication 已存在但不属于当前默认初始化时，保留其活动修订与状态；同 source/version 的私人包摘要冲突也不提升为平台发布。受管理 Connector 升级只替换活动修订，保留管理员显式 disabled 状态，并用 publication version 防止并发覆盖。

由旧脚本创建的同 source Publication 不产生第二条记录；相同 source/version/摘要的 Revision 复用。受管理资源内容更新须增加自身 version，同版本不同内容拒绝提交；重复 Key、同类同名、未知 Skill Key，以及三个内置 Skill 的 Key/名称在加载时拒绝。移除目录不删除数据库资源、用户安装/授权或历史快照。

## 可用性与授权

9 个 MCP 发布定义为 available，仍须遵守既有用户安装、配置、测试与授权边界；外部服务可用性另验。11 个 CLI 发布初始状态为 disabled，不生成用户安装、授权、审批或 Conformance 记录。

CLI 发布服务在结构校验之外查询当前数据库的 exact Bundle SHA-256 × Runtime RepoDigest Conformance。没有真实通过记录就拒绝发布。各 `scripts/connectors/<source>/README.md` 与发布脚本描述当前 source build → Worker Linux + runsc 验证 → Stage/Publish 路径；从 `main_temp` 部署候选版本后在受控主机执行，使用本安装的私有配置及证据目录。不能把原部署的证据行、Runtime 可用状态或用户凭据复制过来使包可用。

变更目录包时增加对应资源版本，执行 `make resources-check`、PostgreSQL 初始化/并发/升级/冲突/失败测试和后端门禁。包摘要自动从规范化内容计算。供应商授权、Linux + gVisor、精确 Runtime Digest 及全新服务器部署仍需独立证据。
