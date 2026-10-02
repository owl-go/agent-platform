# Moka HR 招聘连接器发布证据 — 2026-10-02

## 发布与部署

- 功能提交：`df26ce8`，分支 `codex/moka-hr-connector`；部署集成提交：`899e227`，已推送 `main_temp`。集成保留同日其他连接器的并发改动。
- 正式部署：`moka-hr-20261002-899e227`，由 `scripts/deploy-platform.sh` 执行完整门禁后完成；源目录 `/opt/agent-platform/src.release-moka-hr-20261002-899e227`，Web 目录 `/opt/agent-platform/web/releases/moka-hr-20261002-899e227`，备份 `/opt/agent-platform/backups/pre-moka-hr-20261002-899e227`。
- [平台目录](https://47-237-108-63.sslip.io/resources?tab=connectors)中 `moka-hr` 0.1.0 状态为 `available`，CLI 模式、平台所有者 `user` 身份、`connector_package` 认证 driver。
- Revision：`3eef9151-3311-4570-9779-bac73405ce81`。
- 最终 ZIP 字节 SHA-256：`627b04242822aebb821d01d613984c8701df346cf2f82da8623e2b32ec4357b5`。
- Package SHA-256（实际 `connectorpackage.Parse` 规范化）：`4928abaf8d99dc10bde2b5d6e0fcdc57d6fba616ce6854f920c71574ff8462c3`。
- Bundle SHA-256：`70bef6ad667ba5864f80d4eea2e3457dedbb7d5706f94f3e0acb6da354988763`。
- Node 24.15.0 Runtime：`127.0.0.1:5000/agent-platform/runtime@sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b`。本次部署沿用同一 Digest，未开启新的 Runtime Capability。
- 发布验收 Installation：`7d595025-5006-4a57-8659-caa82ebb84af`，所有者为发布使用的 Platform Administrator；活动 Revision 匹配，状态 `active`，没有 Moka 授权。未给其他 User 安装或写入测试 API Key。

## 上游、授权与覆盖

[Moka 官方 ATS API](https://www.mokahr.com/docs/api/index.html)以及其链接的[旧版 eHR API](https://www.mokahr.com/docs/api/view/v2.html)支持企业 API Key Basic Auth：Key 作为用户名、密码为空，使用 HTTPS。企业 Key 由 Moka CSM 提供，其权限不等同于个人员工权限。本修订采用该官方授权方式；官方企业 clientId/clientSecret 换取 accessToken 的方案未实现。

发现真实[社区 Moka MCP](https://github.com/mingyangsun-sketch/moka-mcpserver/tree/1377d66163cefbd59691a1fd89d9fa96dbf6503b)，固定 commit `1377d66163cefbd59691a1fd89d9fa96dbf6503b`、项目版本 0.1.0、MIT、13 项只读工具。其 PyPI 包查询返回 404，文档推荐 Git 来源 uvx 或自托管 HTTP，没有可直接使用的固定 registry 包或已核实 HTTPS 服务地址。平台现有 stdio assembler 的 registry 安装方式不能直接安装该 Git 实现。因此使用现有 CLI bundle seam 独立实现官方 API 查询，并增加面试列表；没有宣称本包为 Moka 官方 CLI，也没有复制社区运行代码。

包有 14 项只读业务能力：职位列表/详情/字段，候选人搜索/详情/阶段，申请状态，流程、阶段、部门、Offer 字段，人才库列表/候选人以及面试列表；另有 tools/schema/version 三项诊断能力。[操作表](../../../scripts/connectors/moka-hr/operations.md)记录路由、参数、capability、身份和风险。职位列表限公开招聘官网职位，`stage=all` 限 Offer 与待入职阶段，分页仅返回当前页。没有 Moka People、业务写入、raw API、自定义 URL 或 Header 能力。

平台按所有者保存加密 credentials JSON，单次进程只收到 `moka_api_key` 与 `moka_org_id`。密钥不进入 argv、本地持久文件或包。上游与生命周期错误经过原始 Key / Basic base64 精确脱敏；手机号与身份证按字段掩码，其他个人信息不会自动匿名化。连接表单保存不代表企业 API 已验证；`auth`/`status` 用只读部门 API 检查真实连接。

## 已执行验证

- 本地 11 项 Node 协议测试、5 项 Python 构建测试、6 项共享发布生命周期测试通过。协议 fixture 校验方法、路由、Basic Auth、query/body，覆盖全部 14 项业务命令以及拒绝未审阅输入、日期限制、分页、反射脱敏、业务/HTTP 错误和响应限额。
- 最终 ZIP 通过当前 Go `connector-package-validate` 命令实际调用 `connectorpackage.Parse`；`go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...` 通过。
- 功能分支完整 `make test`、`make build`、前端 46 文件 / 423 用例、`make web-typecheck` 与 `make web-build` 通过。合并后针对受影响组件的 151 用例通过；正式部署再次执行完整 Go 测试/构建、前端 46 文件 / 461 用例、typecheck 与生产构建。
- 目标 Linux + runsc Runtime 使用无外网、非 root、只读 Rootfs、512 MiB / 1 CPU / 64 进程限制通过 11 项协议测试。测试使用 `--test-isolation=none`，没有把 macOS 测试当作 Linux 隔离证据。
- Worker 从 source ZIP 实际隔离构建出同一 bundle，对精确 bundle × Runtime RepoDigest 执行平台 Conformance。暂存、发布和 publication-health 返回相同 Digest 与 `conformance_available:true`。
- 临时 Definition `05dc36d5-51d4-49df-80d3-319a14933e5f` 无用户 Enablement 或授权使用，发布器通过管理员 DELETE API 软删除，保留历史 Conformance。Administrator 修订目录与该账号的 User catalog 各恰有一个正式条目；临时构建条目为零，Revision / Installation 图标均投影同一官方品牌 PNG。
- 真实发布重跑复用同一 Revision，Publication version 保持 1；没有重新构建或创建新 Definition。
- 部署中的 Runtime / CLI Builder image smoke、备份校验、API / Worker / Egress Controller / Caddy 健康与 Web 发布检查通过。
- 最终只读检查确认验收 Installation 的 Authorization 数量为零，公开 `/api/healthz` 为 `ok`、`/api/readyz` 为 `ready`。

## 浏览器连接验收

使用 Playwright 实际登录发布管理员自己的 User 视图，确认一个 Moka 正式卡片，其官方图标实际加载为 24×24 PNG。点击详情中的连接按钮，已安装、未授权状态打开企业 API Key（password 输入）与组织 ID 表单，附官方获取指南；两个输入均为空，取消并重新打开仍为空。提交、错误重试、输入清空和旧修订升级路径由组件 fixture 覆盖，线上没有提交 Moka Key。

桌面 1440×1000 和手机 390×844 截图已人工查看。手机 documentWidth 为 390，表单左右边界为 20 / 370，无横向溢出。会话选择器实际加载同一品牌图标；切换未授权 Moka 连接器后到达 `/resources?tab=connectors`。最终截图等待详情关闭并禁用截图期间的动画，避免把过渡帧当作界面状态。

本次部署后其他任务继续从 `main_temp` 发布；最终验收时源目录为 `src.release-tianyancha-region-20261002T1335`、Web 为 `releases/login-entry-05d44769c8a4`。这次实际浏览器和健康检查在后续发布后再次通过，Moka Revision、品牌图标与连接入口仍保留。

## 验证边界与产物

当前没有 Moka 企业 API Key，真实企业授权、模块权限、职位/候选人/面试数据调用未验证。协议 fixture 与平台 Conformance 不代表 14 项业务 API 已在真实账号成功。未运行完整模型 Production Conformance、完整 Sandbox Conformance 或真实远端存储 Conformance；集成测试的 Skip 不计为远端通过。

本地 ZIP、source ZIP、Linux 协议输出、部署日志和不含凭证的发布/目录/浏览器响应位于功能工作区忽略目录 `outputs/moka-hr`，截图位于 `output/playwright/moka-hr`。包内包含 companion Skill 与查询表。远端本次临时 source ZIP、构建 ZIP 与验收文件在复制证据后清理；浏览器上下文关闭。临时构建资源的清理使用平台软删除 API，未硬删除记录或撤销既有用户资源。

## 人力招聘分类调整

同日按用户要求将 `moka-hr` 从“其他”归入“人力招聘”（英文 `HR & recruitment`），市场与已安装视图使用同一分类映射。功能提交 `084e058` 已通过 `main_temp` 集成提交 `2889900` 发布；Web release 为 `moka-hr-category-20261002-2889900`，只发布 Web，没有改动 Connector Revision、安装或授权。

112 项 `ExtensionManager` 测试、`make web-typecheck`、`make web-build` 与 `git diff --check` 通过。`make web-deploy` 使用线上 OIDC 配置再次构建并激活 release。Playwright 实际检查市场与已安装视图，均只有一个 Moka HR 卡片位于“人力招聘”下；390px 手机端 documentWidth 为 390，截图已人工查看，API healthz 为 `ok`。部署日志与浏览器结果保存在分类工作区的 `outputs/moka-hr-category`，截图位于 `output/playwright/moka-hr-category`。
