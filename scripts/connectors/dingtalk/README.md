# 钉钉 CLI Connector Package 构建草案

本工具把钉钉官方 `dingtalk-workspace-cli@1.0.62` 的 Linux amd64/arm64 二进制及同版 MultiSkill 打入项目的 CLI Connector Package。它从同一版 CLI 的 `schema --all` 生成命令策略：保留 1,409 个 `available` 工具，读操作为低风险，其余操作一律经过平台单次批准。另开放 `version`、`schema`、`profile list` 和 `auth status` 四个只读诊断命令。`--include-admin` 包含开发者应用、PAT、审计、事件等管理命令；省略时只保留业务产品。

上游来源固定为 [钉钉 Workspace CLI](https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli) 的 npm 1.0.62 发布包。构建时校验 npm SHA-512 和包内三个资产的 SHA-256，不执行 npm 安装脚本。`schema --all` 必须由同版二进制导出，构建器校验其 catalog hash 和工具数。产出的 ZIP 还必须经仓库的 `connectorpackage.Parse` 验证。

图标使用[钉钉开放平台页面](https://open.dingtalk.com/document/development/dingtalk-cli-performing-tasks-within)引用的 128px 官方 favicon，原文件 SHA-256 为 `fbffbab13dd378fa25cb0ccb7540c0d57a4b7328058beb677b344cea8662124d`。连接器包把该 PNG 内嵌进 `icon.svg`，产品卡片使用相同 PNG。

示例（使用已配置的 Registry Runtime RepoDigest，命令中的路径由操作者替换）：

```bash
npm pack dingtalk-workspace-cli@1.0.62 --pack-destination /tmp --silent
mkdir -p /tmp/dingtalk-linux
tar -xOzf /tmp/dingtalk-workspace-cli-1.0.62.tgz package/assets/dws-linux-amd64.tar.gz \
  | tar -xzf - -C /tmp/dingtalk-linux dws
docker run --rm --platform linux/amd64 --network none --read-only \
  --mount type=bind,src=/tmp/dingtalk-linux,dst=/review,readonly=true \
  --entrypoint /review/dws \
  <runtime-image@sha256:digest> schema --all --format json > /tmp/dingtalk-schema.json
python3 scripts/connectors/dingtalk/build.py \
  --npm-tgz /tmp/dingtalk-workspace-cli-1.0.62.tgz \
  --schema /tmp/dingtalk-schema.json \
  --runtime-image <runtime-image@sha256:digest> \
  --runtime-version <exact-node-version> \
  --include-admin \
  --output /tmp/dingtalk-1.0.62-draft.zip
go -C backend run ./cmd/connector-package-validate /tmp/dingtalk-1.0.62-draft.zip
```

这个 ZIP **尚不能发布或当作已连接使用**。目前平台的 `connector_package` driver 只向隔离 CLI Container 注入 `CONNECTOR_CREDENTIALS_JSON`；钉钉 DWS 使用自己的设备流登录和加密本地 profile，不能从该 JSON 恢复登录。现有 `BeginConnectorAuthorizationFlow` 与 `CompleteConnectorAuthorizationFlow` 也只调用飞书授权服务。必须新增经审查的钉钉授权/凭证物化适配，并在目标 Linux + `runsc`、确切 bundle SHA-256 和 Runtime Registry RepoDigest 上运行 Conformance，之后才可暂存、发布并供 User 授权安装。仅有本地镜像 Digest 或普通 Docker `runc` 冒烟测试都不等于这项证据。

上游 Schema 不提供本项目可直接重验的细粒度 OAuth scopes，当前 manifest 不填写虚构 scopes。Egress 域名取自本版 CLI 中的钉钉端点静态清单，发布前还需要真实账号验证所选产品的网络访问。Schema 标为 `unavailable` 的 26 个工具、未纳入 Schema 的原始 API 等命令不会放行，因为当前平台无法给它们建立可审查的逐命令策略。用户对钉钉组织的 CLI 访问也须经组织管理员批准。
