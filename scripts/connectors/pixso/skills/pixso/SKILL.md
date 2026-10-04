---
name: pixso
display_name: Pixso 云端设计
description: 使用 Pixso Remote MCP 读取云端设计、导出素材、生成代码，或按用户明确范围编辑设计。用户提供 Pixso 图层链接、请求设计还原或设计检查时使用。
version: 1.0.0
author: Agent Workspace
---

# Pixso Remote MCP

先确认 Installation 已启用且用户已在平台点击“连接”完成 Pixso 浏览器 OAuth。执行身份是该用户授权的 Pixso 账号。此包固定公有云 `https://pixso.net/mcp`；私有部署需单独审核其真实服务地址。

## 调用顺序

1. 使用用户提供的图层链接定位目标，保留 `item-id`。文件分享链接和 `page-id` 不能替代图层 ID；缺少目标时先让用户提供图层链接。Remote MCP 无需打开桌面客户端。
2. 读取当前授权服务器实际提供的工具及输入 Schema，再决定参数。下面的工具名称来自官方 Remote MCP 文档核对，尚无本修订的真实账号工具发现或业务调用验收；以当前服务返回为准，缺失时报告而不编造替代调用。
3. 分析结构用 `get_node_dsl`，先取轻量内容，需要时只补取目标子树。查找图层用 `query_nodes` 或 `get_top_level_frames`。代码生成用 `design_to_code`，明确框架和项目约束；优化提示由服务给出。组件映射用 `read_component_config_data`，素材导出用 `get_export_image`，视觉检查用 `take_screenshot`。
4. 设计编辑先用 `fetch_context` 了解资源，再按需读取 `read_components`、`read_styles`、`read_variables`。评论阅读用 `read_comments`，有分页时继续读取再声称完整。规范查询可用 `load_guidelines`；风格查询先 `list_style_tags` 再 `get_style_guide`。布局与属性审计用 `check_layout`、`query_all_unique_props`。
5. 用户明确要求编辑时，先核对文件、图层、变更内容和允许范围，再使用 `write_styles`、`write_variables`、`apply_design`、`replace_props` 或 `code_to_design`。`eval_script` 可产生读写副作用，按写操作处理。遵守平台策略和所要求的一次性批准；Skill 不构成批准。编辑后读取目标并用截图检查；结果不明时先查现状，避免重复写入。脚本失败可能已经产生部分变更。

Local MCP 专用工具（例如 `get_screenshot`、`create_instance` 和 `set_fill_style`）不作为本包的 Remote MCP 能力。此修订的网络白名单仅含 `pixso.net`；素材结果若需要访问其他域名，应报告当前 Egress 限制，不能擅自扩大白名单。

## 失败处理

授权缺失或过期时使用平台连接器设置的连接／刷新入口；不要要求用户把 Token 粘贴进对话或修改此包。工具缺失时区分上游工具清单、当前授权、文件权限与 Runtime 验证缺口。403 先核对该 Pixso 账号是否有目标文件权限。超时、网络失败和写操作结果未知时先查询结果再决定重试。凭证、授权码与完整授权 URL 不进入工作区、业务结果或持久日志。

官方资料：[概述](https://pixso.net/developer/en/mcp/introduction.html)、[Remote MCP](https://pixso.net/developer/en/mcp/remote-mcp.html)、[工具与模式范围](https://pixso.net/developer/en/mcp/tools.html)。
