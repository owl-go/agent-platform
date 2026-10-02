# 中文登录主题发布验证（2026-10-02）

本次将 Keycloak 实际登录表单改为工作区的浅灰背景、白色圆角卡片、系统字体和设计 Token，并默认使用简体中文。继承 Keycloak 原生模板，覆盖样式和文案；保留语言切换、显示密码、保持登录及找回密码。Web 登录请求传递当前语言，无偏好时使用 `zh-Hans`。

## 发布范围

- 功能提交：`dbdd349`，分支 `codex/login-style-refresh`。
- 从最新 `origin/main_temp` (`679ccf5`) 集成，发布提交：`da5b8c5f86fc`；发布前已推送 `main_temp`。
- Web：`/opt/agent-platform/web/releases/login-theme-da5b8c5f86fc`。
- 主题：`/opt/agent-platform/identity-themes/releases/login-theme-da5b8c5f86fc`，通过 `current` 只读挂载到身份容器 `/opt/keycloak/themes`。
- 当前服务源码仍为 `src.release-tianyancha-region-20261002T1335`；针对旧 Compose 增加 `/opt/agent-platform/config/identity-theme.compose.json` 挂载覆盖文件，使用原项目和三个原配置文件，仅重新创建 `identity`。
- Keycloak 镜像未变：`quay.io/keycloak/keycloak@sha256:f1f1f01e472c8a78df40d8f2a49a925274eda4d3d80d5f6edbb5c880ee3c01c6`，版本 26.7.1。
- 管理 API 只更新并回读确认 `loginTheme=agent-workspace`、`internationalizationEnabled=true`、`supportedLocales=[zh-Hans,en]`、`defaultLocale=zh-Hans`。没有重导入现有 Realm，也没有修改用户、客户端、会话期限或认证策略。

## 实际执行的检查

- `python3 scripts/configure-identity-theme_test.py`：2 项通过；覆盖受限 HTTPS 地址、只备份/修改四个外观字段、备份权限及回退。
- `pnpm --dir frontend exec vitest run src/auth/oidc.test.ts src/auth/session.test.ts`：21 项通过，包含默认中文、显式中英文偏好及创建客户端后切换语言。
- 功能分支完整前端测试：46 个文件、425 项通过。
- 集成后的 `pnpm --dir frontend test`：46 个文件、474 项通过。
- 功能及集成分支的 Web 类型检查、E2E TypeScript 检查通过。功能分支 `make web-build` 通过；集成源码使用生产 OIDC 配置执行 `scripts/deploy-web.sh`，类型检查、生产构建及上传激活通过。构建仍有既有大 Chunk 提示。
- `node --check scripts/e2e/run-local.mjs`、`git diff --check` 通过。
- 构建主题、校验全部 Token 引用及本地/上传文件的 SHA-256 Manifest 通过。
- 使用相同镜像启动隔离的本地 Keycloak Fixture，实际浏览器检查桌面和 375px 手机布局、显示/隐藏密码、中文无效凭证提示、语言切换、中文找回密码页、中文账户信息更新页及强制更新密码页。更新密码页显示两个密码输入框；没有实际发送找回邮件。
- 线上新浏览器从产品根路径自动进入原生登录表单，`ui_locales=zh-Hans`、HTML `lang=zh-Hans`，标题“登录工作空间”，没有中间登录按钮。
- 线上背景计算值为 `rgb(245, 247, 250)`，背景图片为 `none`。1440×900 桌面及 375×812 手机截图已检查；手机卡片宽 343px、无横向溢出；Tab 从用户名进入密码。
- 线上语言菜单切换为英文有效，再从产品首页进入会恢复中文，覆盖之前的语言 Cookie。
- 线上 CSS 返回 200 且与发布文件逐字节一致；健康接口 `status=ok`，就绪接口 `status=ready`，身份容器 `running`、重启次数 0，OIDC Discovery 正常。

## 校验与回退材料

- Web `index.html` 的本地、服务器及公开响应 SHA-256 一致：`7d7523401a1ec4d203d10db4d2fea4989c75369afbc0dfbef2bca9f74b17d710`。
- 公开 `design-tokens.css` SHA-256：`f4ebceb89208050081985ea71209fab04752098f0010d021d95e054a591674bb`。
- 公开 `workspace.css` SHA-256：`844274c6816286735ce00faf4ad9b0f1754c0b545dd3411d50d0b9afc06d5805`。
- 发布前备份位于 `/opt/agent-platform/backups/pre-login-theme-da5b8c5f86fc`：身份数据库 Dump、宿主 env、四个外观字段及前一源码/Web 路径。`pg_restore -l` 校验通过；备份目录 0700、外观备份和 env 0600。
- 回退时恢复原宿主 env，使用记录的原源码 Compose 文件重新创建身份容器（不带新增主题覆盖文件），待 Discovery 恢复后执行 `configure-identity-theme.py restore` 恢复外观；Web 可使用 `scripts/deploy-web.sh activate` 激活记录的上一版本。
- 截图留在忽略的 `output/playwright`，未提交认证 URL、Token、登录凭证或浏览器状态。

## 证据边界

未使用生产账号密码登录，未验证生产邮件投递，也未重新执行完整产品 E2E。此前完整 E2E 的 MinIO 镜像拉取受阻；此次登录主题通过独立 Keycloak Fixture 和线上空白登录表单检查，不等同于完整产品 E2E 验收。此任务未修改后端、Runtime、Sandbox 或存储实现，未运行对应 Linux/Production Conformance 门禁。
