# 图片增强与修复工作流

> **前置阅读**：`references/image-processing.md`（enhance 模式值列表、hd/restore 参数）

## 场景

用户有模糊、偏暗、有阴影、有划痕的照片需要修复或增强。

## 决策树

| 用户描述 | 路由到 |
|----------|--------|
| "照片太模糊了" | `image hd` |
| "照片太暗了" | `image enhance --mode 1` |
| "有阴影遮挡" | `image enhance --mode 5` |
| "老照片有划痕" | `image restore` |
| "屏幕翻拍有纹路" | `image enhance --mode 8` |
| "想要黑白效果" | `image enhance --mode 3` |
| "去掉手写标注" | `image enhance --mode 9` |
| "去掉水印" | `image enhance --mode 10` |

## 多步组合

单次处理效果不够时，可链式处理（先输出本地，再二次处理）：

```
camscanner-cli image enhance input.jpg --mode 5 -o temp.jpg
# 上一步成功且 temp.jpg 可用后执行
camscanner-cli image hd temp.jpg -o enhanced.jpg -s --save-title "图片增强"
```

中间产物只存本地。上例最终产物默认双保存；用户已指定保存方式时按 `SKILL.md` 调整，避免复用会覆盖已有文件的路径。
