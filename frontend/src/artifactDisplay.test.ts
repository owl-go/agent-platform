import { describe, expect, it } from "vitest";
import type { Artifact } from "./api/client";
import { displayArtifactNames } from "./artifactDisplay";

const artifact: Artifact = {
  id: "artifact-1",
  run_id: "run-1",
  kind: "file",
  name: "Agent Workspace 项目介绍.pptx",
  path: "reports/Agent Workspace 项目介绍.pptx",
  size: 1,
  expired: false,
  created_at: "2026-09-07T00:00:00Z",
};

describe("displayArtifactNames", () => {
  it("turns a captured Artifact link into a plain file name", () => {
    const result = displayArtifactNames(
      "[下载演示文稿](reports/Agent%20Workspace%20%E9%A1%B9%E7%9B%AE%E4%BB%8B%E7%BB%8D.pptx)",
      [artifact],
    );

    expect(result).toBe("`Agent Workspace 项目介绍.pptx`");
  });

  it("keeps external and unrelated links unchanged", () => {
    const content = "[产品文档](https://example.com/docs) [工作说明](guide.md)";

    expect(displayArtifactNames(content, [artifact])).toBe(content);
  });
});
