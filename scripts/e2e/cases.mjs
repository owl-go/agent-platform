// Each row describes an assertion group, not full completion of every related manual case.
const rows = `
001|ACC-012,ACC-014|浏览器登录和刷新恢复|使用普通用户经 Keycloak 登录；打开 Sessions 并刷新|新建会话按钮可见；URL 无授权码参数
002|ACC-013|匿名访问拒绝|不携带 Token 请求 Sessions|HTTP 401
003|ACC-013|无效 Token 拒绝|携带非法 Bearer Token 请求 Sessions|HTTP 401
004|ACC-009|账号管理权限|普通用户请求账号列表和创建账号|均为 HTTP 403
005|ACC-003|重复用户名冲突|管理员重复创建已存在用户名|返回可识别的冲突错误；按当前领域映射为 HTTP 412
006|SES-008,SES-009,SES-010,SES-012|会话完整管理|创建会话；改名；归档；查归档列表；恢复；删除；查活动列表|状态逐步持久化；删除后活动列表不再存在
007|SES-008|会话版本冲突|创建会话；更新一次；以旧版本再次更新|旧版本请求 HTTP 412
008|ACC-010,SEC-001|会话所有者隔离|普通用户创建会话；管理员列举、读取消息和删除该会话|列表不含其他用户会话；读取和删除均 HTTP 404
009|EXP-001,EXP-015|结构化专家管理|创建完整专家；改名和指导内容；读取；删除；再次读取|complete=true；字段持久化；删除后 HTTP 404
010|ACC-010,SEC-001|专家所有者隔离|普通用户创建专家；管理员读取和删除|均为 HTTP 404
011|EXP-002|专家必填名称|提交空名称和其他有效结构化字段|HTTP 422
012|TEAM-001,TEAM-006|团队成员顺序持久化|创建两位专家；以作者、审核者顺序建立团队；读回；删除团队|读回顺序与专家身份一致；删除成功
013|WF-001,WF-003,WF-020|工作流管理|创建最小工作流；改名；读回；删除；查询活动列表|配置持久化；删除后不在活动列表
014|SEC-001|工作流所有者隔离|普通用户创建工作流；管理员读取详情和 Workspace|均为 HTTP 404
015|WF-017|Secret 环境变量只写|创建含随机 Secret 的工作流；读取详情|响应不含 Secret 原文；configured=true
016|WSP-001,WSP-004|空目录及路径越界|创建工作流；列目录；读取 ../../etc/passwd|目录为空；越界请求 HTTP 422
017|SET-017,SET-019|个人设置持久化与版本冲突|保存自定义人格指导；重新读取；旧版本再次保存|指导内容一致；旧版本 HTTP 412
018|SET-003|全局模型目录权限|普通用户尝试创建模型连接|HTTP 403
019|SET-015|Runtime 可用性如实呈现|读取五种 Runtime 状态|全部 unavailable；native_resume=false
020|FIL-015,FIL-019,FIL-021|附件上传和下载|上传文本；核对 SHA-256；认证下载并逐字节比较；另一用户下载|哈希及内容一致；跨所有者下载 HTTP 404
021|CRD-001,CRD-031|积分读取与管理权限|读余额和账本；普通用户尝试生成兑换码|余额非负；账本可读；生成兑换码 HTTP 403
022|CRD-025,CRD-027|兑换仅生效一次|管理员生成 1234 百分积分兑换码；用户兑换；重复兑换；再查余额|首次增加 1234；再次 HTTP 422；余额不重复增加
023|CLI-001,CLI-008|CLI 管理权限|普通用户创建 CLI Definition 和读取健康页数据|均为 HTTP 403
024|APR-001|审批初始状态|新账号读取待处理命令审批列表|列表为空；不代表审批执行流程已验收
025|UI-001|工作流页面加载|浏览器打开工作流目录|标题与新建会话按钮可见；没有 pageerror
026|UI-001,UI-003|专家页面加载|浏览器打开专家目录|专家标签页与新建会话按钮可见；没有 pageerror
027|UI-001,UI-003|资源页面加载|浏览器打开技能连接器目录|页面标题可见；没有 pageerror
028|UI-001|设置页面加载|浏览器打开个人设置|设置标题可见；没有 pageerror
029|EXP-001|浏览器创建专家|输入名称、简介、核心能力、流程和输出标准；保存；刷新目录|返回专家目录；新专家可见
030|UI-006|移动端会话布局|视口设为 390×844；打开 Sessions；截屏|移动头部可见；页面无水平溢出
031|SES-003,RUN-002|真实 Runtime 与流式终态|需要受验证 Runtime 镜像、模型凭据和 Linux Worker|应验证真实模型增量输出及唯一终态；本轮环境阻塞
032|SCH-001,SCH-011,WF-007|Worker 定时与串行|需要 Worker 和可控调度时钟|应验证定时触发、重复扫描和串行锁；本轮环境阻塞
033|CLI-010,APR-001,APR-017|真实 CLI 授权和审批恢复|需要真实授权、CLI Bundle 和 Worker|应验证授权、命令审批和恢复；本轮环境阻塞
034|SEC-008,SEC-009,SEC-010,OPS-009|Linux Sandbox 验收|需要 Linux + runsc 和 RepoDigest 验收环境|应先运行 Production Preflight 再做隔离验收；本轮环境阻塞
035|WF-001|浏览器创建工作流|打开新建弹窗；填名称和目标；提交；刷新详情|详情 URL 含工作流 ID；名称保存成功
036|SES-001|浏览器立即新建会话|读取会话数量；点击新建会话；检查列表和编辑器|未发送消息前已增加一条会话；编辑器可见
037|SKL-002,SKL-013,SKL-017|ZIP Skill 管理|上传有效 ZIP；读取 SKILL.md；跨用户访问；获取删除影响 Token 并删除|摘要格式有效；文档正确；越权及删除后读取均 HTTP 404
038|SKL-007|ZIP 路径穿越拒绝|上传包含 ../escape.txt 的 ZIP|HTTP 422
039|MCP-001,MCP-011|MCP 合同请求管理|以 mcp_connector 字段创建 HTTPS 配置；查列表；取得影响 Token；删除|配置未测试；列表包含配置；删除成功；未连接外部 MCP
040|SEL-022|会话选择隔离|创建会话 A/B 和专家；仅 A 选择专家；读取 B 的选择|A 为所选专家；B 无专家
041|CRD-029,CRD-027|作废兑换码不可兑换|生成兑换码；管理员作废；用户兑换；重新查余额|兑换 HTTP 422；余额不变
042|OPS-001|开发代理路径回归|经 Web 代理请求 healthz、readyz 和已认证 me|三个接口均 HTTP 200
043|MCP-001,EXP-014|浏览器新增 MCP|资源页切换连接器；新增 MCP；填名称和 HTTPS URL；保存|创建请求 HTTP 200；目录可见新增 MCP
044|TEAM-001,TEAM-006|浏览器重新保存团队|建立两个专家和有序团队；打开团队编辑页；不变更成员直接保存|PATCH HTTP 200；返回专家团队目录
`.trim().split('\n');
export const cases = rows.map(row => {
  const [number, refs, title, steps, expected] = row.split('|');
  const id = `E2E-${number}`;
  const blocked = Number(number) >= 31 && Number(number) <= 34;
  const browser = [1,25,26,27,28,29,30,35,36,43,44].includes(Number(number));
  return { id, refs: refs.split(','), title, steps, expected, type: blocked ? '环境门禁' : browser ? '浏览器 E2E' : 'API 集成 E2E', precondition: blocked ? 'Linux + runsc Worker、受验证镜像和对应外部凭据' : '独立 PostgreSQL、Keycloak、MinIO 与当前 API/Web；管理员及普通用户均经真实 OIDC 登录', priority: [5,7,11,25,26,27,28,30].includes(Number(number)) ? 'P1' : 'P0' };
});
