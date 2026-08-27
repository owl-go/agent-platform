---
status: accepted
---

# 保持 Keycloak 为账号与密码权威

Agent Workspace 保留 OIDC Authorization Code + PKCE，并由唯一 bootstrap Administrator 通过 Keycloak Admin API 创建、禁用、启用和重置普通 User；应用数据库只保存身份投影，不保存登录密码。该选择避免自行实现认证和密码生命周期，同时明确 Administrator 只管理账号元数据、不能读取 User 私有内容；禁用账号还必须停止其定时任务、Workflow API 调用和正在运行的工作。
