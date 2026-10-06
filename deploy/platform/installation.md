# 首次安装

在项目目录执行：

```bash
make install
```

按提示填写 **服务器、平台域名、管理员邮箱**。脚本自动安装 Docker / gVisor、生成密钥和配置、创建管理员、启动服务并检查入口；成功后保存部署地址，以后更新运行 `make deploy`。

准备一台 Ubuntu 22.04 / 24.04 amd64 专用空服务器，使用 root SSH 密钥登录；域名解析到该服务器，并在云安全组开放 TCP 80 / 443。服务器需有 Python 3、至少 15 GiB 可用构建空间，建议规划 8 vCPU、16 GiB RAM、100 GiB SSD。执行命令的电脑需要项目开发工具（Git、Go 1.25、Node.js 24、pnpm 11、Make、Python 3.9+、SSH）。

安装成功后访问提示中的 HTTPS 地址。账号为 `platform-admin`，初始密码保存在服务器的 `/srv/agent-workspace/config/admin-access.txt`，仅 root 可读；登录后修改密码。密码和密钥不会复制到电脑或打印在安装输出中。

安装中断时，修正提示的问题并重跑相同命令，会继续原安装版本并保留已经生成的密钥。已有安装会提示使用 `make deploy`，不会覆盖数据。

登录后在管理后台配置模型和需要开放的注册方式。执行引擎与 CLI Builder 保持关闭，需按[Runtime 验收](../../docs/technical/production-conformance.md)启用经过验证的镜像；安装成功表示登录与服务检查通过。

需要指定参数或只检查服务器：

```bash
scripts/install-platform.sh --host root@server --domain workspace.example.com --email admin@example.com
# 在上述命令后加 --check，只检查，不安装；--root 可指定安装目录。
```

自定义外部服务等高级配置见[手工安装参考](installation-reference.md)。当前脚本已通过配置、重试和启动顺序测试，全新 Linux 服务器端到端安装尚待验收。
