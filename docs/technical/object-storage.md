# Object Storage

状态：MinIO 与阿里云 OSS 双 Provider；CLI Connector bundle 的不可变写入与下载后二次校验已实现，生产 Digest 证据待补

Artifact、Skill 包、CLI Connector bundle 与消息附件只通过 `internal/objectstore.Provider` 访问对象存储。业务表仅保存经过路径校验的逻辑 Object Key，不保存 Provider URL、Endpoint 或签名参数。

生产支持 `minio` 与 `aliyun_oss`，`memory` 仅用于单元测试。Provider 选择集中在 `providerfactory`。写入必须校验精确 Size 与小写 SHA-256，Bucket 保持私有；下载前重新执行 User/Workflow 授权，并签发短期 URL。

消息附件使用 `attachments/<owner-user-id>/<attachment-id>` 逻辑 Key。单文件上限为 100 MiB，每个消息或 Workflow Run Turn 最多冻结十个附件。API 不接受客户端提供 Object Key；发送消息时按当前 User 重新解析附件，并把名称、类型、Size、Digest 和 Object Key 冻结到该 Turn。Worker 下载后再次校验 Size 与 Digest，只在本次执行的 Scratch 中生成只读副本；Sandbox Runner 将附件目录只读挂载到 `/workspace/.agent-platform-attachments`，使 Runtime 文件访问边界可以读取它，但该保留路径不会进入 Workflow Workspace 或 Artifact。图片预览与普通文件下载均由登录态 API 在重新校验当前 User 后代理对象字节，浏览器不接收对象存储 Endpoint 或签名参数。

Skill 安装会把 Git 精确 Commit 或 ZIP 内容规范化，验证根目录存在 `SKILL.md`，再保存不可变对象和 SHA-256。Run Snapshot 冻结 Skill 的 Object Key 与 Digest，后续更新不改变已排队 Run。

CLI Connector bundle 由隔离且无 User 凭证的 Builder 从 Administrator 指定的 exact npm package、version 和 integrity 生成，保存不可变 Object Key、npm integrity 与最终小写 SHA-256。Run Snapshot 只冻结 Definition、bundle Digest、能力策略和 Authorization identity；App Secret 与 Token 不进入 Snapshot。Worker 下载后重新校验 Digest，并只读挂载 bundle，永不向 Runtime 暴露对象存储 URL 或签名参数。

Provider 行为变化先进入共享 Conformance，再分别验证 MinIO 与阿里云 OSS。缺少远端凭据导致的 Skip 不能记作通过。
