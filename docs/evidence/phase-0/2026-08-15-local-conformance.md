# Phase 0 本地 Conformance 证据

日期：2026-08-15  
环境：macOS 26.5.2 arm64，Docker Desktop 29.2.1  
结论：本地可执行门禁通过；Production Conformance 未通过，不能替代 Linux + gVisor 证据。

## Runtime 镜像

四个独立镜像均从按 manifest digest 固定的 Docker Official Image 基础镜像构建，CLI 版本固定，最终用户为 `65532:65532`。本地 Image ID 仅用于复现本次构建；发布必须使用 Registry RepoDigest。

| Runtime | CLI 版本 | 本地 Image ID | `--version` | 非 root |
|---|---:|---|---|---|
| Claude Code | 2.1.233 | `sha256:b3821234ad782b5775a7b9d8c9b16dd01288cdb1f2c4e781923a933cbf2b3746` | 通过 | 通过 |
| Codex CLI | 0.147.0 | `sha256:6be975d97c7404b484cec0e61d52c25e97e8d3ed66cc2a3808eff6c304614c14` | 通过 | 通过 |
| Hermes Agent | 0.19.0 | `sha256:8893bc149c0e9f0a85fed9e919269c29347540a33c1f8b4c560f7c2c71d553cb` | 通过 | 通过 |
| OpenClaw | 2026.7.1-2 | `sha256:b20c9b29b125a2e74c69d45e4f357026aa464abe2b33fe428f439ecc0defc3b4` | 通过 | 通过 |

执行命令：

```bash
RUNTIME_BUILD_METADATA_DIR=build/runtime-images scripts/build-runtime-images.sh
scripts/conformance/runtime-image-smoke.sh
```

四个镜像都只验证了构建、实际 CLI 版本和非 root 身份。没有模型凭证、Registry 和真实代码任务，因此不声明 Production Runtime。

Codex 在 `/tmp` 下使用可写 `CODEX_HOME` 时会警告不创建 PATH helper alias；实际 `codex --version` 成功。该警告不影响本地版本门禁，但必须在 Linux Sandbox 的真实任务中确认不会影响执行，再决定是否为 Runtime State 增加独立 tmpfs。

## Object Storage

使用隔离的临时 MinIO Container 执行：

```bash
scripts/conformance/minio-local.sh
```

通过场景：

- 写入、读取、Stat、Content-Type、自定义元数据和 SHA-256；
- 12 MiB 大对象分段上传；
- 校验失败和已取消上传不留下对象；
- Not Found、删除和按 Prefix 生命周期清理；
- 签名 URL 在有效期内可下载正确内容；
- 去掉签名后不能访问私有对象；
- 签名 URL 到期后不能继续访问。

MinIO 测试镜像固定为 `minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e`。阿里云 OSS 凭证和测试 Bucket 未提供，因此 OSS 的真实服务行为未验证。

## 单元与契约测试

Go 测试覆盖 Runtime 契约、四个 Driver、Process Harness、取消/强杀、Secret Redaction、Sandbox 参数与漂移拒绝、Object Storage Provider 和 Factory。Vue/TypeScript 仅验证当前脚手架的类型和构建。

最终回归命令：

```bash
go test -race ./...
go vet ./...
pnpm web:typecheck
pnpm web:build
```

## 未满足的 Production 门禁

本机没有 `runsc`，且不是 Linux Worker；没有已配置模型凭证、临时 Git SSH 仓库、镜像 Registry 和阿里云 OSS 测试配置。以下证据仍必须在 Production Conformance 环境产生：

1. 四个 Runtime 的真实代码修改、成功/失败测试、Diff、Interrupt、Cancel、超时、强杀重建和 Review Branch Push；
2. gVisor 身份、只读根文件系统、资源限制、Reconcile、宿主/元数据/私网拒绝、DNS rebinding、重定向和代理绕过；
3. 模型 Key 与 SSH Key 在日志、事件、Diff、Artifact、Memory 和 Snapshot 中均不泄漏；
4. 阿里云 OSS 的私有 Bucket、分段上传、签名 URL 到期和生命周期行为；
5. 四个 Runtime 镜像的 Registry RepoDigest、场景耗时、模型 Usage 和失败 Artifact 链接。
