# PDF 处理参考

> **保存方式**：以下是参数示例，`-s` 单独使用仅保存云端，`-o`/`-d` 仅指定本地输出。实际任务按 `SKILL.md` 的保存策略选择；未指定方式的最终产物应双保存，且使用上下文生成 `--save-title`。

> **能力边界**：当前没有 PDF 文件合并命令；多个 PDF 只能在用户要求各自处理时逐个执行。逐页转图片是渲染，不保留原 PDF 的文本层和结构；不把它与图片合成组合成 PDF 合并承诺。

## pdf convert — PDF 格式转换

将 PDF 文档转换为其他可编辑格式。

| 目标格式 | `--format` 值 | 输出扩展名 | 说明 |
|----------|--------------|-----------|------|
| Word | `word` | .docx | 保留排版（默认） |
| Excel | `excel` | .xlsx | 适合表格类 PDF |
| Markdown | `md` | .md | 纯文本带格式 |
| TXT | `txt` | .txt | 纯文本（不支持 `-s`） |

```bash
camscanner-cli pdf convert report.pdf --format word -s
camscanner-cli pdf convert invoice.pdf --format excel -s
camscanner-cli pdf convert paper.pdf --format md -s
camscanner-cli pdf convert doc.pdf --format txt -o plain.txt
```

## pdf to-images — PDF 逐页转图片

将 PDF 每一页渲染为 JPEG 图片。

```bash
# 散页输出到目录
camscanner-cli pdf to-images report.pdf -d ./pages
# 产物：pages/page_1.jpg, pages/page_2.jpg, ...

# 仅云端：保存为多页图片文档（用户明确要求云端时）
camscanner-cli pdf to-images report.pdf -s --save-title "报告页面"

# 未指定保存方式：本地目录和云端双保存，不使用 -o
camscanner-cli pdf to-images report.pdf -d ./report_pages -s --save-title "报告页面"
```

| 参数 | 说明 |
|------|------|
| `-d, --dir` | 输出目录（默认 `<文件名>_pages/`） |
| `-s` | 仅云端；与 `-d dir` 同用时双保存 |

## pdf to-images-zip — PDF 转图片 ZIP

与 `to-images` 相同功能，但由服务端打包为单个 ZIP 文件。

```bash
camscanner-cli pdf to-images-zip report.pdf -o report_images.zip
```

> 注意：`to-images-zip` 不支持 `-s`（ZIP 不在云文档支持类型中）。

## pdf watermark — 添加水印

| 参数 | 说明 |
|------|------|
| `--text` | 水印文字（**必填**） |
| `--color` | 颜色，如 `#FF0000` |
| `--opacity` | 透明度 0-1 |
| `--size` | 字体大小 |

```bash
camscanner-cli pdf watermark contract.pdf --text "内部资料" -s
camscanner-cli pdf watermark doc.pdf --text "DRAFT" --color "#999999" --opacity 0.2 -s
```

## pdf remove-watermark — 去除水印

去除 PDF 中已有的水印。

```bash
camscanner-cli pdf remove-watermark document.pdf -s
camscanner-cli pdf remove-watermark doc.pdf -o clean.pdf
```

## 限制与注意事项

- **文件大小**：上传限制 40MB
- **页数限制**：水印操作最大 100 页
- **PDF 类型**：支持文字型和扫描型 PDF
- **加密 PDF**：不支持有密码保护的 PDF
