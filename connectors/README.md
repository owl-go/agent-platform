# 连接器目录

每个连接器的发布内容放在 `<source>/package/`：`connector-meta.json`、`icon.svg`、`mcp.json` 或 `cli.json`、`skills/<name>/SKILL.md`。CLI 还需 `cli-bundle.tgz`。没有构建工具的连接器也可以直接将这些文件放在 `<source>/`。

`source` 必须与目录名一致，修改发布内容时增加包版本。运行 `make resources-check`，提交并集成到 `main_temp` 后部署，平台自动导入定义。

`scripts/connectors/` 保留原有构建、测试、Conformance 和发布工具，不参与自动扫描。这里只维护正式发布内容，构建产物确认后才更新 `package/`。不要把测试包或第二份同 source 定义放入本目录。

相同 source/version/摘要复用已有修订；重复启动不会生成副本。已有管理员发布记录和 disabled 状态保留，版本内容冲突不覆盖。自动导入不携带用户授权；新环境 CLI 需完成其 bundle 与 Runtime Digest 的 Conformance 后才能开放。

详见[目录加载与去重规则](../docs/technical/default-resources.md)。
