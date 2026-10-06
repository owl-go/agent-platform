# 云文档管理参考

### doc search — 搜索云文档

搜索用户云端的 CamScanner 文档。支持关键词搜索、时间范围过滤、文档类型过滤，可组合使用。

```bash
camscanner-cli doc search [keyword] [flags]
```

| 参数/标志 | 说明 |
|----------|------|
| `keyword`（位置参数） | 搜索关键词（多个词用空格分隔，任一命中即匹配） |
| `-f, --filter` | 文档类型过滤：pdf/word/excel/ppt/image/markdown/html |
| `-n, --limit` | 返回数量上限（默认 5，最大 50） |
| `-a, --after` | 起始时间（支持 `2006-01-02`、`2006-01-02 15:04:05` 或 unix 时间戳） |
| `-b, --before` | 截止时间（同上格式） |
| `-s, --scope` | 搜索范围：`title`（默认，标题+页标题+备注）或 `full`（含 OCR 全文） |

**使用示例**：

```bash
# 搜索包含"合同"的文档
camscanner-cli doc search "合同"

# 搜索最近的 PDF 文档
camscanner-cli doc search -f pdf -n 10

# 搜索指定时间范围内的文档
camscanner-cli doc search --after 2026-08-01 --before 2026-08-31

# 关键词 + 类型 + 时间组合搜索
camscanner-cli doc search "报告" -f word --after 2026-08-01

# 全文搜索（包含 OCR 内容）
camscanner-cli doc search "发票编号" -s full -n 20
```

**输出格式**：CLI 以表格形式展示搜索结果。

**Agent 展示规范（强制）**：Agent 向用户呈现搜索结果时，**必须**至少包含以下四列信息：

| 列名 | 来源 | 说明 |
|------|------|------|
| 标题 | CLI 输出的「标题」列 | 文档标题 |
| 类型 | 从链接 URL 路径推断（见下方规则） | 文档类型 |
| 所在目录 | CLI 输出的「所在目录」列 | 文档所在文件夹 |
| 链接 | CLI 输出的「链接」列 | 可点击的 Web 承接页地址 |

> **类型推断规则**（两步推断）：
> 1. **优先从链接路径推断**：`/pdfDetail` → PDF，`/markdownDetail` → Markdown，`/detail` → 扫描件/图片，`/htmlDetail` → HTML
> 2. **路径无法区分时，用 cs_doc_id 后缀补充推断**：`/officeDetail` 路径对应 Word/Excel/PPT 三种类型，此时根据文档ID列的后缀区分：`_word1` → Word，`_exce1` → Excel，`_pptx1` → PPT
>
> Agent 禁止省略类型列或仅展示标题和链接。

> **cs_doc_id（内部使用）**：CLI 输出的「文档ID」列包含 `cs_doc_id`，这是执行 `doc download` 和 `doc move` 的必要参数。Agent 应从搜索结果中记录此值以供后续操作使用，但**无需向用户展示**。

**Agent 行为规范**：
- 用户说"找/搜/查我的文档"时，走 `doc search`，**不走** image/pdf 处理流程
- 多关键词用空格分隔，采用 OR 语义（任一命中即匹配）
- 未传 keyword 时返回最近文档列表
- 默认返回 5 条，用户需要更多时增加 `-n`

**关键词分词策略**：

Agent 应对用户描述的搜索内容进行合理分词，用空格分隔传入，以提高命中概率（OR 语义下多词命中更广）。但分词必须谨慎：
- **应分词**：用户说"论文公式高清"→ 拆为 `"论文 公式 高清"`；用户说"会议纪要8月"→ 拆为 `"会议纪要 8月"`
- **不应分词**：专有名词、品牌名、人名、固定短语不可强行拆分。如"扫描全能王"不拆、"张三的报告"中"张三"不拆
- **无法确定时不分词**：如果不确定拆分是否能提高效果，保持用户原文作为单个关键词传入

**语义意图识别**：

Agent 必须对用户查询进行语义解析，将时间、类型等结构化意图提取为对应参数，**而非当作搜索关键词**：

- **时间意图 → `--after` / `--before` 参数**：用户提到时间范围时，解析为时间过滤条件，不作为关键词
  - "去年8月份的论文" → `doc search "论文" --after 2025-08-01 --before 2025-08-31`
  - "上周的会议纪要" → `doc search "会议纪要" --after 2026-08-17 --before 2026-08-23`
  - "今年的合同" → `doc search "合同" --after 2026-01-01`
- **类型意图 → `-f` 参数**：用户提到文档类型时，映射为类型过滤
  - "找一下我的 PDF 发票" → `doc search "发票" -f pdf`
- **数量意图 → `-n` 参数**：用户说"最近几个"、"多找一些"等，调整返回数量

> **核心原则**：分词策略仅针对**真正的搜索关键词**部分。时间、类型、数量等结构化语义必须提取为对应的命令参数，绝不能混入关键词中。错误示范：`doc search "去年8月份 论文"` — 这会把"去年8月份"当文本去匹配文档内容，而不是按时间过滤。

