# Object Storage

状态：MinIO 与阿里云 OSS 双 Provider；CLI Connector bundle 的不可变写入与下载后二次校验已实现；AI Creation 图片对象生命周期已设计但尚未实现；生产 Digest 证据待补

Artifact、Skill 包、CLI Connector bundle 与消息附件只通过 `internal/objectstore.Provider` 访问对象存储。Workflow Run 与 Session response 生成的 Artifact 都保存为不可变对象；业务表仅保存经过路径校验的逻辑 Object Key，不保存 Provider URL、Endpoint 或签名参数。

生产支持 `minio` 与 `aliyun_oss`，`memory` 仅用于单元测试。Provider 选择集中在 `providerfactory`。写入必须校验精确 Size 与小写 SHA-256，Bucket 保持私有；Artifact 下载由登录态 API 重新执行 User 与 Session/Workflow 授权并代理对象字节，浏览器不接收对象存储 Endpoint 或签名参数。

消息附件使用 `attachments/<owner-user-id>/<attachment-id>` 逻辑 Key。单文件上限为 100 MiB，每个消息或 Workflow Run Turn 最多冻结十个附件。API 不接受客户端提供 Object Key；发送消息时按当前 User 重新解析附件，并把名称、类型、Size、Digest 和 Object Key 冻结到该 Turn。Worker 下载后再次校验 Size 与 Digest，只在本次执行的 Scratch 中生成只读副本；Sandbox Runner 将附件目录只读挂载到 `/workspace/.agent-platform-attachments`，使 Runtime 文件访问边界可以读取它，但该保留路径不会进入 Workflow Workspace 或 Artifact。图片预览与普通文件下载均由登录态 API 在重新校验当前 User 后代理对象字节，浏览器不接收对象存储 Endpoint 或签名参数。

Smart Assistant 图标使用 `ai-applications/assistant-icons/<owner-user-id>/<icon-id>` 逻辑 Key。上传接口只接受最多 2 MiB、解码后不超过 4096×4096 且总像素不超过 16777216 的 PNG、JPEG、WebP 或 GIF；服务端根据真实图片字节确定格式并校验 Size 与小写 SHA-256。Assistant 配置只保存逻辑 Object Key，预览由登录态 API 在重新校验 Assistant owner 后代理，浏览器不接收对象存储地址。上传对象与 Assistant 乐观锁更新作为一个补偿式操作：配置更新失败时删除刚写入的对象。

AI Creation 使用 `ai-creation/temp/<owner-user-id>/<upload-id>`、`ai-creation/records/<owner-user-id>/<record-id>/references/<position>` 和 `ai-creation/records/<owner-user-id>/<record-id>/outputs/<position>` 逻辑 Key。Reference Image 每张最多 20 MiB 和 6400 万解码像素，Generated Image 每张最多 25 MiB 和 6400 万解码像素；写入前按真实图片字节验证格式、Size、像素与小写 SHA-256，供应商返回的 URL、Base64 和原始响应不持久化。未绑定上传在页面离开时请求删除，并由 24 小时生命周期兜底；绑定的输入和输出字节在记录创建 90 天后过期。预览、单张下载与临时 ZIP 下载均由登录态 API 重新验证 owning User 后代理，Administrator 权限不绕过内容所有权。

Skill 安装会把 Git 精确 Commit 或 ZIP 内容规范化，验证根目录存在 `SKILL.md`，再保存不可变对象和 SHA-256。Run Snapshot 冻结 Skill 的 Object Key 与 Digest，后续更新不改变已排队 Run。

ZIP 上传支持根目录直接包含 `SKILL.md`，或由一个顶层 Skill 目录包裹全部有效文件。上传时移除这层目录及 macOS 元数据（`__MACOSX`、`.DS_Store`、`._*`），按路径排序重建 ZIP，确保 Runtime 读取的包根目录包含普通文件 `SKILL.md`。多个候选 Skill 目录、Skill 目录之外的文件、路径穿越、重复或冲突路径、符号链接和特殊文件均拒绝；读取内容时校验 ZIP 完整性，压缩包、解压文件总量和规范化后 ZIP 均不得超过 50 MiB。Size 与 SHA-256 根据规范化后的实际对象字节计算。Skill 和 CLI Connector 的 ZIP 新建与更新通过 Base64 JSON 传输；HTTP 层为这四个上传路由保留 50 MiB 压缩包编码后的空间及 64 KiB JSON 元数据空间，其余 JSON 路由仍限制为 64 KiB。大小校验按实际读取字节执行，不依赖 Content-Length；超限返回 HTTP 413 `request_body_too_large`，格式错误保持 HTTP 400 `invalid_request_body`。

CLI Connector 的 ZIP 源包在 API 边界完成路径、类型、大小、`package.json` 和 `bin` 校验后，以逻辑 Object Key 和小写 SHA-256 不可变保存。CLI Connector bundle 由隔离且无 User 凭证的 Builder 从 Administrator 指定的 exact npm package 或已校验 ZIP 源包生成，保存不可变 Object Key、包完整性与最终小写 SHA-256。Run Snapshot 只冻结 Definition、bundle Digest、能力策略和 Authorization identity；App Secret 与 Token 不进入 Snapshot。Worker 下载后重新校验 Digest，并只读挂载 bundle，永不向 Runtime 暴露对象存储 URL 或签名参数。

Provider 行为变化先进入共享 Conformance，再分别验证 MinIO 与阿里云 OSS。缺少远端凭据导致的 Skip 不能记作通过。
