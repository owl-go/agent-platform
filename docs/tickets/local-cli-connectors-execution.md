# 本地 runc CLI Connector 恢复

2026-10-19；会话分支 `codex/local-deployment`，沿用 `/Users/frank/projects/agent-platform-local-deployment`。基线集成 `origin/main_temp` 的 `f88bc11`。不操作欠费云服务器，不读取或迁移旧账号凭据。

## 范围与验收

- AC-LCLI-1 / G-SEC、G-RUNTIME：仅显式 local/runc 使用个人 bridge 和实际本地 Image ID；生产省略 profile/production 仍强制 runsc/RepoDigest，禁止自动降级。模型容器仅挂载 Broker socket，连接器容器只挂当前 bundle，保持非 root、只读、限额、审批和脱敏。
- AC-LCLI-2 / G-DB、G-API：当前实际 bundle × Image ID 的本地证据独立存储，不继承生产/其他镜像证据；API 与 Worker 分别校验。只恢复受管理默认目录，管理员明确停用与 User 私有授权不被覆盖。
- AC-LCLI-3 / G-RUNTIME：飞书、钉钉、企业微信真实分发包在当前 ARM64 Runtime 启动，`agent-cli`→Unix Broker→独立 Connector Container 完整往返及 canary 脱敏通过。AMD64 仿真另记，不能称原生 AMD64。
- AC-LCLI-4 / G-E2E：本地部署后检查管理员目录每种一个条目、真实品牌图标、安装与未授权连接入口；真实账号授权及只读业务调用独立验收，无账号不能宣称通过。
- AC-LCLI-5 / G-DEPLOY：通过适用门禁，提交推送本会话分支，集成 main_temp 后才构建并切换本地服务；保留原镜像、数据卷、私密备份和恢复点。

## 发现与实现

诊断已复现三个 CLI Publication 均 disabled、原生产证据为零，市场接口过滤 disabled，前端同时隐藏空分类。不能把目录启用当作执行成功。

显式 local 模式装配独立的无网络/无凭据 Docker 启动验证；现成 bundle 经 Object Store 完整性复核，不启用 npm Builder。API 展示实际配置镜像的兼容验证结果，生产查询与 Worker 执行保持分离。Migration 000088 添加本地证据表和初始化恢复标记，默认记录的 seed 修订升级保留标记，管理员变更清除。没有可用 Runtime 时 API 仍可启动，但不会回退生产证据。

真实 ARM64 测试首先发现并修复：模型 Container 禁止 Broker socket；WeCom 1.4.2 分发只带 x64。1.4.3 追加同一固定上游的 ARM64 npm artifact，复核已审核两个原生二进制 SHA-256，保留全部 90 项能力和授权方式，升级不可变资源版本而不是改写旧 Revision。SQL 初次集成发现 seed 的 text ID 与 Revision UUID 比较不匹配，已显式转换并保留回归。

## 已执行证据（进行中）

- 聚焦 local process/profile/conformance 测试先因缺少 Profile 支持而红，后通过；生产拒绝 runc/未知 profile、root、host 网络的边界继续覆盖。
- 第一次真实 Docker 复现 WeCom 架构错误和 socket 限制；修复后 ARM64 三个包启动均通过，真实模型客户端→Broker→独立 Fixture 进程通过，测试 canary 已脱敏。该 Fixture 不访问第三方账号，不证明外部业务 API。
- WeCom 包 Parse：版本 1.4.3，包 SHA-256 `34bc101e21dabd46592f9dd48aad705812dee4f44fe0173fe2d5572cf0711be7`，bundle SHA-256 `16747bdbc47d4923ec4cfd92f3b5e54eff1ce5f9ed495693c19ca95364488ea1`；五项 Python 检查通过。新 bundle 的生产 runsc 证据尚无。
- `make test`、`make build`、`make deploy-test`、`go -C backend vet ./...` 通过；受影响包最后增补的取消清理测试也通过。独立 PostgreSQL 上完整 `gormrepo` 与 `gormdb` 套件实际运行通过（约 99 秒），未把默认无 DSN 的 Skip 当作数据库通过。随后增补的 Migration 000088 重跑与标记回填、seed 升级保留标记、没有 Runtime 时禁止回退的聚焦测试通过。目录 Parser 和不可变资源版本测试通过。

