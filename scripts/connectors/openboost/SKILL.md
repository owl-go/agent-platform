---
name: openboost
display_name: OpenBoost 跨境数据
description: 用 OpenBoost 查询 Amazon 商品、销量、关键词、评论与类目市场，TikTok 商品、达人、视频与店铺，专利和社媒数据，以及当前账号套餐与额度。
version: 0.1.0
author: Agent Workspace
---

# OpenBoost 跨境数据

前提：选择已安装且已连接的 OpenBoost Connector，使用 User 身份。未连接时引导到连接器管理页填写官网获得的 Secret Key；密钥由平台加密保存，不进入命令或对话。官方支持密钥认证，本修订使用固定 HTTPS 统一 MCP 的 `secret-key` 请求头。

## 调用顺序

1. 用 `agent-cli invoke --connector <installation-id> --capability openboost_tools --identity user -- tools` 查看当前服务端实际提供且本修订放行的工具。包内 [工具 Schema](tools.json) 是 2026-10-02 的公开目录快照；实际参数以当前服务端为准。
2. 根据需求选择工具，再用 `agent-cli invoke --connector <installation-id> --capability openboost_schema --identity user -- schema call <tool-name>` 查看参数格式、必填项与分页限制。站点编号、地区、类目和统计周期必须按 Schema 和类目查询结果提供。
3. 用 `agent-cli invoke --connector <installation-id> --capability openboost_call_<tool-name> --identity user -- call <tool-name> --json '<JSON 对象>'` 查询。托管 broker 使用 `--json`；本地桥接器另支持 `--stdin`。账号权益使用 `openboost_account_status` capability 与 `account status` 命令。
4. 读取并检查 `ok`、MCP `isError` 和业务结果中的状态，再分析数据。保留站点、统计周期、币种、估算口径、分页范围和来源；销量预估不当作平台实测销量，专利检索结果不当作法律结论。

## 操作范围

该修订固定放行 100 个发现的工具：Amazon 选品、竞品、SKU/销量、关键词/ABA、流量、评论、榜单与市场分布；TikTok 商品、达人、视频、店铺、评论与大盘；专利文本、同族、状态、翻译与图搜；Instagram、YouTube、Reddit 数据；账号、套餐目录、链接与已有订单查询。逐项映射与策略见 [capabilities.json](capabilities.json)。CLI 是仓库维护的官方 MCP 桥接器，不是 PyPI 官方 `openboost-cli`。

查询可能消耗上游套餐额度，社媒实时采集可能等待约 120 秒；先查账号权益，再控制查询量与分页。`account_order_create` 不在白名单，付款与购买在官网完成。新增上游工具需要审核和新修订，工具发现不能自动扩展能力。

## 失败处理

- `authorization_required`：重新连接或在官网检查 Secret Key 与服务权益。握手或目录成功只证明协议可达，不证明账号有权调用业务数据。
- `upstream_unsupported`：当前目录缺少该工具，说明工具名及当前修订，不猜测整个服务不可用。
- `tool_error`：读取脱敏后的业务错误，区分额度、参数、缺少数据或服务权限；空结果不等于零销量。
- `transport_error` / `output_limit`：缩小查询与分页，确认额度和结果后由用户决定是否重试。桥接器不自动重试，避免重复扣减额度。

包内公开 Schema 与协议 fixture 已核对；具体账号的业务数据、权益和扣费只有真实授权调用才能验证。
