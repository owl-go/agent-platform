---
status: accepted
---

# 扫码注册仍通过 Keycloak 完成产品登录

微信公众号扫码关注和飞书企业扫码首次使用时需要创建普通 User，扩展 ADR-0016 中仅由 Administrator 建号的限制。供应商协议通过 Account 的窄端口和 Go OIDC Broker 转换为一分钟的身份断言，Keycloak 保存联邦身份、签发产品会话并继续管理禁用与密码账号；Broker 不签发产品 Access Token，不保存供应商用户 Token，也不以邮箱或显示名称自动关联已有账号。

Registration Method 与应用共同定义外部身份命名空间，短期 Attempt 绑定浏览器、供应商确认、OIDC State/Nonce 和 S256 PKCE；数据库 CAS 保证只兑换一次。微信原先只接受安全模式的加密关注/扫码事件，这一确认方式已由 [ADR-0044](0044-wechat-registration-with-message-code.md) 取代。飞书企业身份由应用凭证查询并与 User 信息匹配，不能由浏览器指定。

微信公众号无法保证提供邮箱，因此部署配置使 Keycloak email 可选；产品 Application 仍要求 Administrator 手动创建的密码账号具有有效邮箱。部署还预声明两个 admin-only 身份标记以拒绝账号碰撞并支持幂等联邦身份创建。产品服务保留 manage-users/view-users/manage-identity-providers 的窄权限，不获得 manage-realm。配置同步失败保留关闭的 pending 版本，管理员刷新后重试。
