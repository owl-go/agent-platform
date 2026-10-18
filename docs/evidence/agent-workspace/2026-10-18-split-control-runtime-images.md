# 产品控制镜像与 Runtime 镜像拆分发布 — 2026-10-18

## 范围与发布物

用户要求 `haozhi152/agent-platform` 只承载前端入口和后端控制服务，五个 Runtime 由独立的 `haozhi152/agent-platform-runtime` 承载，并以 Docker Compose 作为安装入口。实现提交为 `477b258fb0aaec5eaf35529637758d6068040cea`，已快进到 `origin/main_temp`。

原 `haozhi152/agent-platform` 仓库已按用户要求删除并重新创建，避免继续保留职责错误的整合镜像。先前误建的 `haozhi152/agent-platform-runtimeatest` 也已删除；Runtime 仓库中三个指向旧整合镜像的 `2026-10-18-1eaca52efd84*` 标签在新版本发布后删除。两个正式仓库的 Docker Hub Overview 已分别更新为产品控制服务和五 Runtime 镜像的功能、Compose 使用方式与注意事项。

本次发布标签为 `2026-10-18-477b258fb0aa`：

| 仓库 | 多架构 Digest | ARM64 Digest | AMD64 Digest |
|---|---|---|---|
| `haozhi152/agent-platform` | `sha256:95e7f07894efe51f1edf09641bd24c55546a4033872ded8149b66c0ce2a00a8a` | `sha256:18a3586ea134dfaef82e7c9ac9a4927e2cf952e9deb0b1fe14f0a1a598ac5e13` | `sha256:73af011c2637692e154e9c534ccca16e33079bf4566df9459f8a78a94c72b3cc` |
| `haozhi152/agent-platform-runtime` | `sha256:e87886e26da0f098c874f4ca1349b6a4f92ed484b5a7d87e7a6fa69c458ba3f8` | `sha256:79f313b38a13ea8ea67f1dcba5cc192306b0a54e7090c03ea6cd4fac1aa12606` | `sha256:a3f1a38d8af2b34418b29c4b95aaa0f384b878d4f840448cace4866a4052a63c` |

两个仓库的 `latest` 和版本标签均为 Linux ARM64/AMD64 清单；`arm64`、`amd64` 别名也已更新。产品镜像实测不存在 `codex`、`claude`、`hermes`、`openclaw` 或 `pi` 命令。Runtime 镜像固定 Claude Code `2.1.233`、Codex `0.147.0`、Hermes `0.19.0`、OpenClaw `2026.7.1-2`、PI Agent `0.84.4`。

## 实际验证

以下检查通过：

```bash
python3 -m unittest discover -s deploy/all-in-one -p '*_test.py'
python3 -m unittest discover -s scripts -p 'unified_publish_test.py'
make deploy-test
make test
make build
make web-typecheck
make web-build
make image-build
RUNTIME_IMAGE_REF=haozhi152/agent-platform-runtime:local make runtime-image-smoke
python3 scripts/smoke-unified-image.py haozhi152/agent-platform:local --runtime-image haozhi152/agent-platform-runtime:local
RUNTIME_IMAGE=haozhi152/agent-platform-runtime:local IMAGE_TAG=local docker compose -f deploy/all-in-one/compose.yaml run --rm --no-deps runtime-image
docker buildx imagetools inspect haozhi152/agent-platform:latest
docker buildx imagetools inspect haozhi152/agent-platform-runtime:latest
docker pull --platform linux/arm64 haozhi152/agent-platform:latest
docker pull --platform linux/arm64 haozhi152/agent-platform-runtime:latest
RUNTIME_IMAGE_REF=haozhi152/agent-platform-runtime:latest make runtime-image-smoke
python3 scripts/smoke-unified-image.py haozhi152/agent-platform:latest --runtime-image haozhi152/agent-platform-runtime:latest
docker compose -f deploy/all-in-one/compose.yaml config --quiet
git diff --check
cmp -s AGENTS.md CLAUDE.md
```

完整发布后 smoke 在 macOS Apple Silicon + Docker Desktop Linux/ARM64 上通过：控制容器的 Web、API、Worker、Keycloak、PostgreSQL、MinIO 全部健康，匿名受保护 API 被拒绝，保留数据重启通过；隔离任务使用独立 Runtime Image ID，以 UID 65532、只读 Rootfs、无网络和最小挂载运行。AMD64 镜像完成构建期二进制与依赖检查并进入多架构清单，但没有在独立 AMD64 主机上做完整启动 smoke。

`make test` 和 `make build` 初次被已有的 `skills/.DS_Store`、`experts/.DS_Store` 资源目录文件拒绝；门禁期间临时移走并在结束后原样恢复，两项随后通过。发布第一次因 Debian CDN 超时中止，第二次因 Docker Registry EOF 中止；别名保护生效，`latest` 未暴露半完成版本。网络恢复后复用相同提交和不可变标签完成其余构建，再统一更新两个仓库的别名。

## 尚未验证的范围

- 没有 Linux + `runsc` Worker，本次未运行 Sandbox / Production Conformance，不据此宣称生产隔离已通过。
- 未配置 `WORKSPACE_TEST_POSTGRES_DSN`；Go 门禁中的 PostgreSQL 集成 Skip 不算通过。整合镜像内 PostgreSQL 的启动和保留数据重启属于本次 smoke 的已验证子集。
- 未执行 Aliyun OSS、完整远端 MinIO Conformance、真实模型调用或五个 Runtime 的生产 Capability Conformance。
- Docker Hub `latest` 是可移动标签；生产仍应分别固定上述产品和 Runtime RepoDigest，并为 Runtime Digest 补齐目标环境 Conformance。
