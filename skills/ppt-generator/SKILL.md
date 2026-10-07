---

name: "ppt-generator"
display_name: "PPT 一键生成大师"
display_name_en: "PPT Generation Suite"
description: "PPT 生成套件，覆盖 AI 配图演示、标准 PPTX（11种版式）、解决方案框架 PPT 与网页演示 PPT。当用户制作演示文稿、幻灯片或解决方案 PPT 时使用，按产出形态路由。"
description_zh: "PPT 生成套件，覆盖 AI 配图演示、标准 PPTX（11种版式）、解决方案框架 PPT 与网页演示 PPT"
description_en: "PPT generation suite with AI-illustrated decks, standard PPTX layouts and solution presentations."
category: "design"
version: "3.0.0"
author: "Azir"
agent_created: "true"
merged_from: "4 skills"

---

# ppt-generator

PPT 生成能力，4 个技能融合为单一入口。

## 何时使用

当用户需求属于「PPT 生成」范畴时使用。

## 模块

| 能力 | 模块 | 说明 |
|---|---|---|
| AI配图演示 | `deck-generator` | 生成带AI图片的专业演示，风格预设 |
| 标准PPTX | `pptx-generator` | 11种版式5套配色，JSON驱动生成可编辑PPTX |
| 解决方案PPT | `solution-ppt-generator-v3` | 按标准解决方案框架生成面客PPT |
| 网页PPT | `guizang-ppt-skill` | 横向翻页单HTML网页PPT，WebGL背景 |

## 渐进式加载

1. 执行某模块：读 `references/<模块名>.md`（完整调用细节）
2. 脚本在 `scripts/`（文件以 <模块名>__ 为前缀）（相对路径按模块目录执行）
3. 模板/配置：`assets/`（文件以 <模块名>__ 为前缀）；评测用例：`evaluation/`（文件以 <模块名>__ 为前缀）
4. 查找模块：`grep -l "关键词" references/*.md`

## 触发词

PPT、演示文稿、幻灯片、解决方案PPT、deck、slides、发布会