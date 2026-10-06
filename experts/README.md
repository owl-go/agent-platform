# 专家目录

每个专家放在 `<name>/expert.json`，参考 [travel-planning/expert.json](travel-planning/expert.json)。

填写稳定 `key`、`version`、名称、图标、简介、核心能力、工作流程、输出标准和可选注意事项。`skill_keys` 引用 Skill 的稳定 Key，加载时自动解析成本安装的 ID；也可引用三个 `system.create_*` Skill。定义不包含模型设置或用户凭据。

更新内容时增加 `version`，运行 `make resources-check` 后提交并部署。首次安装自动创建，重复启动复用已有专家和绑定；同名管理员自建资源保留，目录原版通过发布更新。
