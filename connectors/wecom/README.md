# 企业微信 Connector Package

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
