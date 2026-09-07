import { describe, expect, it } from "vitest";
import { localizedSkillDescription, parseSkillDocument } from "./skillDocument";

describe("Skill document metadata", () => {
  const document = `---
name: pdf
display_name: PDF 文档处理
version: 1.2.0
description: Process PDF documents.
description_zh: >
  创建、读取并检查 PDF 文档。
  支持表格提取。
---
# Instructions
`;

  it("selects the description matching the interface language", () => {
    const metadata = parseSkillDocument(document);
    expect(metadata.displayName).toBe("PDF 文档处理");
    expect(localizedSkillDescription(metadata, "zh-CN")).toBe("创建、读取并检查 PDF 文档。 支持表格提取。");
    expect(localizedSkillDescription(metadata, "en-US")).toBe("Process PDF documents.");
    expect(metadata.body).toBe("# Instructions\n");
  });
});
