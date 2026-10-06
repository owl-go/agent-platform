# CamScanner CLI 1.1.8 help

### image convert

```text
将图片转换为 Word/Excel/TXT/Markdown 格式

Usage:
  camscanner-cli image convert [file] [flags]

Flags:
      --format string        目标格式 (word/excel/txt/md) (default "word")
  -h, --help                 help for convert
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image edit

```text
基于 scan 返回的 document_info 对图片进行编辑（修改文字、移动/删除元素）

Usage:
  camscanner-cli image edit [flags]

Flags:
      --document-info string   scan 返回的 document_info OSS key（必填）
      --edit-request string    编辑请求 JSON 字符串（必填）
  -h, --help                   help for edit
      --input-image string     scan 返回的 input_image OSS key（必填）
  -o, --output string          输出文件路径
  -s, --save                   保存到云文档
      --save-dir string        保存到指定文件夹（按名称）
      --save-dir-id string     保存到指定文件夹（按 ID）
      --save-title string      云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image enhance

```text
图片增强

Usage:
  camscanner-cli image enhance [file] [flags]

Flags:
  -h, --help                 help for enhance
      --mode int             增强模式 (1=亮度 2=锐化 3=黑白 4=灰度 5=去阴影 6=去点阵 7=超级滤镜 8=去摩尔纹 9=手写擦除 10=去水印)
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image extract-formula

```text
提取数学公式

Usage:
  camscanner-cli image extract-formula [file] [flags]

Flags:
  -h, --help                 help for extract-formula
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image hd

```text
图片高清化

Usage:
  camscanner-cli image hd [file] [flags]

Flags:
  -h, --help                 help for hd
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image merge-excel

```text
多图合并为 Excel

Usage:
  camscanner-cli image merge-excel [files...] [flags]

Flags:
  -h, --help                 help for merge-excel
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image merge-pdf

```text
多图合并为 PDF

Usage:
  camscanner-cli image merge-pdf [files...] [flags]

Flags:
  -h, --help                 help for merge-pdf
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image merge-text

```text
多图 OCR 合并为文本

Usage:
  camscanner-cli image merge-text [files...] [flags]

Flags:
      --format string   输出格式 (txt/md) (default "txt")
  -h, --help            help for merge-text
  -o, --output string   输出文件路径（不指定则打印到终端）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image merge-word

```text
多图合并为 Word

Usage:
  camscanner-cli image merge-word [files...] [flags]

Flags:
  -h, --help                 help for merge-word
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image ocr

```text
OCR 文字识别

Usage:
  camscanner-cli image ocr [file] [flags]

Flags:
  -h, --help   help for ocr

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image receipt

```text
识别图片中的发票/票据信息，返回结构化 JSON（发票类型、金额、日期、号码等）

Usage:
  camscanner-cli image receipt [file] [flags]

Flags:
  -h, --help            help for receipt
      --json            以原始 JSON 格式输出（默认为人类可读格式）
  -o, --output string   输出 JSON 文件路径（不指定则打印到终端）

Global Flags:
      --human   人类可读格式输出
```

### image restore

```text
照片修复

Usage:
  camscanner-cli image restore [file] [flags]

Flags:
  -h, --help                 help for restore
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image scan

```text
分析图片版面结构（文字位置、表格、图片元素），返回 document_info 供 edit 命令使用

Usage:
  camscanner-cli image scan [file] [flags]

Flags:
  -h, --help             help for scan
      --include-layers   返回图层分离结果
  -o, --output string    输出 JSON 文件路径（不指定则打印到终端）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image to-pdf

```text
图片转 PDF

Usage:
  camscanner-cli image to-pdf [file] [flags]

Flags:
  -h, --help                 help for to-pdf
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image translate

```text
翻译图片中的文字，保留原始排版

Usage:
  camscanner-cli image translate [file] [flags]

Flags:
  -h, --help                 help for translate
      --lang string          目标语言 (default "en")
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image validate

```text
mode=1: PS篡改检测; mode=2: AI生成检测

Usage:
  camscanner-cli image validate [file] [flags]

Flags:
  -h, --help       help for validate
      --mode int   检测模式 (1=篡改检测, 2=AI生成检测) (default 1)

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### image watermark

```text
添加文字水印

Usage:
  camscanner-cli image watermark [file] [flags]

