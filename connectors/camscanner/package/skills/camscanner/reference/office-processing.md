# Office 文档处理参考

> **保存方式**：以下是参数示例，`-s` 单独使用仅保存云端，`-o` 仅指定本地输出。实际任务按 `SKILL.md` 的保存策略选择；未指定方式的最终产物应双保存，且使用上下文生成 `--save-title`。

## office convert — Office 文档格式转换

将 Word、Excel、PPT 文档转换为 PDF，或将旧格式（DOC/XLS/PPT）升级为新格式（DOCX/XLSX/PPTX）。CLI 根据文件扩展名自动识别源文件类型。

### 支持的转换

| 输入格式 | `--format` 值 | 输出扩展名 | 说明 |
|----------|--------------|-----------|------|
| DOC/DOCX | `pdf`（默认） | .pdf | Word 转 PDF |
| DOC | `docx` | .docx | 旧版 Word 格式升级 |
| XLS/XLSX | `pdf`（默认） | .pdf | Excel 转 PDF |
| XLS | `xlsx` | .xlsx | 旧版 Excel 格式升级 |
| PPT/PPTX | `pdf`（默认） | .pdf | PPT 转 PDF |
| PPT | `pptx` | .pptx | 旧版 PPT 格式升级 |

> 已经是新格式的文件（DOCX/XLSX/PPTX）不支持转换为同格式（如 DOCX → DOCX），仅支持转 PDF。

### 参数

| 参数/标志 | 说明 |
|----------|------|
| `file`（位置参数） | 输入文件路径（必填） |
| `--format` | 目标格式：`pdf`（默认）、`docx`、`xlsx`、`pptx` |
| `-o, --output` | 输出文件路径（不传则自动推导） |
| `-s, --save` | 保存到云文档 |
| `--save-title` | 云文档标题 |
| `--save-dir` | 保存到指定文件夹（按名称） |
| `--save-dir-id` | 保存到指定文件夹（按 ID） |

### 使用示例

```bash
# Word 转 PDF（默认）
camscanner-cli office convert report.docx -o "report.pdf" -s --save-title "报告PDF版"

# Excel 转 PDF
camscanner-cli office convert data.xlsx -o "data.pdf" -s --save-title "数据表PDF"

# PPT 转 PDF
camscanner-cli office convert slides.pptx -o "slides.pdf" -s --save-title "演示文稿PDF"

# 旧格式升级：DOC → DOCX
camscanner-cli office convert old.doc --format docx -o "old_upgraded.docx" -s

# 旧格式升级：XLS → XLSX
camscanner-cli office convert legacy.xls --format xlsx -o "legacy_upgraded.xlsx" -s

# 旧格式升级：PPT → PPTX
camscanner-cli office convert presentation.ppt --format pptx -o "presentation_upgraded.pptx" -s
```

## 限制与注意事项

- **文件大小**：上传限制 40MB
- **加密文档**：不支持有密码保护的 Office 文档
- **格式检测**：CLI 根据文件扩展名判断类型，确保扩展名正确
- **不支持的转换**：DOCX → DOCX、XLSX → XLSX、PPTX → PPTX（同格式无需转换）
