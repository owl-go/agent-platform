# 好会计打包工具部署 — 2026-10-02

状态：打包工具已部署并通过服务器验证；业务连接器尚未发布或安装。

## 发布标识

- 部署源：`main_temp` 集成提交 `4f785bac0b05b83f67f25b9781f5a5ab75fff2ed`；功能提交 `73cd822`，合并无冲突。
- SSH 主机：`agent-platform`，Linux amd64，Python 3.14.4、Go 1.26.0。
- 激活版本：`haokuaiji-tooling-4f785bac0b05`。
- 版本目录：`/opt/agent-platform/connectors/haokuaiji/releases/haokuaiji-tooling-4f785bac0b05`。
- 原子入口：`/opt/agent-platform/connectors/haokuaiji/current/scripts/connectors/haokuaiji/build.py`。
- 工具 tar.gz SHA-256：`3dde5f86ffd509bc2af1f9a43dfc727f37add59a10b7d1df6101cacfb20be484`。

## 已执行的检查

发布工作区基于最新 `origin/main_temp`，合入功能分支后执行 9 项 Python 测试、`make test`、`make build` 和 `git diff --check`，均成功；随后将集成提交推送到 `main_temp`。

工具 artifact 来自该集成提交的 tracked 文件，包含 `scripts/connectors/haokuaiji`、`backend/cmd/connector-package-validate`、Go `connectorpackage` 与 `icon` 包、`go.mod` 和 `go.sum`，合计 33 个源文件。没有上传环境文件、用户配置、凭证或开发工作区的未跟踪文件。服务器先校验 tar.gz SHA-256，再逐项校验源文件 SHA-256。

服务器实际执行以下检查，均成功：

- `python3 -m unittest discover -s scripts/connectors/haokuaiji -p 'test_*.py' -v`：9 项测试通过。
- `go -C backend test ./internal/connectorpackage/...`：通过。
- `python3 scripts/connectors/haokuaiji/build.py --help`：入口可运行。
- 用临时 synthetic 配置实际执行 `build.py --config ... --output ...`，生成测试 ZIP，并调用同版本 `connectorpackage.Parse` 验证。逐项读取 ZIP，确认其中没有 fixture 凭证字节。测试 JSON/ZIP 随临时目录自动删除。

验证后将工具 source 移入独立版本目录，原子替换好会计工具的 `current` 符号链接，删除 staging 上传 archive 和目录。该操作未修改平台 `/opt/agent-platform/src` 或 Web 指针，没有服务替换或数据库 Migration。

从服务器访问公网 `https://47-237-108-63.sslip.io/api/healthz` 返回 `{"status":"ok"}`，`/api/readyz` 返回 `{"status":"ready"}`。

## 验证边界

本记录证明打包工具和指定 Go 解析器在服务器上可运行，不证明真实好会计连接器可用。没有目标企业的官方 MCP 导出，因此没有交付真实 Connector ZIP，没有调用畅捷通 MCP，没有创建 Revision、Publication、Installation 或 Authorization，也没有声明任一财税业务操作通过验收。

host Linux 测试不是 Linux + runsc 的 Runtime Conformance。未执行隔离 MCP 握手、真实账号调用、品牌图标目录投影或未授权卡片连接验收；未修改 Web 或 Runtime 镜像，因此未执行对应构建／镜像门禁。后续实际发布仍需官方企业配置、工具目录审阅、连接入口与目录验收。
