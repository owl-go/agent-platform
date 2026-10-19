# 企业微信 Connector Package

## 1.4.3 双架构分发

`package/` 的 1.4.3 修订在同一 bundle 内携带经 `build.py` 固定 SHA-256 校验的 Linux ARM64 与 x64 原生二进制，修复旧 1.4.2 分发包只有 x64 的问题。CLI、90 项能力策略、授权方式与上游参考版本不变。`dual_arch.py` 只组包，不运行 npm 生命周期脚本或上游代码；ARM64 tarball 固定为官方 npm 的 `@wecom/cli-linux-arm64@1.3.4` 并验证 SHA-512。已存在相同 ARM64 内容时幂等，任何不同内容或未审核摘要拒绝。

```bash
curl --fail --output /tmp/wecom-arm64-1.3.4.tgz https://registry.npmjs.org/@wecom/cli-linux-arm64/-/cli-linux-arm64-1.3.4.tgz
python3 -m connectors.wecom.dual_arch --arm64-package /tmp/wecom-arm64-1.3.4.tgz --output outputs/wecom/wecom-1.4.3-dual.zip
go -C backend run ./cmd/connector-package-validate ../outputs/wecom/wecom-1.4.3-dual.zip
```

下面保留原始单架构构建路径；新的正式分发应再通过上述步骤补齐 ARM64 后验证。当前修订在生产 runsc 上的旧 bundle 证据不适用于新 bundle，必须重新取得 exact 组合证据后发布。显式本地 runc 按 [ADR-0049](../../docs/adr/0049-local-cli-connector-verification.md)记录独立兼容性证据；两者均不等于实际账号业务验证。

本目录构建 CLI 模式的企业微信 Connector Package。上游固定为 [`@wecom/cli@1.3.4`](https://github.com/WecomTeam/wecom-cli)，其二进制报告源提交 `f9b2815`；npm tarball 的 SHA-512 与两个 Linux 原生二进制的 SHA-256 固定在 `build.py`。企业微信[帮助页](https://open.work.weixin.qq.com/help2/pc/21676)说明 CLI 与 MCP 两种接入方式；本包选择 CLI，以便使用平台的逐命令能力策略和写操作一次性批准。

构建使用仓库的隔离 CLI Builder 生成 `bundle.tgz`、`package.tgz` 与 `integrity.txt`。将同一 Builder 输出目录传给 `build.py`，同时传入目标 Linux 架构、目标 Runtime 的**真实** RepoDigest 和 Node 版本。脚本核对 npm tarball、固定版本与原生可执行文件，再用 [`capabilities.json`](capabilities.json) 生成 90 项逐命令策略，并把同一目录交给适配入口校验。Connector Package 修订版本为 1.4.2，固定 CLI 仍是 1.3.4。`upstream-v1.3.4.tar.gz` 固定源提交 `f9b28151dc8d42f703db624fc9b26723c3c93fc5`，包含上游全部 15 个 Skills、相邻资源、CLI 命令参考和 MIT License；构建时校验其 SHA-256，作为根 Skill 的参考文件进入 ZIP。生成的 ZIP 位于 `outputs/`，不提交到 Git。CLI 容器的进程上限为 128；此前的 8 会使生产 `runsc` 在启动沙箱进程时失败。

```bash
python3 connectors/wecom/build.py \
  --builder-output /path/to/cli-builder-output \
  --runtime-digest sha256:<目标 Runtime RepoDigest> \
  --runtime-version 24.15.0 \
  --arch arm64 \
  --output outputs/wecom-1.4.2-linux-arm64.zip
go -C backend run ./cmd/connector-package-validate ../outputs/wecom-1.4.2-linux-arm64.zip
```

平台的提供凭证授权接收 `{"bot_id":"...","secret":"..."}`。这两个值来自 User 自己已获企业微信授权的 API 模式机器人。平台按 User 加密保存，单次命令注入；适配入口使用上游 `get_cli_config` 签名协议换取本次 Bearer token，再以 `WECOM_CLI_ACCESS_TOKEN` 运行固定 CLI。安装不代表已连接，连接不代表企业已审批相关业务权限。

包解析验证文件和策略格式。发布或调用还要求目标 Linux + `runsc` 环境对**同一 bundle SHA-256 × Runtime RepoDigest**通过 CLI Conformance。当前 `build.py` 不创建 Conformance 记录；需要在目标平台执行并核对 Publication、Installation 与 Authorization 状态。上游服务目录在线下发，真实业务能力还需在已授权的测试企业中逐项验证。文件路径仅可指向本次 `/workspace` 的真实文件；下载文件进入其 `.wecom-downloads` 子目录。
