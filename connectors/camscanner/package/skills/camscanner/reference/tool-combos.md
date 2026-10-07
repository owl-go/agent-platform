# 工具组合速查

先按 `SKILL.md` 判断输入类型与最终产物是否支持。此处只组合已支持的任务；没有 PDF 文件合并或独立上传云端的命令，不通过 PDF 转图片再合成来承诺 PDF 合并。

下方通用处理示例默认双保存；用户明确选择本地或云端时按主文档调整。`--save-title` 根据实际上下文命名。中间产物只存本地，只有前一步成功且产物可用后才执行下一步。云目录和文档管理示例保留其明确的保存/管理意图。

## 基础组合

| 用户需求 | 推荐命令 | 说明 |
|----------|----------|------|
| 识别图片文字 | `image ocr photo.jpg` | 输出到终端 |
| 单图转文档 | `image convert photo.jpg --format word -o "photo_convert.docx" -s` | 本地和云端双保存 |
| 多图转文档 | `image merge-word "page1.jpg" "page2.jpg" "page3.jpg" -o "merged.docx" -s` | 多页合并，显式列出 |
| PDF 转可编辑 | `pdf convert doc.pdf --format word -o "doc_convert.docx" -s` | |
| 美化照片 | `image hd blurry.jpg -o "blurry_hd.jpg" -s` | |
| 保护文档 | `pdf watermark file.pdf --text "机密" -o "file_watermark.pdf" -s` | |
| Word 转 PDF | `office convert report.docx -o "report.pdf" -s` | |
| Excel 转 PDF | `office convert data.xlsx -o "data.pdf" -s` | |
| PPT 转 PDF | `office convert slides.pptx -o "slides.pdf" -s` | |
| 旧格式升级 | `office convert old.doc --format docx -o "old_upgraded.docx" -s` | DOC→DOCX、XLS→XLSX、PPT→PPTX |

## 多步组合

### 批量扫描图片生成 PDF

```bash
# 步骤1：多张扫描件合并为 PDF 并双保存
camscanner-cli image merge-pdf "scan_001.jpg" "scan_002.jpg" "scan_003.jpg" -o "merged.pdf" -s --save-title "扫描文档合并"
```

### 图片中表格数据提取为 Excel

```bash
# 步骤1：将多张包含表格的图片合并转 Excel
camscanner-cli image merge-excel "table_page1.jpg" "table_page2.jpg" -o "merged.xlsx" -s
```

### 文档加水印后保存

```bash
# 步骤1：给 PDF 添加水印
camscanner-cli pdf watermark contract.pdf --text "仅供内部使用" -o "contract_watermark.pdf" -s --save-title "合同-水印版"
```

### 多语言文档翻译工作流

```bash
# 步骤1：翻译图片中的文字（保留原始排版）
camscanner-cli image translate document.jpg --lang en -o "document_translate.jpg" -s --save-title "翻译-英文版"
```

### OCR 提取后转文档

```bash
# 方式1：直接转 Markdown（推荐，带格式）
camscanner-cli image convert document.jpg --format md -o "document_convert.md" -s

# 方式2：OCR 提取纯文本后保存
camscanner-cli image ocr document.jpg > extracted.txt
camscanner-cli txt to-word extracted.txt -o "extracted_to_word.docx" -s --save-title "OCR提取结果"
```

### PDF 拆分为独立图片

```bash
# 本地和云端双保存：逐页渲染为图片
camscanner-cli pdf to-images report.pdf -d ./pages -s --save-title "报告页面"

# 或拆分为 ZIP 包
camscanner-cli pdf to-images-zip report.pdf -o report_pages.zip
```

### 图片真伪鉴定

```bash
# 检测 PS 篡改
camscanner-cli image validate suspect.jpg --mode 1

# 检测 AI 生成
camscanner-cli image validate ai_photo.jpg --mode 2
```

### 图片文字编辑（替换/删除/移动）