Flags:
      --color string         水印颜色 (如 #FF0000)
  -h, --help                 help for watermark
      --opacity float        透明度 (0-1)
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）
      --size int             字体大小
      --text string          水印文字（必填）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### pdf convert

```text
将 PDF 转换为 Word/Excel/TXT/Markdown 格式

Usage:
  camscanner-cli pdf convert [file] [flags]

Flags:
      --format string        目标格式 (word/excel/txt/md) (default "word")
  -h, --help                 help for convert
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### pdf remove-watermark

```text
去除 PDF 水印

Usage:
  camscanner-cli pdf remove-watermark [file] [flags]

Flags:
  -h, --help                 help for remove-watermark
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### pdf to-images

```text
将 PDF 每页渲染为 JPEG 并下载到本地目录，产物为 page_1.jpg、page_2.jpg……

Usage:
  camscanner-cli pdf to-images [file] [flags]

Flags:
  -d, --dir string           输出目录（不指定则为 <输入文件名>_pages）
  -h, --help                 help for to-images
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### pdf to-images-zip

```text
将 PDF 每页渲染为 JPEG 图片，由服务端打包为单个 ZIP 文件

Usage:
  camscanner-cli pdf to-images-zip [file] [flags]

Flags:
  -h, --help            help for to-images-zip
  -o, --output string   输出 ZIP 文件路径（不指定则为 <输入文件名>.zip）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### pdf watermark

```text
添加文字水印

Usage:
  camscanner-cli pdf watermark [file] [flags]

Flags:
      --color string         水印颜色 (如 #FF0000)
  -h, --help                 help for watermark
      --opacity float        透明度 (0-1)
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）
      --size int             字体大小
      --text string          水印文字（必填）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### office convert

```text
将 Word/Excel/PPT 文档转换为 PDF 或升级格式

支持的输入格式和转换目标:
  Word  (.doc/.docx) → pdf, docx
  Excel (.xls/.xlsx) → pdf, xlsx
  PPT   (.ppt/.pptx) → pdf, pptx

示例:
  camscanner-cli office convert report.docx
  camscanner-cli office convert data.xls --format pdf
  camscanner-cli office convert slides.ppt --format pptx
  camscanner-cli office convert old.doc --format docx

Usage:
  camscanner-cli office convert [file] [flags]

Flags:
      --format string        目标格式 (pdf/docx/xlsx/pptx) (default "pdf")
  -h, --help                 help for convert
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### txt to-word

```text
将纯文本文件按行转换为 Word 文档 (.docx)

Usage:
  camscanner-cli txt to-word [file] [flags]

Flags:
  -h, --help                 help for to-word
  -o, --output string        输出文件路径
  -s, --save                 保存到云文档
      --save-dir string      保存到指定文件夹（按名称）
      --save-dir-id string   保存到指定文件夹（按 ID）
      --save-title string    云文档标题（不传则自动生成）
      --title string         文档标题（不传则自动生成）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### doc dirs

```text
查询并展示用户云端文件夹目录树。

Usage:
  camscanner-cli doc dirs [flags]

Flags:
  -h, --help   help for dirs

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### doc download

```text
将云端文档下载到本地。Office 文档保持原格式；图片文档默认导出为 PDF，可选导出为 JPG ZIP 包。

示例:
  camscanner-cli doc download 7DD6DB7134654A5Fbg9af7hS_word1
  camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf
  camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf -f zip
  camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf -o ~/Downloads/扫描件.pdf

Usage:
  camscanner-cli doc download <doc_id> [flags]

Flags:
  -f, --format string   图片文档导出格式: pdf（默认）或 zip
  -h, --help            help for download
  -o, --output string   输出文件路径（不指定则使用 doc_id 自动命名）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### doc move

```text
将一个或多个云端文档移动到指定文件夹。
通过文件夹 ID（--dir-id）指定目标，或使用 --root 移动到根目录。
可先通过 doc dirs 查看文件夹列表获取 dir_id。

示例:
  camscanner-cli doc move DOC_ID --dir-id EBC6aFh3WHgTP9VWWUeP2R4J
  camscanner-cli doc move DOC_ID_1 DOC_ID_2 --dir-id EBC6aFh3WHgTP9VWWUeP2R4J
  camscanner-cli doc move DOC_ID --root

Usage:
  camscanner-cli doc move <doc_id> [doc_id...] [flags]

Flags:
      --dir-id string   目标文件夹 ID
  -h, --help            help for move
      --root            移动到根目录（与 --dir-id 互斥）

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```

### doc search

```text
搜索用户云端文档。支持关键词搜索和最近文档搜索两种模式。

传入 keyword 时走关键词全文搜索（标题+OCR+备注）；
不传 keyword 时走最近文档搜索。两种模式均支持按文档类型过滤。

Usage:
  camscanner-cli doc search [keyword] [flags]

Flags:
  -a, --after string    起始时间过滤（支持 2006-01-02、2006-01-02 15:04:05 或 unix 时间戳）
  -b, --before string   截止时间过滤（支持 2006-01-02、2006-01-02 15:04:05 或 unix 时间戳）
  -f, --filter string   文档类型过滤: pdf/word/excel/ppt/image/markdown/html
  -h, --help            help for search
  -n, --limit int       返回文档数量上限（最大 50） (default 5)
  -s, --scope string    搜索范围: title（默认）或 full (default "title")

Global Flags:
      --human   人类可读格式输出
      --json    JSON 格式输出
```
