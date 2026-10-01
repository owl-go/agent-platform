# 扫描全能王 CLI Connector Package

官方来源：[CLI 文档](https://www.camscanner.com/agent-docs/zh/platforms/for-agents/cli/)。固定 `camscanner-cli@1.1.8`；包使用两个同版 Linux 原生 npm 资产并校验 npm SHA-512，官方 Skill ZIP 固定 SHA-256。`build.py` 不运行 npm 安装脚本。图标来自文档站 favicon，48×48 PNG 的 SHA-256 为 `95ac2cbe24e91e2be243648b857b16c09b91dc941dd17b10c46ac79db8d46fd3`；包和目录显示同一图像。

`capabilities.json` 的 28 个业务命令逐一对照原版 `--help` 与官方 Skill，另有四个诊断。所有处理上传、生成、保存、下载和移动均高风险，要求具体 target 及平台一次性批准；搜索和目录只读。身份是当前 User，原版 headless 协议没有可请求的细粒度 scopes，不填虚构权限。

浏览器连接使用原版 1.1.8 的无界面登录协议。平台回传固定官方授权页，轮询、Token 续期及加密存储由平台负责。执行 wrapper 仅在一次命令的临时 HOME 恢复原版 Linux AES-GCM keychain。原版在到期前 24 小时主动续期；wrapper 先校验真实有效期至少剩余 180 秒，再仅将命令临时 keychain 的缓存到期值延后 24 小时，抑制运行内续期。实际平台到期值不变，命令上限 120 秒；过期或接近过期时先由平台刷新，绝不丢弃运行内生成的轮换 Token。auth、安装、升级和原始任意命令不在 Agent 白名单。远程 MCP 的 OAuth PKCE 与原生 CLI 授权不同，不能混用 Token 或宣称已验证。协议来源及边界见 `docs/technical/connector-platform.md`。

构建（版本和 Runtime RepoDigest 必须与部署配置一致）：

```bash
npm pack @camscanner-cli/linux-x64@1.1.8 --ignore-scripts --pack-destination /tmp
npm pack @camscanner-cli/linux-arm64@1.1.8 --ignore-scripts --pack-destination /tmp
curl -fL https://data.camscanner.com/camscanner-cli/releases/latest/camscanner-skill-latest.zip -o /tmp/camscanner-skill.zip
python3 scripts/connectors/camscanner/build.py --x64-npm /tmp/camscanner-cli-linux-x64-1.1.8.tgz --arm64-npm /tmp/camscanner-cli-linux-arm64-1.1.8.tgz --skill-zip /tmp/camscanner-skill.zip --runtime-image <Registry-RepoDigest> --runtime-version <Node-version> --output <absolute-path>/camscanner-1.1.8.zip
go -C backend run ./cmd/connector-package-validate <absolute-package-path>
```

构建同时输出 `.source.zip`，由平台 ZIPPackageBuilder 生成逐字节相同 bundle。所有代码先经开发分支测试、提交、推送，再集成 `main_temp` 并通过门禁，从该分支发布 API/Web 支持及包。运行 `publish.py --config <deployment-env> --package <ZIP> --source <source-ZIP> --evidence-directory <directory>`：复用已有 exact Conformance，缺少时由 Worker 执行 Linux + runsc 验证，Stage/Publish 后软删除无用户使用的临时 Definition，保留历史证据。

本地检查：`python3 -m unittest discover -s scripts/connectors/camscanner -p 'test_*.py'`、`node --test scripts/connectors/camscanner/*.test.cjs`、目标 Go 测试及 Web typecheck/build。原版 Linux 协议测试用 `CAMSCANNER_TEST_NATIVE=<binary>` 启用，并在隔离 Runtime 内执行；不设置时 Skip，不能记为通过。模拟 HTTPS upstream 拒绝缺少原生协议头或错误 Bearer 的请求，验证真实原版 CLI 登录、轮询、续期和目录调用。

真实账号授权、OCR/转换、云保存/移动与 CDN 下载仍须账号完成浏览器登录后验证。当前 Egress 只放行固定 AI Tools 主机；未知 CDN 不放行。无账号证据时不宣称业务 API 可用，也不保存测试 Token 到真实 Installation。
