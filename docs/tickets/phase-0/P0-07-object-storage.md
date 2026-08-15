# P0-07 实现阿里云 OSS 与 MinIO Provider

状态：实现完成，等待真实 MinIO 与阿里云 OSS Conformance

## 目标

让 Artifact 和 Workspace Snapshot 通过同一个 Object Storage Interface 使用阿里云 OSS 或 MinIO。

## 工作

- 定义对象写入、读取、元数据、校验、删除、生命周期和短期签名访问 Interface。
- 实现原生阿里云 OSS Adapter。
- 实现 MinIO S3-compatible Adapter。
- 提供内存 Fake，业务测试不依赖真实对象存储。

## 验收

- 两个 Adapter 通过同一套 Conformance Test。
- 覆盖分段上传、Checksum 失败、对象不存在、签名过期、删除和生命周期清理。
- 上层只保存逻辑 Object Key，不保存 Provider 专有 URL。
- Bucket 默认私有，下载必须经过平台授权。

## 依赖

无，可与 Runtime 工作并行。

## 当前证据

- `internal/objectstore.Provider` 固定逻辑 Key、Put/Get/Stat、SHA-256、Delete、Lifecycle 和 Presigned GET 语义。
- `memory.Provider` 已通过共享 Conformance，业务测试不需要云服务。
- MinIO Adapter 使用 `minio-go/v7` 的 S3-compatible API 与自动 multipart；阿里云 OSS Adapter 使用原生 SDK 与显式 multipart/abort。
- 两个 Adapter 均只在内部拼接部署 Prefix，并把 Not Found 映射为相同错误；业务 Metadata 采用统一编码，避免 HTTP Header 大小写差异。
- 两个真实 Provider 的 Integration Test 已调用同一套 Conformance；当前开发机 Docker daemon 未运行，且未配置阿里云 OSS/MinIO 测试凭据，因此真实服务结果尚未产生。
- Bucket 私有性、签名 URL 实际到期和云端生命周期行为留给 P0-08 的真实环境验收，完成前本 Ticket 不标记“已完成”。
