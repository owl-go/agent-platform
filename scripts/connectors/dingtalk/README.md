# 钉钉 CLI Connector Package 构建与上架

本工具把固定钉钉 `dingtalk-workspace-cli@1.0.62` 源码构建的 Linux amd64/arm64 二进制及同版 MultiSkill 打入版本为 `1.0.63` 的 CLI Connector Package。它从同一版 CLI 的 `schema --all` 生成命令策略：保留 1,409 个 `available` 工具，读操作为低风险，其余操作一律经过平台单次批准，另开放 `version` 和 `schema` 两个只读诊断命令。托管连接器不开放 `profile list` 和 `auth status`，因为它们只检查 CLI 本地 Profile，无法反映平台注入的短期令牌，会把有效平台授权误报为未登录。`--include-admin` 包含开发者应用、PAT、审计、事件等管理命令；省略时只保留业务产品。

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
scripts/connectors/dingtalk/build-patched-cli.sh <pinned-upstream-checkout> /tmp/dingtalk-patched
python3 scripts/connectors/dingtalk/build.py \
  --npm-tgz /tmp/dingtalk-workspace-cli-1.0.62.tgz \
  --schema /tmp/dingtalk-schema.json \
  --runtime-image <runtime-image@sha256:digest> \
  --runtime-version <exact-node-version> \
  --include-admin \
  --amd64-binary /tmp/dingtalk-patched/dws-linux-amd64 \
  --arm64-binary /tmp/dingtalk-patched/dws-linux-arm64 \
  --output /tmp/dingtalk-1.0.63.zip
go -C backend run ./cmd/connector-package-validate /tmp/dingtalk-1.0.63.zip
```

先从钉钉仓库固定提交 `70323e1486e64b1ca823fa2bd9c48b9b07f88519` 执行 `build-patched-cli.sh <upstream-checkout> /tmp/dingtalk-patched`。该构建只增加受审查的环境变量令牌入口，不把 Secret 写入参数。平台的钉钉授权适配器执行设备流并校验组织 CLI 可用状态；换票结果缺少用户 ID 时用新令牌查询当前用户身份。刷新凭证保存在平台加密存储中，单次 CLI 进程只接收短期令牌。

包解析成功不等于可以上架。必须在目标 Linux + `runsc`，对确切 `cli-bundle.tgz` SHA-256 和 Registry Runtime RepoDigest 执行 Conformance，并由平台记录通过证据，才能调用 Administrator Stage 与 Publish。缺少证据时 Stage 会拒绝。上架后 User 还须安装并通过钉钉设备授权；组织管理员未开放 CLI 访问时授权会失败。未使用真实账号验证的产品 API 不应声称已经通过生产调用。

上游 Schema 不提供本项目可直接重验的细粒度 OAuth scopes，当前 manifest 不填写虚构 scopes。Egress 域名取自本版 CLI 中的钉钉端点静态清单，发布前还需要真实账号验证所选产品的网络访问。Schema 标为 `unavailable` 的 26 个工具、未纳入 Schema 的原始 API 等命令不会放行，因为当前平台无法给它们建立可审查的逐命令策略。用户对钉钉组织的 CLI 访问也须经组织管理员批准。
