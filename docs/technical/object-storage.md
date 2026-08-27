# Object Storage

状态：MinIO 与阿里云 OSS 双 Provider

Artifact 与 Skill 包只通过 `internal/objectstore.Provider` 访问对象存储。业务表仅保存经过路径校验的逻辑 Object Key，不保存 Provider URL、Endpoint 或签名参数。

生产支持 `minio` 与 `aliyun_oss`，`memory` 仅用于单元测试。Provider 选择集中在 `providerfactory`。写入必须校验精确 Size 与小写 SHA-256，Bucket 保持私有；下载前重新执行 User/Workflow 授权，并签发短期 URL。

Skill 安装会把 Git 精确 Commit 或 ZIP 内容规范化，验证根目录存在 `SKILL.md`，再保存不可变对象和 SHA-256。Run Snapshot 冻结 Skill 的 Object Key 与 Digest，后续更新不改变已排队 Run。

Provider 行为变化先进入共享 Conformance，再分别验证 MinIO 与阿里云 OSS。缺少远端凭据导致的 Skip 不能记作通过。
