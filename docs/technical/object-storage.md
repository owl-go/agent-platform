# Object Storage Provider 技术规格

状态：Phase 0 实施基线

## 目标与边界

Artifact、Runtime 原始输出和 Workspace Snapshot 只通过 `internal/objectstore.Provider` 访问对象存储。业务记录持久化逻辑 Object Key，不持久化阿里云 OSS URL、MinIO Endpoint 或签名参数。

首期 Provider：

- `aliyun_oss`：阿里云 OSS 原生 Go SDK；
- `minio`：MinIO S3-compatible Go SDK；
- `memory`：仅用于业务单元测试，不用于生产。

`providerfactory.New` 根据部署配置选择实现，调用方不写 Provider 分支。

## Interface

```go
type Provider interface {
    Put(ctx context.Context, key string, body io.Reader, options PutOptions) (Object, error)
    Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
    Stat(ctx context.Context, key string) (Object, error)
    Delete(ctx context.Context, key string) error
    PresignGet(ctx context.Context, key string, expiresIn time.Duration) (SignedURL, error)
    DeleteExpired(ctx context.Context, query LifecycleQuery) (int, error)
}
```

Object Key 必须是无前导斜杠、无 `.`/`..` 穿越的逻辑相对路径。部署可配置 Provider Prefix；Prefix 同样经过路径校验，并只在 Adapter 内部映射成物理 Key。

## 写入与校验

调用方必须提供确切 Size 和小写十六进制 SHA-256。共享 `PrepareUpload` 将 Reader 流式写入临时文件，同时校验 Context、Size 和 SHA-256；校验失败时不会调用远端 Put，因此不会留下部分可见对象。

临时文件让两个 SDK 获得可重读、已知长度的输入：

- MinIO 使用 `PutObject` 和配置的 PartSize，超过 PartSize 时由 SDK 执行 multipart；PartSize 不得小于 5 MiB。
- 阿里云 OSS 在超过 PartSize 时显式执行 Initiate、UploadPartFromFile 和 Complete；任一 Part 或 Complete 失败都会 Abort；PartSize 不得小于 100 KiB。

SHA-256 与业务 Metadata 存入保留的对象元数据头。业务 Metadata 先编码为 JSON/Base64，避免两个 Provider 对 Header 大小写的差异改变上层语义。

## 读取、删除与生命周期

`Get` 先执行 `Stat`，统一把 Provider 的 404 映射为 `ErrNotFound`，然后返回流式 Reader。`Delete` 是幂等操作。

`DeleteExpired` 只枚举部署 Prefix 与业务 Prefix 的交集，并删除 `LastModified < Before` 的对象。MinIO SDK 负责分页迭代；OSS Adapter 显式使用 Marker 分页。生产定时任务应使用稳定的截止时间和独立前缀，失败后可安全重试。

## 私有下载

Bucket 由部署流程预先创建且必须保持私有，应用 Provider 不创建 Bucket、不修改 ACL，也不返回永久公开 URL。平台完成用户授权后，才调用 `PresignGet` 生成最长七天、通常数分钟的临时 GET URL。`SignedURL` 只用于即时响应，不能保存为 Artifact 字段。

P0-08 必须在真实服务上验证 Bucket ACL/Policy、未授权访问失败以及 URL 到期后失效。

## Conformance

共享套件位于 `internal/objectstore/conformance`，覆盖：

1. Put/Get/Stat 与 Metadata 一致性；
2. 12 MiB 大对象路径；
3. SHA-256 失败不落对象；
4. Context 取消不落对象；
5. Not Found、Delete 与 Lifecycle Prefix；
6. Presigned URL 的有效期契约。

内存 Provider 在普通 `go test ./...` 中执行完整套件。MinIO 与阿里云 OSS 的测试文件调用同一套件；缺少以下真实环境变量时明确 Skip：

- MinIO：`MINIO_ENDPOINT`、`MINIO_BUCKET`、`MINIO_ACCESS_KEY`、`MINIO_SECRET_KEY`，可选 `MINIO_SESSION_TOKEN`、`MINIO_SECURE`；
- 阿里云 OSS：`ALIYUN_OSS_ENDPOINT`、`ALIYUN_OSS_BUCKET`、`ALIYUN_OSS_ACCESS_KEY`、`ALIYUN_OSS_SECRET_KEY`，可选 `ALIYUN_OSS_SESSION_TOKEN`。

P0-08 记录两次真实套件的输出、Bucket 隐私证据和签名 URL 到期后的 HTTP 结果。