```bash
# 步骤1：扫描获取版面结构和字符索引
camscanner-cli image scan document.jpg

# 步骤2：在 scan 返回的 JSON 中定位目标文字的 start_char_idx 和 end_char_idx
# （从 result.document_info.sections[].columns[].paragraphs[].lines[].characters 中查找）

# 步骤3：执行编辑（替换文字示例）
camscanner-cli image edit \
  --input-image "<result.urls.input_image>" \
  --document-info "<result.urls.document_info>" \
  --edit-request '{"edit_type":"update","start_char_idx":39,"end_char_idx":40,"target_text":"北京"}' \
  -o edited.jpg -s --save-title "图片文字编辑"
```

### 搜索并下载云文档

```bash
# 步骤1：搜索目标文档
camscanner-cli doc search "合同"

# 步骤2：Agent 向用户展示搜索结果，用户确认要下载的文档
# 步骤3：使用用户确认的 cs_doc_id 下载到本地
camscanner-cli doc download <cs_doc_id> -o ~/Downloads/合同.pdf
```

### 处理文件并保存到指定文件夹

```bash
# 转换图片为 Word 并直接保存到指定文件夹
camscanner-cli image convert photo.jpg --format word -s --save-dir "工作文档"
```

### 整理文档到文件夹（需用户确认）

```bash
# 步骤1：搜索需要整理的文档
camscanner-cli doc search "发票" -n 20

# 步骤2：Agent 向用户展示搜索结果，用户确认要移动的文档
# 步骤3：查看文件夹列表获取 dir_id
camscanner-cli doc dirs

# 步骤4：在文件夹列表中匹配用户指定的文件夹名称
#   - 精确匹配：使用对应 dir_id
#   - 相近名称：列出相近文件夹让用户确认
#   - 无匹配：告知用户文件夹不存在（CLI 不支持创建文件夹），列出所有可用文件夹供选择

# 步骤5：使用用户确认的 cs_doc_id 和匹配到的 dir_id 移动
camscanner-cli doc move <cs_doc_id> --dir-id <folder_id>
```

## 场景映射

| 场景 | 最佳方案 |
|------|----------|
| 会议白板照片 → 可编辑文档 | `image convert whiteboard.jpg --format word -o "whiteboard_convert.docx" -s` |
| 论文扫描件 → Markdown | `image merge-text "page1.jpg" "page2.jpg" --format md -o paper.md`（多图文本合并仅本地）；单图可用 `image convert page1.jpg --format md -o page1.md -s` |
| 发票照片 → 结构化数据提取 | `image receipt invoice.jpg` |
| 发票照片 → Excel 表格 | `image convert invoice.jpg --format excel -o "invoice_convert.xlsx" -s` |
| 合同 PDF → Word 编辑 | `pdf convert contract.pdf --format word -o "contract_convert.docx" -s` |
| Word 报告 → PDF | `office convert report.docx -o "report.pdf" -s --save-title "报告PDF"` |
| 旧版 DOC → DOCX 升级 | `office convert old.doc --format docx -o "old_upgraded.docx" -s` |
| Excel 数据 → PDF | `office convert data.xlsx -o "data.pdf" -s --save-title "数据表PDF"` |
| PPT 演示 → PDF | `office convert slides.pptx -o "slides.pdf" -s --save-title "演示文稿PDF"` |
| 名片照片 → 文字提取 | `image ocr namecard.jpg` |
| 外文菜单 → 中文翻译 | `image translate menu.jpg --lang zh -o "menu_translate.jpg" -s` |
| 手写笔记 → 电子文档 | `image enhance notes.jpg --mode 9 -o clean.jpg` 然后 `image convert clean.jpg --format word -o "clean_convert.docx" -s` |
| 模糊证件照 → 高清 | `image hd id_photo.jpg -o "id_photo_hd.jpg" -s` |
| 老照片修复 | `image restore vintage.jpg -o "vintage_restore.jpg" -s` |
| 多张试卷图片 → 一个 PDF | `image merge-pdf "q1.jpg" "q2.jpg" "q3.jpg" -o "merged.pdf" -s` |
| 搜索云端文档 | `doc search "关键词" -f pdf -n 10` |
| 下载云端文档 | `doc download <doc_id> -o ~/Downloads/文件.pdf` |
| 查看云端文件夹 | `doc dirs` |
| 移动文档到文件夹 | `doc move <doc_id> --dir-id <folder_id>` |
| 处理后保存到文件夹 | `image convert photo.jpg --format word -s --save-dir "文件夹名"` |
