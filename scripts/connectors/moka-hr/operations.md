# 审阅查询表

所有能力身份为平台 `user`，实际请求使用企业 API Key，scopes 为空，风险为 `low`，只访问 `api.mokahr.com`。`!` 表示必填，其他字段可选。通过 `schema <command>` 查看类型。

| 操作 | command | capability ID | 方法和固定路径 | 参数 |
|---|---|---|---|---|
| 官网职位列表 | jobs list | moka_jobs_list | GET /v1/jobs/{orgId} | mode!（social/campus）、keyword、limit（1–100）、offset |
| 职位详情 | jobs get | moka_jobs_get | GET /v1/jobs/{orgId}/{jobId} | jobId! |
| 职位自定义字段 | jobs fields | moka_jobs_fields | 同职位详情，提取 customFields | jobId! |
| eHR 阶段候选人 | candidates search | moka_candidates_search | GET /v1/data/ehrApplications | stage!（offer/pending_checkin/all）、email、phone、movedAtStartTime、movedAtEndTime、order（ASC/DESC）、limit（1–20）、next |
| 候选人申请详情 | candidates get | moka_candidates_get | GET /v1/data/ehrApplications | applicationId!（最多 20 个正整数的逗号串） |
| 当前申请阶段 | candidates stage | moka_candidates_stage | 同申请详情，提取阶段 | applicationId! |
| 候选人相关申请 | applications list | moka_applications_list | POST /candidate/v1/getApplicationStates（只读查询） | candidateId!（安全正整数） |
| 招聘流程 | pipelines list | moka_pipelines_list | GET /v2/pipelines/getPipelinesList | 无 |
| 阶段清单 | stages list | moka_stages_list | GET /v2/stage/getStagesList | 无 |
| 部门树 | departments list | moka_departments_list | GET /v1/departments | 无 |
| Offer 字段定义 | offers fields | moka_offers_fields | GET /v1/offers/custom_fields | 无 |
| 人才库清单 | pools list | moka_pools_list | GET /v1/talentPool/list | 无 |
| 人才库候选人 | pools candidates | moka_pools_candidates | GET /v1/talentPool/candidates | talentPoolIds!（1–20 个正整数数组）、archivedAtStart!、archivedAtEnd! |
| 面试列表 | interviews list | moka_interviews_list | GET /v1/interviews | startDate+endDate 或 createStartDate+createEndDate 至少一对，日期范围最多 31 天；hireMode（1/2） |

前缀统一为 `/api-platform`；只连接中国版生产环境。日期为 `YYYY-MM-DD` 或带时区的 ISO8601。eHR 接口的 `all` 是 Offer 和待入职阶段，不代表所有阶段候选人。职位列表是招聘官网列表，不保证涵盖未发布职位。人才库参数 `talentPoolIds` 发送为 JSON 数组字符串。面试列表是查询已有安排，没有写入安排能力。

本地：`moka jobs list --json '{"mode":"social","limit":20}'`。
托管：`agent-cli --connector <Installation ID> --capability moka_interviews_list --identity user -- interviews list --json '{"startDate":"2026-10-02","endDate":"2026-10-09"}'`。
