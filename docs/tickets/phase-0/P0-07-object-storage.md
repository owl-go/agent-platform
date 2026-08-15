# P0-07 实现阿里云 OSS 与 MinIO Provider

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

