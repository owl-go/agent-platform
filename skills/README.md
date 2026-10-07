# Skill 目录

每个 Skill 放在独立子目录：`SKILL.md`、可选脚本/参考资料，以及 `resource.json`：

```json
{"key":"local.skill.example","version":"1.0.0","icon":"sparkles"}
```

名称读取 `SKILL.md` frontmatter 的 `display_name`。`key` 保持稳定，更新内容时增加 `version`；`system.*` 留给三个内置创建 Skill。参考 [travel-planning](travel-planning/) 的完整结构。

`make resources-check` 检查内容，安装与部署自动加载；重复启动复用已有 Skill。目录资源作为不可修改的 Platform Skill 分发，个人定制可另建资源。
