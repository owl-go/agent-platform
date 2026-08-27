---
status: accepted
---

# Workflow Run 只在成功后推进持久 Workspace

每个 Workflow 拥有一个持久 Workspace，但每次 Run 在临时副本中执行并且同一 Workflow 串行写入；只有成功 Run 才原子推进持久文件树，失败或取消会丢弃本次变化。该选择比直接在共享目录执行需要更多存储和合并工作，却能让 Workspace 始终代表最近成功状态，避免半成品污染后续 Run；Artifact 则独立冻结成功 Run 的最终结果和变更文件。
