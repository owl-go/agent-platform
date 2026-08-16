# Single-Worker Deployment Baseline

This Compose stack runs the current API, Worker, Web, PostgreSQL, and MinIO services on one Linux Worker. Only the Web port is published; PostgreSQL and MinIO remain on an internal Docker network.

The stack is a deployment baseline, not evidence that the MVP is complete. The current API and Worker remain Phase 0 skeletons until the later implementation phases are delivered and verified.

## Configuration

API 和 Worker 只读取 YAML 配置。默认 Compose 配置使用 `config/platform.minio.yaml`；切换阿里云 OSS 时，从 `config/platform.aliyun-oss.example.yaml` 创建部署专用文件，并通过 `PLATFORM_CONFIG_FILE` 指定它。`${NAME}` 占位符在进程启动时从环境展开，未知 YAML 字段、缺失环境变量和非法值会让服务 fail closed。

复制 `.env.example` 到仓库外，替换所有 Secret，并把可变基础设施 Tag 解析为 Repository Digest。Runtime 镜像单独管理，始终使用 Runtime Conformance 记录的 RepoDigest。

```bash
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml config
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

部署文件可以明确覆盖 YAML 路径：

```bash
PLATFORM_CONFIG_FILE=/opt/agent-platform/config/platform.yaml \
docker compose --env-file /opt/agent-platform/config/platform.env \
  -f deploy/platform/compose.yaml up -d --build
```

Verify the public entrypoint and proxied API health endpoint:

```bash
curl --fail http://127.0.0.1/
curl --fail http://127.0.0.1/api/healthz
curl --fail http://127.0.0.1/api/readyz
```