## 迁移与恢复边界

Migration 000088 是追加变更，不改已有 migration；新表无账号或凭据，主键为 bundle × Image ID；新增标记默认 false，只回填首次默认目录。旧代码忽略新表/列，应用回退不删 schema；包旧版本与历史记录仍保留。实际切换前备份业务数据库、Keycloak 数据库及加密状态，备份仅放私密数据卷且不提交；数据恢复会回退到备份时点，不能冒称零数据损失。恢复演练和切换结果待补。

## 待完成

代码首轮 `e5214bc` 已推送本会话分支并快进集成 `main_temp`。GitHub 最初 TLS/分块推送失败，使用本次命令级 HTTP/1.1 与 50 MiB postBuffer 后成功，未修改全局 Git 配置或历史。候选 ARM 控制镜像构建中，尚未切换服务。业务库、身份库与加密状态备份位于私密卷 `/data/backups/local-cli-before-e5214bc`（0600）；业务库恢复出 1 User、20 Publications、1 Model Connection，身份库恢复出 2 Realms，恢复库已删除，原库未改。

额外 AMD64 仿真检查的三个启动用例在 30 秒内超时，不能记为通过或等同原生 AMD64 验收。检查 Docker 元数据发现 `latest` 的本地 Image ID 是多架构索引；补充 Migration 000089，以 Worker 架构隔离本地证据，旧无架构行失效。此项不修改已提交 Migration 000088，也不改变生产证据。

构建/解包结束后的同用例 AMD64 仿真重跑通过（69.58 秒：DingTalk 13.03、Feishu 9.09、WeCom 23.19、模型客户端/Broker/独立进程 6.83 秒）。首次超时及五分钟测试总超时保留，不归类为原生 AMD64 通过。为使索引选择与证据架构一致，本地四条 Docker 创建路径显式绑定 Worker 平台且禁止任务时拉取；生产路径和指纹不变。此增补需重新跑实际 Docker Fixture，并以最终提交构建，不安装中间候选镜像。

显式平台参数的 ARM64 重验通过，27.67 秒，三包启动和模型客户端/Broker/独立命令及脱敏均通过；AMD64 同补丁重验进行中。完整 Go test/build/vet 和部署脚本门禁再次通过。实际构建发现 API 小改会重复执行约 110 秒的固定 MinIO 编译及 Keycloak augmentation，Dockerfile 将固定 MinIO 安装移至 backend COPY 前，并将 API/Worker 二进制 COPY 移到最终资源层之后；运行产物、版本、许可和部署结构不变，部署门禁已通过，最终镜像构建仍须实际检查。

显式平台参数的 AMD64 仿真重验通过，55.52 秒（三包 8.36/11.12/10.72 秒，完整往返 15.50 秒），不等于原生硬件证据。最终候选真实构建捕获 MinIO `/out` 目录原先依赖 API build 隐式建立的问题；已加入显式初始化与 Dockerfile 层次回归，未切换旧服务，没有修改实际用户数据。中间候选不发布，最终 SOURCE_REVISION 使用完整 Git SHA。

生产兼容复核发现 API-only 配置原本不要求 Worker runtime；API 现在只在显式 local profile 时装配本地验证，避免无故要求生产 API 填写 Worker 字段。新增构造 seam 回归验证省略 profile/production 不启用本地表、local/runc 只使用当前实际目标，Worker 严格配置校验不变。

最终门禁、真实部署和 UI 验收、第三方账号授权与只读业务调用、Git 推送及本地镜像切换。Linux + gVisor 和真实供应商完整矩阵缺环境时保持未验证。仅本地修复不更新生产配置，也不复制生产凭据或验证记录。
