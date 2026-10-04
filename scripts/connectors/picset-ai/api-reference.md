# API 参数说明

依据 https://picsetai.cn/developer-api，核对日期 2026-10-01。`schema` 给出各命令的字段、必填项和素材 scene；服务端负责最新枚举和模型组合验证。REST 基础路径固定为 `https://picsetai.cn/functions/v1/agent-api-v1/`。

| 命令 | 业务 | 素材 |
|---|---|---|
| upload-url | 分配上传地址 | scene |
| image-audit | 审核已上传图 | scene、oss_path |
| product-main / product-detail | 商品主图 / 详情图 | product_images，1–6 张同商品 |
| ad-image | 商品广告图 | product_images，1–6 张 |
| style-replicate | 风格复刻 | reference_image，product_images 1–6 张 |
| sku-replace | SKU 替换 | reference_image、product_image |
| image-refinement / image-translation | 精修 / 翻译 | images，1–6 张 |
| canvas-image | 图片生成 | prompt；可选 reference_images 1–9 张 |
| image-layer-2-0-auto | 自动分层 | image |
| image-layer-2-0-natural-language | 文字指定分层 | image、prompt |
| image-layer-2-0-region | 区域分层 | image、rectangles |
| image-layer-1-0 | 经典分层 | image |
| request | GET 查询 | request_id UUID |

图片对象只含 `oss_path`，不得传 URL、MIME 或文件名。上传限制 20 MiB；通用审核支持 JPEG/PNG/WebP，分层 2.0 只接受真实 JPEG/PNG。分层图片均使用 image_layer scene，其他 scene 见 schema。预签名 URL 不修改、不转发给无关服务。

一般文本最长 4000 字；SKU 合并文本最长 12000 字。主图/详情 output_count 为 1–16；广告图还接受 20、25、30、35、40、45、50。其他接口不支持 output_count。详情图 amazon_aplus 使用 21:9；主图不支持 amazon_aplus。翻译 target_language 默认 en，不能为 none。

模型公开名为 nova-2.0、nova-pro、nova-2.0-lite、nova-img-2、nova-img-2-vip。nova-img-2 不传 resolution；gpt_quality 省略表示自动，可显式 low/medium/high，不传 auto。VIP 只使用 fast；canvas-image VIP 不传 gpt_quality，服务端固定 medium。Lite 只支持 1K。其他质量、比例与速度组合按官方对应接口的兼容表选择；同一模型在不同业务可用比例不同。

自然语言分层 prompt 去首尾空白后 1–1000 字；区域分层 prompt 最长 1000 字。rectangles 是 1–20 个 [x1,y1,x2,y2]，整数 0–999，x1<x2 且 y1<y2；坐标相对于原图。分层 2.0 的可选 speed_mode 为 normal/fast，resolution 为 auto/1K/1.5K/2K。自动 2.0 和分层 1.0 只接受 image。

结果分层只提供结构化图层，未提供 PSD 或 ZIP。查询可能增量返回图片及部分失败；image_url 是结果数据，此 CLI 不主动下载。
