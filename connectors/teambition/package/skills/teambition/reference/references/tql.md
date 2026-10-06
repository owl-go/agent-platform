# 任务 TQL 参考

TQL（Task Query Language）用于查询 Teambition 任务。

## 目录

- [构造流程](#构造流程)
- [常用场景速查](#常用场景速查)
- [字段说明](#字段说明)
- [时间函数](#时间函数)
- [运算符](#运算符)
- [排序](#排序)
- [注意事项](#注意事项)

## 构造流程

1. 将自然语言条件拆成独立谓词，并区分可直接翻译、需要解析为 ID 和暂不可表达三类；需要解析的
   谓词先通过只读查询获得唯一 ID，暂不可表达的条件不得静默省略。
2. 默认用 `AND` 组合同时成立的条件；只有用户明确表达“任一、或者”时才使用 `OR`。
3. 先构造完整布尔谓词，再分离可选的尾部 `ORDER BY`。普通任务查询加入默认谓词
   `isArchived = false`；用户明确查询回收站/已归档任务，或完整 TQL 已显式约束 `isArchived`
   时不再追加。
4. 没有其他谓词时直接使用默认谓词；已有谓词时用 `AND` 连接；表达式包含 `OR` 时先用括号包住
   完整表达式。最后接回唯一的 `ORDER BY`，并校验括号成对、无前导或尾随逻辑运算符。
5. 字符串字面量使用单引号；函数、布尔值、`null` 和数值不加引号。传给 shell 时用双引号包住
   整个 flag 值。

示例：

```text
(priority = <priorityValue1> OR priority = <priorityValue2>) AND isArchived = false ORDER BY dueDate ASC
```

## 常用场景速查

> **重要**：以下所有 TQL 示例均包含 `isArchived = false` 以排除回收站/已归档任务。拼装新查询时
> 按上述构造流程加入该默认谓词，禁止直接在空表达式或 `ORDER BY` 后追加
> `AND isArchived = false`。仅当用户明确查询回收站/已归档任务，或完整 TQL 已显式约束
> `isArchived` 时不追加。

### 个人任务

| 场景 | TQL |
|------|-----|
| 我的待办任务 | `executorId = me() AND isDone = false AND isArchived = false` |
| 我的逾期任务 | `executorId = me() AND isDone = false AND dueDate < startOf(d) AND isArchived = false` |
| 我的已完成 | `executorId = me() AND isDone = true AND isArchived = false` |
| 我今天截止的任务 | `executorId = me() AND dueDate >= startOf(d) AND dueDate <= endOf(d) AND isArchived = false` |
| 我本周截止的任务 | `executorId = me() AND dueDate >= startOf(w) AND dueDate <= endOf(w) AND isArchived = false` |
| 我创建的未完成任务 | `creatorId = me() AND isDone = false AND isArchived = false` |
| 我参与的任务 | `involveMembers = me() AND isArchived = false` |
| 即将逾期（未来3天截止） | `executorId = me() AND isDone = false AND dueDate <= endOf(d, 3d) AND dueDate >= startOf(d) AND isArchived = false` |

### 团队任务

| 场景 | TQL |
|------|-----|
| 未指派的任务 | `executorId = null AND isDone = false AND isArchived = false` |
| 指定优先级未完成 | `priority = <priorityValue> AND isDone = false AND isArchived = false` |
| 本周创建的任务 | `created >= startOf(w) AND created <= endOf(w) AND isArchived = false` |
| 指定项目 | `projectId = 'xxx' AND isArchived = false` |
| 标题模糊搜索 | `title ~ '关键词' AND isArchived = false` |
| 全文搜索（标题+备注+短ID） | `text ~ '关键词' AND isArchived = false` |
| 上周更新 | `updated >= startOf(w, -1w) AND updated <= endOf(w, -1w) AND isArchived = false` |
| 过去7天更新 | `updated >= startOf(d, -7d) AND updated <= endOf(d) AND isArchived = false` |

## 字段说明

| 字段 | 支持的操作符 | 说明 |
|------|------------|------|
| `executorId` | `=` `!=` `IN` `NOT IN` | 执行人 ID，用 `me()` 表示当前用户，`null` 表示未指派 |
| `creatorId` | `=` `!=` `IN` `NOT IN` | 创建人 ID |
| `involveMembers` | `=` `!=` `IN` `NOT IN` | 参与者 ID |
| `isDone` | `=` `!=` | 是否完成（`true` / `false`） |
| `isArchived` | `=` `!=` | 是否在回收站/已归档（`true` / `false`），查询时默认追加 `AND isArchived = false` 排除 |
| `dueDate` | `=` `!=` `>` `>=` `<` `<=` | 截止时间 |
| `startDate` | `=` `!=` `>` `>=` `<` `<=` | 开始时间 |
| `accomplished` | `=` `!=` `>` `>=` `<` `<=` | 完成时间 |
| `created` | `=` `!=` `>` `>=` `<` `<=` | 创建时间 |
| `updated` | `=` `!=` `>` `>=` `<` `<=` | 更新时间 |
| `priority` | `=` `!=` `IN` `NOT IN` | 企业配置中的优先级值；名称必须先解析为当前企业返回的值 |
| `projectId` | `=` `!=` `IN` `NOT IN` | 项目 ID |
| `title` | `~` | 任务标题（模糊匹配） |
| `text` | `~` | 全文搜索（标题 + 备注 + 短ID） |
| `tagId` | `=` `!=` `IN` `NOT IN` | 标签 ID |
| `stageId` | `=` `!=` `IN` `NOT IN` | 任务列 ID |
| `taskflowstatusId` | `=` `!=` `IN` `NOT IN` | 任务状态 ID |
| `scenarioId` | `=` `!=` `IN` `NOT IN` | 任务类型 ID |
| `tasklistId` | `=` `!=` `IN` `NOT IN` | 任务分组 ID |
| `storyPoint` | `=` `!=` | Story Point |
| `cf:<fieldId>` (数字) | `=` `!=` `>` `>=` `<` `<=` | 自定义字段（数字类型） |
| `cf:<fieldId>` (文本) | `~` `!~` | 自定义字段（文本类型） |
| `cf:<fieldId>` (日期) | `=` `!=` `>` `>=` `<` `<=` | 自定义字段（日期类型） |
| `cf:<fieldId>` (多选) | `~` `!~` | 自定义字段（多选类型） |
| `cf:<fieldId>` (单选) | `=` `!=` `IN` `NOT IN` | 自定义字段（单选类型） |

## 时间函数

### 基础函数

| 函数 | 说明 |
|------|------|
| `startOf(d)` | 今天开始（00:00:00） |
| `endOf(d)` | 今天结束（23:59:59） |
| `startOf(w)` | 本周开始（周一） |
| `endOf(w)` | 本周结束（周日） |
| `startOf(M)` | 本月开始 |
| `endOf(M)` | 本月结束 |
| `startOf(y)` | 今年开始 |
| `endOf(y)` | 今年结束 |

### 时间偏移（以 `dueDate` 为例，其他时间字段同理）

| 场景 | TQL |
|------|-----|
| 今天 | `dueDate >= startOf(d) AND dueDate <= endOf(d)` |
| 昨天 | `dueDate >= startOf(d, -1d) AND dueDate <= endOf(d, -1d)` |
| 过去3天 | `dueDate >= startOf(d, -3d) AND dueDate <= endOf(d, -1d)` |
| 过去7天 | `dueDate >= startOf(d, -7d) AND dueDate <= endOf(d, -1d)` |
| 过去30天 | `dueDate >= startOf(d, -30d) AND dueDate <= endOf(d, -1d)` |
| 最近7天（含今天） | `dueDate >= startOf(d, -6d) AND dueDate <= endOf(d)` |
| 未来3天 | `dueDate >= startOf(d, 1d) AND dueDate <= endOf(d, 3d)` |
| 未来7天 | `dueDate >= startOf(d, 1d) AND dueDate <= endOf(d, 7d)` |
| 本周 | `dueDate >= startOf(w) AND dueDate <= endOf(w)` |
| 上周 | `dueDate >= startOf(w, -1w) AND dueDate <= endOf(w, -1w)` |
| 本月 | `dueDate >= startOf(M) AND dueDate <= endOf(M)` |
| 上月 | `dueDate >= startOf(M, -1M) AND dueDate <= endOf(M, null, -1M)` |
| 今年 | `dueDate >= startOf(y) AND dueDate <= endOf(y)` |
| 未填写 | `dueDate = null` |
| 已填写 | `dueDate != null` |
| 指定日期范围 | `dueDate >= '2026-03-01T00:00:00+08:00' AND dueDate <= '2026-03-31T23:59:59+08:00'` |

## 运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `=` | 等于 | `isDone = false` |
| `!=` | 不等于 | `isDone != true` |
| `<` | 小于 | `dueDate < startOf(d)` |
| `<=` | 小于等于 | `dueDate <= endOf(w)` |
| `>` | 大于 | `dueDate > endOf(d)` |
| `>=` | 大于等于 | `dueDate >= startOf(w)` |
| `~` | 模糊匹配 | `title ~ '关键词'` |
| `IN` | 包含（多值匹配） | `projectId IN ('projectId1', 'projectId2')` |
| `NOT IN` | 不包含 | `tagId NOT IN ('tagId1', 'tagId2')` |
| `AND` | 与 | `isDone = false AND priority = <priorityValue>` |
| `OR` | 或 | `priority = <priorityValue1> OR priority = <priorityValue2>` |

## 排序

```
ORDER BY dueDate ASC       # 截止时间升序
ORDER BY dueDate DESC      # 截止时间降序
ORDER BY priority ASC      # 按企业配置返回的优先级数值升序
ORDER BY updated DESC      # 最近更新在前
ORDER BY created DESC      # 最新创建在前
ORDER BY accomplished DESC # 最近完成在前
ORDER BY startDate ASC     # 开始时间升序
```

## 注意事项

- **必须使用 `me()`**：查询"我的"任务时，用 `executorId = me()` 而非硬编码用户 ID
- **时区**：TQL 中的时间函数基于用户时区自动处理，无需手动转换
- **字符串值**：字符串值用单引号包裹，如 `projectId = 'xxx'`
- **null 查询**：`field = null` 表示未填写，`field != null` 表示已填写
- **自定义字段**：使用 `cf:<fieldId>` 格式；先从 `project --help` 定位自定义字段查询动作，读取该动作 Help，再按 `projectId` 获取 `fieldId`
- **优先级值**：先从 `project --help` 定位优先级查询动作，读取该动作 Help，并以当前企业返回的名称、值和顺序为准；禁止套用固定数字含义
- **回收站过滤**：查询任务时默认加入 `isArchived = false` 排除回收站/已归档任务；按上述流程
  先构造谓词、再加入默认条件、最后追加排序
