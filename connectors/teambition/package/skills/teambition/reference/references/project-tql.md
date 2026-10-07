# 项目 TQL 参考

项目 TQL 用于查询 Teambition 项目。

## 构造流程

1. 将自然语言条件拆成独立谓词，并区分可直接翻译、需要解析为 ID 和暂不可表达三类；需要解析的
   谓词先通过只读查询获得唯一 ID，暂不可表达的条件不得静默省略。
2. 默认用 `AND` 组合同时成立的条件；只有用户明确表达“任一、或者”时才使用 `OR`。
3. 先构造完整布尔谓词，再分离可选的尾部 `ORDER BY`。普通项目查询加入默认谓词
   `isTemplate = false`；用户明确查询模板项目，或完整 TQL 已显式约束 `isTemplate` 时不再追加。
4. 没有其他谓词时直接使用默认谓词；已有谓词时用 `AND` 连接；表达式包含 `OR` 时先用括号包住
   完整表达式。最后接回唯一的 `ORDER BY`，并校验括号成对、无前导或尾随逻辑运算符。
5. 字符串字面量使用单引号；函数、布尔值、`null` 和数值不加引号。传给 shell 时用双引号包住
   整个 flag 值。

示例：

```text
(nameText ~ '电商' OR text ~ '协作') AND isTemplate = false ORDER BY updated DESC
```

## 常用场景速查

> 下表示例是条件片段。普通项目查询必须按上述构造流程显式加入
> `isTemplate = false`；禁止直接在空表达式或 `ORDER BY` 后追加 `AND isTemplate = false`。
> 仅当用户明确查询模板项目，或完整 TQL 已显式约束 `isTemplate` 时不追加。

| 场景 | TQL |
|------|-----|
| 我参与的项目 | `involveMembers = me()` |
| 我创建的项目 | `creatorId = me()` |
| 名称模糊搜索 | `nameText ~ '电商'` |
| 名称或描述全文搜索 | `text ~ '关键词'` |
| 未归档项目 | `isSuspended = false` |
| 已归档项目 | `isSuspended = true` |
| 在回收站的项目 | `isArchived = true` |
| 今天创建 | `created >= startOf(d) AND created <= endOf(d)` |
| 本周创建 | `created >= startOf(w) AND created <= endOf(w)` |
| 今天更新 | `updated >= startOf(d) AND updated <= endOf(d)` |
| 过去7天创建 | `created >= startOf(d, -7d) AND created <= endOf(d, -1d)` |

## 字段说明

| 字段 | 支持的操作符 | 说明 |
|------|------------|------|
| `involveMembers` | `=` `!=` `IN` `NOT IN` | 项目成员 ID，用 `me()` 表示当前用户 |
| `creatorId` | `=` `!=` `IN` `NOT IN` | 创建人 ID，支持 `me()` |
| `nameText` | `=` `~` | 项目名称（`~` 模糊匹配，`=` 精确匹配） |
| `text` | `~` | 全文搜索（项目名称 + 项目描述） |
| `description` | `=` | 项目描述 |
| `isSuspended` | `=` | 是否已归档（`true` / `false`） |
| `isArchived` | `=` | 是否在回收站（`true` / `false`） |
| `isTemplate` | `=` | 是否是模板项目（`true` / `false`） |
| `visibility` | `=` | 可见性：`project`（私有）/ `organization`（企业公开）/ `org`（公开） |
| `created` | `=` `!=` `>` `>=` `<` `<=` | 创建时间 |
| `updated` | `=` `!=` `>` `>=` `<` `<=` | 更新时间 |
| `cf:<fieldId>` (数字) | `=` `!=` `>` `>=` `<` `<=` | 自定义字段（数字类型） |
| `cf:<fieldId>` (文本) | `=` `!=` | 自定义字段（文本类型） |
| `cf:<fieldId>` (日期) | `=` `!=` `>` `>=` `<` `<=` | 自定义字段（日期类型） |
| `cf:<fieldId>` (多选) | `=` `!=` `IN` `NOT IN` | 自定义字段（多选类型） |
| `cf:<fieldId>` (单选) | `=` `!=` `IN` `NOT IN` | 自定义字段（单选类型） |

## 时间偏移速查（以 `created` 为例，`updated` 同理）

| 场景 | TQL |
|------|-----|
| 今天 | `created >= startOf(d) AND created <= endOf(d)` |
| 昨天 | `created >= startOf(d, -1d) AND created <= endOf(d, -1d)` |
| 过去3天 | `created >= startOf(d, -3d) AND created <= endOf(d, -1d)` |
| 过去7天 | `created >= startOf(d, -7d) AND created <= endOf(d, -1d)` |
| 过去30天 | `created >= startOf(d, -30d) AND created <= endOf(d, -1d)` |
| 最近7天（含今天） | `created >= startOf(d, -6d) AND created <= endOf(d)` |
| 本周 | `created >= startOf(w) AND created <= endOf(w)` |
| 上周 | `created >= startOf(w, -1w) AND created <= endOf(w, -1w)` |
| 本月 | `created >= startOf(M) AND created <= endOf(M)` |
| 上月 | `created >= startOf(M, -1M) AND created <= endOf(M, null, -1M)` |
| 今年 | `created >= startOf(y) AND created <= endOf(y)` |
| 指定日期之后 | `created >= '2026-03-01T00:00:00+08:00'` |
| 指定日期范围 | `created >= '2026-03-01T00:00:00+08:00' AND created <= '2026-03-31T23:59:59+08:00'` |


## 最佳实践

1. **默认排除模板**：普通项目查询按“谓词 → `isTemplate = false` → `ORDER BY`”的顺序拼装；
   不要假设 CLI 或服务端会自动添加
2. **必须使用 `me()`**：查询"我的"项目时，用 `involveMembers = me()` 而非硬编码用户 ID
3. **项目无 dueDate**：项目没有截止时间字段，只有 `created` 和 `updated`
4. **复杂条件加括号**：多个 OR 条件与 AND 组合时，用括号明确优先级，避免歧义
5. **合理排序**：时间相关查询建议加 `ORDER BY updated DESC` 或 `ORDER BY created DESC`
