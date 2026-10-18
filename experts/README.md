# 专家与专家团目录

每个 `experts/<key>/` 子目录放一个资源包，入口为 `.plugin/plugin.json`。示例见 [旅行专家清单](travel-planning/.plugin/plugin.json) 和 [Markdown 指引](travel-planning/agents/expert.md)。新增子目录自动发现，无需更新中心清单。

清单声明 `schema_version: 1`、稳定 `id`、语义 `version` 和 `kind: expert` 或 `expert_team`。专家含名称、展示简介和 `guidance_file`；专家团含明确的 `lead_member_id` 及 2–10 位独立成员。显示顺序不决定执行顺序。成员手工责任 labels 可保留，不生成标签或分类。

随仓 8 位专家当前使用 `1.3.0`，每位包含中英双语名称、能力简介、三条逐条对应的常用任务和完整 Markdown 指引。`translations.zh-CN` 与默认中文字段一致，`translations.en` 保存英文文案。创建专家的 Skill 同样生成这份 profile；旧模板中的能力、流程、交付标准和注意事项正文合并到指引，不再作为独立 Expert 写入字段。

可引用目录 Skill 的稳定 `skill_keys`，或在 `skills/<key>/` 携带包内技能。`avatars/` 可携带有效 PNG、JPEG、GIF、WebP；每个 profile 最多三条 `starter_prompts`。外部连接器只声明来源、类型与版本，不携带账号授权。详见[资源包技术契约](../docs/technical/portable-experts.md)。

更改内容须增加版本，运行 `make resources-check`。初始化和升级复用稳定资源身份；目录原件属于 Bootstrap Administrator 且不可编辑或删除。同名管理员自建资源受保护，移除目录不会删除已初始化资源。发布仍须经过 `main_temp` 及适用验收；目录校验不证明外部授权或 Runtime Conformance。
