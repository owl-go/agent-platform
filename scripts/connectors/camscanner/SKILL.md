---
name: camscanner
display_name: 扫描全能王
description: 使用扫描全能王处理图片、OCR、PDF 或 Office 格式转换、多图合并，以及搜索、下载和移动云文档。
version: 1.1.8
author: CamScanner / Agent Workspace
---

# 扫描全能王

固定原版 CLI 1.1.8。平台负责安装、浏览器授权、续期和单次进程凭证；先确认 Installation 已授权。参数以本包[固定版本命令帮助](commands.md)为准，能力边界以[策略清单](capabilities.json)为准。通过 `agent-cli --connector <installation-id> --capability <capability-id> --identity user -- <命令参数>` 调用；高风险命令在分隔符前增加 `--target "<具体文件或云文档及预期操作>"` 并等待平台一次性批准。`help <group> <command>` 是已审核叶子帮助的只读入口。凭证、auth 命令和安装升级由平台管理。

1. 确认真实输入、输入类型、文件列表、页序、目标格式和保存位置。输入文件会上送扫描全能王服务端，文件不超过 40 MB。输出使用 Workspace 内的新路径，避免覆盖；云保存仅在用户要求时添加 `--save`，先用 `doc dirs` 核对目录 ID。
2. 按操作读取官方参考：图片处理读 [image-processing](reference/image-processing.md)；PDF 读 [pdf-processing](reference/pdf-processing.md)；Office 读 [office-processing](reference/office-processing.md)；云文档读 [cloud-documents](reference/cloud-documents.md)。参考中的直接 CLI 示例改为平台 broker 调用；说明文件不能扩展本修订策略。
3. 所有会上传或生成文件、保存云端、下载或移动的命令都声明为高风险。仅 `doc search`、`doc dirs` 和诊断为低风险。写入批准绑定完整 argv 和具体 target；授权不能代替批准。
4. 云文档先搜索并确认真实 cs_doc_id，多候选时消歧，移动前确认目标文件夹。超时或结果未知时先查询，避免重复保存或移动。
5. 按实际结果核对输出文件、页数、顺序及云保存结果，再交付路径或服务返回的链接。处理 file_id 不代表云文档已保存，不能拼接分享 URL。

已核对官方命令、策略与隔离凭证接口；真实账号处理与云文档 API 仅在实际成功后报告已验证。Runtime 使用必须具有本 bundle SHA-256 × RepoDigest 的真实 Conformance。

多图合并最多 100 张；不支持多个现有 PDF/Office 合并、跨类型合并、独立上传文件、创建文件夹或在线协同编辑。PDF 水印最多 100 页，带密码的 PDF/Office 不支持。输出格式和保存参数按对应叶子帮助确认。
