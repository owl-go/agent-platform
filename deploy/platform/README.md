# 一键部署

用于已有的单服务器安装。发布工作站需要 Git、Go、pnpm、Make、Python 3.9+、SSH 和 rsync；服务器已有 Docker Compose v2 和可用服务。

## 第一次

```bash
make deploy-setup
```

只填两个值：

| 项目 | 示例 |
| --- | --- |
| 服务器 | `user@server`，或已配置的 SSH 别名 |
| 安装目录 | `/srv/agent-workspace`，填写已有安装的实际路径 |

配置保存在工作站的 `~/.config/agent-workspace/deploy.json`（设置 `XDG_CONFIG_HOME` 时使用该目录），不进入 Git。账号、密码、密钥和 OIDC 参数复用服务器的配置，无需重复填写或复制到本地。

## 以后发布

```bash
make deploy
```

脚本自动完成：读取 `origin/main_temp` → 执行适用检查 → 备份和校验 → 更新应用 → 验证 HTTPS、API、Worker 与 OIDC。

只改前端就只发前端，只改后端就重建 API / Worker。前端只构建一次；应用无改动则检查健康后结束。第一次接管尚无版本记录的安装会重建后端，避免把旧 Worker 当成最新版本。

有任务运行时会停止发布，稍后重试同一个命令。部署失败时，Migration 未变化则自动恢复原应用；新 Migration 已应用时保留备份并停止 Worker，按[运维参考](operations.md#发布失败时)恢复。

想先查看目标与更新范围，运行 `make deploy-check`。更换服务器，重新运行 `make deploy-setup`。

## 首次安装与特殊升级

- 空服务器：[首次安装说明](installation.md)。当前入口不自动安装 Docker、gVisor 或供应身份系统。
- Compose、Runtime、身份主题、Sandbox 变更：[运维参考](operations.md#基础设施与-runtime-升级)。日常入口会明确提示，并保留现有配置与镜像。
- 扫码注册、对象存储与身份配置：[运维参考](operations.md)。这些配置不需要每次部署重复操作。
