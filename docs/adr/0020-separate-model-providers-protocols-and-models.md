---
status: accepted
---

# 分离模型供应商、API 协议与模型

Agent Workspace 以用户私有的 Model Provider Connection、Model API Protocol 和 Provider Model 取代将 Endpoint、模型与 Secret 混在一起的 Model Profile。供应商品牌不能证明 Runtime 兼容性，同一供应商也可能同时暴露 Responses、Chat Completions 或 Anthropic Messages；因此 Session Response 和 Workflow Run 分别冻结实际连接版本、协议、Endpoint、模型与 Runtime，而 API Key 只通过受保护的版本化凭证引用使用。平台不建设统一协议转换代理，未验证组合允许选择但明确提示，实际不兼容则显式失败。