**搜索范围决策（`-s` 参数）**：

| 用户意图 | 使用参数 |
|----------|----------|
| 明确说"标题中"、"备注中"、"页标题" | `-s title` |
| 明确说"文档内容中"、"正文里"、"全文搜索" | `-s full` |
| 无明确意图（默认） | 先用 `-s title` 搜索；若无结果，自动用 `-s full` 重试一次 |

> **两步搜索策略**：用户未明确搜索范围时，先 title 搜索（速度快），无结果再 full 搜索（覆盖 OCR 全文）。两次都无结果时说明本次搜索未找到匹配文档，不据此断言文档不存在或云保存一定未发生。

### doc download — 下载云文档

将云端文档下载到本地。Office 文档（Word/Excel/PPT/PDF/Markdown/HTML）保持原格式；图片类文档默认导出为 PDF，可选导出为 JPG ZIP 包。

```bash
camscanner-cli doc download <doc_id> [flags]
```

| 参数/标志 | 说明 |
|----------|------|
| `doc_id`（位置参数） | 文档 ID（必填，可从 `doc search` 结果获取） |
| `-o, --output <path>` | 输出文件路径（不指定则使用 doc_id 自动命名） |
| `-f, --format` | 图片文档导出格式：`pdf`（默认）或 `zip`（JPG 压缩包） |

**使用示例**：

```bash
# 下载 Word 文档（自动保持 .docx 格式）
camscanner-cli doc download 7DD6DB7134654A5Fbg9af7hS_word1

# 下载图片文档为 PDF
camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf

# 下载图片文档为 JPG ZIP 包
camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf -f zip

# 指定输出路径
camscanner-cli doc download D5BC4625B8A14F3EVX7tY9Hf -o ~/Downloads/扫描件.pdf
```

**Agent 行为规范**：
- `doc download` 需要 `cs_doc_id` 参数（来自 `doc search` 结果中的"文档ID"列），**没有 cs_doc_id 就无法下载**
- **搜索结果必须经用户确认后才能操作**：即使搜索结果只有一条，Agent 也必须向用户展示结果并等待用户明确确认后再执行下载。**禁止**跳过确认步骤自动执行
- **若用户未提供 cs_doc_id 且当前会话无搜索结果**，Agent 必须先引导用户执行 `doc search` 搜索、确认目标文档后再下载
  - 示例：用户说"下载我的合同" → Agent 回复"我先帮您搜索一下包含'合同'的文档"→ 执行 `doc search "合同"` → 展示结果让用户确认 → 用户确认后执行 `doc download <cs_doc_id>`
- Office 文档（doc_id 尾缀含 `_word1`、`_exce1`、`_pdfx0` 等）自动保持原格式，无需指定 `-f`
- 图片类文档（无已知尾缀）默认导出 PDF；用户需要原始图片时使用 `-f zip`
- 执行前检查输出路径是否已存在文件，避免静默覆盖
- **下载完成后引导规则**：下载成功后，Agent 应根据文件类型提示用户可以继续处理该文件。处理流程完全复用现有的文件处理能力（上传→处理→保存云文档）。引导规则如下：
  - **PDF 文件**（图片文档导出的 PDF 或原生 PDF）：提示可以进行格式转换（转 Word/Excel/Markdown/TXT）、拆分为图片、加水印或去水印
  - **图片文件**（通过 `-f zip` 导出后解压的 JPG）：提示可以进行增强/高清化/修复、格式转换、OCR、翻译、公式提取、文字编辑等
  - **TXT/Markdown 文件**：提示可以转换为 Word 文档
  - **Word/Excel/PPT/HTML 文件**：当前暂无可用的处理能力，不做引导
  - 引导方式：下载完成后简要告知用户"如需进一步处理此文件，可以……"并列出 2-3 个最常用的操作建议。**不要强制引导**，用户不需要时直接结束即可

### doc dirs — 查看文件夹列表

查询并展示用户云端文件夹目录树。用于获取文件夹 ID 以供 `doc move`、`--save-dir-id` 等命令使用。

```bash
camscanner-cli doc dirs
```

无参数，直接执行即可。

**输出格式**：树状目录结构，每行显示文件夹名称、文档数量和文件夹 ID：

```
文件夹列表（共 3 个）:

├── 工作文档 (5)  [EBC6aFh3WHgTP9VWWUeP2R4J]
│   └── 合同 (2)  [FCC7bGi4XIhUQ0WXXVfQ3S5K]
└── 个人资料 (3)  [GDD8cHj5YJiVR1XYYWgR4T6L]
```

**Agent 行为规范**：
- 用户问"我有哪些文件夹"、"查看目录"、"列出文件夹"时使用 `doc dirs`
- 当需要获取 dir_id 给 `doc move` 或 `--save-dir-id` 使用时，先执行 `doc dirs` 获取列表
- 向用户展示结果时保留树状结构，方括号中为文件夹 ID

### doc move — 移动文档

将一个或多个云端文档移动到指定文件夹，或移回根目录。

