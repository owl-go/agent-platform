---
status: accepted
---

# Run 结果与文件 Artifact 分开持久化

Run 的最终文本或 JSON 属于 Run Conversation，并继续保存在 Run 的 `final_result` 中。Artifact 只表示该成功 Run 实际新增或修改的文件；普通文本或 JSON 回复不会生成名为 `Final result` 的伪文件。

本决策取代 ADR 0018 中“Artifact 冻结最终结果和变更文件”的表述。已有 `kind=result` 记录为兼容历史数据保留在数据库中，但不再由新 Run 创建，也不通过 Artifact 列表展示；历史对话结果仍从 Run 记录正常展示。
