# 钉钉项目（Teambition）CLI Connector Package

固定上游 `@tng/teambition-mcp-cli@0.3.3`，使用 npm SHA-512 与两个 Linux 资产 SHA-256 校验，无安装脚本执行。官方 Skill 2.0.2 下载包单独固定 SHA-256；保留其业务编排及两份 TQL 参考。本包的来源是 `teambition`，与钉钉 DWS `dingtalk` 独立。

上游文档：[Teambition Skill 使用指南](https://open.teambition.com/docs/documents/6a561ee8662d4dfb252d0bb8)。公有云 CLI 连接 `https://open.teambition.com/api/mcp/v2`。CLI 动态发现帮助与命令；本文档不把服务端目录的变化直接转为授权策略。

## 包与授权

包内只开放 `capabilities.json` 的 19 项策略：官方文档/Skill 明确出现的项目、任务查询，文件链接查询，任务创建/移动、发表和读取任务评论，只读文档检索、工具目录/Schema 及帮助。任务创建/移动与发表评论是高风险操作，包括其叶子 `--help`，需平台具体 target 和一次性批准。状态更新、成员管理及其他动态操作尚未进入本修订；获取相应账号的最新工具目录并审阅后再发布新修订。

版本 0.3.7 使用官方浏览器 OAuth + PKCE：点击「连接」跳转 `account.teambition.com/oauth2/mcp/authorize`，平台以动态注册的 Client ID 和 S256 challenge 绑定 HTTPS 回调。state 加密绑定 User 和单次 Flow；收到 code 后，由原 User 的授权轮询原子领取并交换，不允许回调重放或并发重复交换。OAuth 凭证由平台加密保存，refresh token 单独保存，不交给 Runtime。无需手工申请或填写 UserToken。

上游原版 CLI 使用 OS keyring 保存本地 OAuth，远程 Linux Runtime 无此钥匙串。本包保留原版 CLI 的发现与命令解析，通过单次进程的 127.0.0.1 HTTP 适配器连接固定 HTTPS MCP：CLI 只持有随机本地路由 nonce，适配器校验 nonce 后替换为短期 OAuth Bearer。路由 nonce 不发给 Teambition；只转发受控 MCP headers，拒绝任意路径／方法，不跟随重定向。临时 HOME、连接及子进程执行后清理。旧 `user_token` 仅保留为历史授权的执行兼容，不是新连接入口。

平台安装与连接由各 User 独立完成；发布流程不替 User 完成第三方账号授权。浏览器入口和模拟服务协议测试不能替代真实账号项目查询。

包内 `platform status` 仅验证凭证格式，返回 `configured` 与 `verification: credential_shape_only`，不验证 Token 有效性或业务权限。上游 `--help` 会请求动态 MCP 目录，所以 wrapper 的总览帮助是本修订离线策略列表；Conformance 还应检查实际 native `--version`。版本 0.3.4 使用 [Teambition 官网 favicon](https://www.teambition.com/favicon.ico) 的 128px PNG，SHA-256 为 `941a5a0aa5c3609ead813267836a717d369c1e48321fd69eae1d985df4674908`；包内 SVG 嵌入同一 PNG，平台 `DisplayIcon` 将该品牌图标投影为前端支持的 PNG Data URL。

## 构建与检查

在仓库根目录执行，使用当前生产 Registry RepoDigest 与真实 Node 版本：

```bash
npm pack @tng/teambition-mcp-cli@0.3.3 --pack-destination /tmp --ignore-scripts --silent
curl -fL -o /tmp/teambition-skills.zip \
  https://file.teambition.net/public/teambition-cli/skills/teambition/teambition.zip
python3 scripts/connectors/teambition/build.py \
  --npm-tgz /tmp/tng-teambition-mcp-cli-0.3.3.tgz \
  --skill-zip /tmp/teambition-skills.zip \
  --runtime-image <registry/repository@sha256:digest> \
  --runtime-version <exact-node-version> \
  --output <absolute-output-directory>/teambition-0.3.7.zip
go -C backend run ./cmd/connector-package-validate <absolute-package-path>
node --test scripts/connectors/teambition/*.test.cjs
python3 -m unittest discover -s scripts/connectors/teambition -p 'test_*.py'
go -C backend test ./internal/connectorpackage/... ./internal/cliconnector/...
```

构建器同时输出 `.source.zip`。它用 `cli-connector-bundle` 调用现有 `ZIPPackageBuilder`，使平台从同一 source ZIP 构建的 bundle 与外层包逐字节一致；包和 source ZIP 的产物放在不提交的输出目录。构建器输出 `conformance: not_run`，只有真实生产 Worker 执行后才能记录通过。

## 平台发布

代码先提交、推送并集成到 `main_temp`，通过适用门禁。将同一输出包、source ZIP 和发布脚本传到已授权的部署主机，在主机读取其现有平台环境配置；不要将环境配置或 Token 拉到仓库。

```bash
python3 publish.py \
  --config /srv/agent-workspace/config/platform.env \
  --package <package-path> \
  --source <source-zip-path> \
  --evidence-directory <release-evidence-directory>
```

脚本用部署管理员的 OIDC + PKCE 登录，不输出登录材料。在部署主机上传大包时可传 `--api-base <已核实的本机容器 API origin>`，避免公网回流占满普通 API 的请求期限；默认使用公网平台 origin。API 地址必须属于本次授权的平台，鉴权和服务端校验照常执行。通过现有管理员 CLI upload/publish API 让 Worker 执行 ZIP 构建和 Linux + runsc Conformance，校验返回的 exact bundle SHA-256 与当前 Runtime Digest，通过 Stage/Publish API 发布 managed Publication，确认没有用户使用后以 Delete API 软删除临时 legacy Definition，保留原有 Conformance 行与历史证据。停用仍会在管理员目录显示，不能代替清理。再次运行或仅更新包内展示资产时复用已有 exact bundle/Runtime 验证，不重新创建 Definition；失败保持平台状态，不写入伪造的通过记录或直接修改数据库。`source.zip` 与外层包不一致时在 Stage 之前拒绝。

发布证据只包含非敏感的 build、stage、publication 和 health 响应。Conformance 通过仅证明这个 bundle 在这个 Runtime 可启动；真实 Teambition 账号授权、项目查询、任务写入、业务权限和动态帮助仍需实际账号验证。其他 Runtime Digest、私有部署以及未完成的真实第三方账号授权不由本修订宣称已验证。

## 已执行的发布证据（2026-09-30）

平台 Publication 当前为 `available`，修订 `8bd71134-c739-477e-9806-fbe733a2a448`、版本 `0.3.4`，复用 0.3.3 的同一 bundle/Runtime 运行验证。生产 Worker 的 exact bundle/Runtime 验证及证据边界见 `docs/technical/connector-platform.md` 的 Teambition 段落。发布构建来自 `main_temp`；首次发布的临时 legacy Definition 当时仅 disabled，导致管理员目录出现第二张卡片；该问题由后续 0.3.4 修订的软删除流程修复。平台用户使用 managed Publication。未替任何 User 安装或授权。

实际通过：Node launcher 三项边界测试、Python 构建器三项测试、目标 Go 包测试、`make test`、`make build`、最终 ZIP 的 `connectorpackage.Parse`、生产 Worker Linux + runsc Conformance，以及 native `--version`、无凭证状态、未放行命令拒绝检查。功能分支和 `main_temp` 均已执行测试与构建；同一输入再次构建的包逐字节一致。未执行真实 Teambition 账号业务 API、其他 Runtime Digest、私有部署、完整模型 Runtime Production Conformance；未改 Web 或 Runtime 镜像，对应构建门禁不适用。

0.3.4 修复验收：公共 catalog 只有一个 Teambition Publication；管理员 legacy catalog 没有临时构建条目。原 Definition 已软删除，Conformance 行仍在。再次运行发布脚本返回同一修订，未新增 Definition。Playwright 检查「全部状态」下的实际卡片，只有一个「钉钉项目」，品牌 PNG 加载成功（128×128），显示版本 0.3.4。六项 Python 测试、目标 Go 测试和功能分支／main_temp 的 `make test`、`make build` 均通过。图标 API 镜像从 main_temp `1e120f7` 构建并通过容器健康检查；未改变 Runtime 镜像。非敏感响应与卡片截图保存在忽略目录 `outputs/teambition/evidence-0.3.4`。


## 浏览器 OAuth 修复验收（2026-10-01）

版本 0.3.5 已发布为 available，Revision `1c4535f5-421b-4021-b37f-8a47a0761552`，规范化包 SHA-256 `2e59ed92430d83c5eb524384fdd1d80903bde637b5e490a664c99fe2274c1ad3`，bundle SHA-256 `72a297caf2bdd303db7a4cbad2c1a22f3fdfcfba266278069b6d25636eff7d12`。生产 Worker 对本 bundle × Runtime `sha256:e4e3a508e82f296dd8dce1e40db0ddda639bc2a44bb1b45439cedce63ae12d0b` 完成 Linux + runsc Conformance；同一镜像中五项 OAuth 传输测试全部通过，包括原版 Linux CLI 经模拟 OAuth upstream 发现工具。发布重跑返回同一 Revision，没有新建临时 Definition。

实际通过：Go 目标包与全仓测试／构建，真实 PostgreSQL Repository 集成测试，395 项前端测试及 typecheck／生产构建，9 项本地 Node 测试（含原版 macOS CLI），6 项 Python 测试和最终 ZIP Parse。API、Worker 与 Web 从集成后的 `main_temp` `2eb7758` 发布并通过健康检查，Runtime 镜像未变。未携带 JWT 的伪造 callback 得到 400；邻接路径和其他方法仍要求平台认证。

线上 Playwright 以原有 0.3.4 Installation 点击连接，实际先升级至 0.3.5，再动态注册并打开 `https://account.teambition.com/login`，页面显示「使用钉钉扫码授权」。目录只有一个钉钉项目卡片，128px 品牌 PNG 加载成功，没有 UserToken 输入框。非敏感响应、卡片截图和 Linux 测试记录在忽略目录 `outputs/teambition/evidence-0.3.5`。浏览器入口已验证；第三方账号仍待用户扫码，未宣称真实账号授权、业务查询／写入或完整模型 Runtime Production Conformance 已通过。

## MCP2 请求头修复（0.3.6）

真实会话报告 `-32020: 缺少必需的 Mcp-Method 请求头`。0.3.5 的 OAuth 适配器丢弃了原版 CLI 发出的 `Mcp-Method` 与 `Mcp-Name`。0.3.6 将这两个 MCP2 路由头加入明确的转发白名单；Authorization 仍由平台短期 OAuth grant 替换，Host、Cookie 和无关头仍不转发。

回归服务现在按实际服务端要求拒绝缺少 `Mcp-Method` 的请求，并验证它与 JSON-RPC method 一致、`Mcp-Name` 与 resource URI／tool name 一致。固定版本原版 CLI 的工具目录、资源读取和工具调用均经过此检查。修复前复现了截图的同一错误；修复后协议测试通过。原有 17 项命令策略、scopes、身份及 Runtime Digest 均不变，已授权 Installation 升级无需重新扫码。

0.3.6 于 2026-10-01 从集成 `main_temp` `7d43ff7` 的 source ZIP 构建并发布，Revision `d35988c8-d7d4-4a0c-9f06-83c75a7cd731`，规范化包 SHA-256 `e19d105fc657597ce77fba24bb2c06ef02c535aea6e4552761df23fe9b982f6c`，bundle SHA-256 `498ded3b85ef22bbf063c01626a0def6d39fcdcf89a6bd3ae0620cfb226245fb`。生产 Worker 对同一 bundle × 原 Runtime Digest 完成 Linux + runsc Conformance；该镜像中的六项传输回归全部通过，含原版 Linux CLI。功能分支与 main_temp 的 `make test`、`make build`、10 项 Node 测试、6 项 Python 测试和最终 ZIP Parse 通过；目标 Go 包测试通过。重跑发布返回同一 Revision，目录只有一个正式卡片、有品牌 PNG、无 staging Definition。目标 Installation 已升级 0.3.6，保留原 selected Authorization，并刷新原会话选择。仅更新 CLI bundle，API、Worker、Web 和 Runtime 镜像没有变更；未运行 Web 门禁或完整模型 Runtime Production Conformance。非敏感发布证据位于忽略目录 `outputs/teambition/evidence-0.3.6`。

真实账号验证：原所有者会话在刷新选择后执行 message 683，最终 Snapshot 的 bundle SHA-256 为上述 0.3.6 值，六项连接器证据均为 succeeded。实际 `user me` 返回 1 条，`project query --my` 返回 0 条，按当前账号负责且未完成条件查询第一页任务返回 0 条；task query 帮助先用于确认参数，未替代业务查询。未再出现 Mcp-Method 缺失错误，无写操作。此证据仅覆盖当前授权账号和上述只读查询，不宣称任务写入、其他账号或组织已验证；0 条是该次查询返回数量，不是平台错误。

## 任务评论能力（0.3.7）

当前账号的官方服务端目录及文档版本 1.8.4 审阅确认 `teambition.task.comment` 对应 `task comment`（task:write、非幂等），`teambition.task.activity` 对应 `task activity`（task:read、只读）。0.3.6 缺少这两个命令的策略，导致评论在调用前被阻止；0.3.7 增加独立的高风险评论与低风险动态查询 capability，未扩展 OAuth scopes、身份或 Egress。官方评论对象只接受 taskId，不能以项目 ID 代替；本次未发现可确认的项目级评论命令。

参数按官方 Schema／文档审阅：taskId 和正文必需；Markdown、@提及、已上传 fileTokens 仅使用明确授权且已解析的值。发表要求具体 target 与平台一次性批准，评论后通过 task activity 按返回 ID 核验；UNKNOWN_OUTCOME 时先查询，禁止自动重发。companion Skill、wrapper 白名单、manifest scope 映射和元数据共同更新。回归先复现 wrapper 拒绝评论命令，再验证新增评论高风险、动态低风险与拒绝原始写 Tool／未审阅删除更新。原版 CLI 的评论／动态 Remote Tool 协议载荷在隔离模拟服务验证，不把该模拟写入当作真实发表评论证据。

0.3.7 于 2026-10-01 从集成 `main_temp` `d91b9ce` 构建并发布，Revision `c6bd49d7-96bf-4fc4-a977-fe1af44cb4f2`，规范化包 SHA-256 `90d6d31b7151061dc247494f3b1e520fbd0c411c89c49b856a7d8fdd5ff48f3d`，bundle SHA-256 `db28b46d2ba857c91ccb7251c99ce0d809212ceb0a35fcb77cfa2d1a5b105e99`。生产 Worker 对同一 bundle × 原 Runtime Digest 完成 Linux + runsc Conformance；12 项 Node 回归在本地及该隔离 Linux 镜像通过，7 项 Python 测试、目标 Go 测试、功能分支与 main_temp 的 make test／make build 和最终 ZIP Parse 通过。再次发布复用同一 Revision，目录只有一个带品牌 PNG 的正式条目，无临时 Definition。所有者 Installation 已升级 0.3.7 并保留选中的有效 OAuth Grant，原会话选择已刷新。没有 API／Worker／Web／Runtime 镜像变更，不适用 Web 或镜像构建门禁；未运行完整模型 Runtime Production Conformance。发布与只读审阅证据在忽略目录 `outputs/teambition/evidence-0.3.7`。未因诊断发表评论或发起写审批。

真实只读验证：所有者原会话 message 701 冻结的是上述 0.3.7 bundle，`task --help` 确认 comment／activity 公开命令存在；对本会话已确认任务 ID 的 task activity 实际调用成功（返回码 200），三项连接器证据均 succeeded。动态返回 2 条、其中评论 0 条，无下一页；虽然帮助声称默认仅评论，此次服务结果包含非评论动态，必须按动作类型／返回评论 ID 验证，不能把动态条数当评论条数。未执行发表评论、写命令帮助或写审批；真实评论写入／通知效果未验证。非敏感结果保存为 comment-read-result.json。