```bash
camscanner-cli doc move <doc_id> [doc_id...] --dir-id <folder_id>
camscanner-cli doc move <doc_id> [doc_id...] --root
```

| 参数/标志 | 说明 |
|----------|------|
| `doc_id`（位置参数） | 文档 ID，支持多个（必填） |
| `--dir-id <id>` | 目标文件夹 ID（通过 `doc dirs` 获取） |
| `--root` | 移动到根目录（与 `--dir-id` 互斥） |

**使用示例**：

```bash
# 移动单个文档到指定文件夹
camscanner-cli doc move DOC_ID --dir-id EBC6aFh3WHgTP9VWWUeP2R4J

# 批量移动多个文档
camscanner-cli doc move DOC_ID_1 DOC_ID_2 --dir-id EBC6aFh3WHgTP9VWWUeP2R4J

# 移动文档回根目录
camscanner-cli doc move DOC_ID --root
```

**Agent 行为规范**：
- `doc move` 需要明确的 `cs_doc_id`（来自 `doc search` 结果），**没有 cs_doc_id 就无法移动**
- `--dir-id` 和 `--root` 必须二选一，不能同时使用，也不能都不传
- **移动是会改变用户云端数据的危险操作**，Agent 必须格外谨慎：
  - **禁止**在用户未确认具体文档的情况下执行移动。即使搜索到了匹配文档，也必须向用户展示并获得明确确认后才能操作
  - **禁止**批量移动用户未逐一确认的文档。例如用户说"帮我把PDF文档移到根目录"，**绝对不能**自动搜索所有 PDF 然后全部移动
  - 若用户未提供 cs_doc_id，Agent 必须先引导搜索、展示结果、让用户确认具体是哪个/哪些文档后再执行
- 支持一次移动多个文档，cs_doc_id 用空格分隔
- **文件夹匹配流程（用户指定文件夹名称时必须遵循）**：
  1. 先执行 `doc dirs` 获取完整文件夹列表
  2. 在返回的列表中匹配用户说的文件夹名称
  3. **精确匹配成功**：使用对应的 dir_id 执行 `doc move --dir-id <dir_id>`
  4. **存在相近名称的文件夹**：向用户列出相近的文件夹名称，让用户确认要移动到哪个。例如用户说"工作文件"，但只有"工作文档"和"工作资料"，应提示"未找到'工作文件'文件夹，您是否要移动到以下文件夹？1. 工作文档 2. 工作资料"
  5. **完全没有匹配**：直接告知用户该文件夹不存在，列出当前所有可用文件夹供用户选择。**禁止**静默降级到根目录或猜测执行
- **移动成功后的反馈**：CLI 输出中的目标是 dir_id（如 `7KT73KB8DyCTr3LAgTbFUEHL`），对用户无意义。Agent 在移动前已通过 `doc dirs` 获取了文件夹名称，**必须**用文件夹名称告知用户移动结果，例如：`已将"合同文档"移动到"工作文档"文件夹`。多层目录时展示完整路径，如 `已移动到"创建文件夹测试/tt1"`
- **典型多步流程**：用户说"把合同移到工作文档文件夹" → Agent 回复"我先帮您搜索'合同'文档"→ 执行 `doc search "合同"` → 展示结果 → 用户确认"移动第1个"→ 执行 `doc dirs` 获取 dir_id → 在列表中匹配"工作文档"→ 匹配成功则执行 `doc move <cs_doc_id> --dir-id <dir_id>` → 成功后告知用户"已将'合同文档'移动到'工作文档'文件夹"

### 云文档工作流组合

**搜索 → 下载**：用户需要获取云端文档到本地时的典型流程。

```bash
# 步骤1：搜索找到目标文档
camscanner-cli doc search "合同"
# 步骤2：Agent 向用户展示搜索结果，用户确认要下载的文档
# 步骤3：使用用户确认的文档对应的 cs_doc_id 下载
camscanner-cli doc download <cs_doc_id> -o ~/Downloads/合同.pdf
```

**处理 → 保存到文件夹**：处理本地文件后自动保存到指定文件夹。

```bash
# 转换图片为 Word 并保存到"工作文档"文件夹
camscanner-cli image convert photo.jpg --format word -s --save-dir "工作文档"
```

**搜索 → 移动归档**：将已有云文档整理到文件夹。**移动是危险操作，必须经用户确认。**

```bash
# 步骤1：搜索找到目标文档
camscanner-cli doc search "发票"
# 步骤2：Agent 向用户展示搜索结果，用户确认要移动的文档
# 步骤3：查看文件夹列表
camscanner-cli doc dirs
# 步骤4：在文件夹列表中匹配用户指定的文件夹名称
#   - 精确匹配：使用对应 dir_id
#   - 相近名称：列出相近文件夹让用户确认
#   - 无匹配：告知用户文件夹不存在，列出所有可用文件夹供选择
# 步骤5：使用用户确认的 cs_doc_id 和匹配到的 dir_id 移动
camscanner-cli doc move <cs_doc_id> --dir-id <folder_id>
```
