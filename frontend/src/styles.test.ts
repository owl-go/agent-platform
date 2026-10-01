import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const styles = readFileSync(resolve(process.cwd(), "src/styles.css"), "utf8");

describe("conversation layout styles", () => {
  it("keeps Workflow card text to one line and aligns the bottom actions", () => {
    const title = styles.match(/\.workflow-card h2\s*\{([^}]*)\}/)?.[1];
    const goal = styles.match(/\.workflow-goal\s*\{([^}]*)\}/)?.[1];
    const body = styles.match(/\.workflow-card\.el-card > \.el-card__body\s*\{([^}]*)\}/)?.[1];
    const footer = [...styles.matchAll(/\.workflow-card footer\s*\{([^}]*)\}/g)].at(-1)?.[1];

    for (const text of [title, goal]) {
      expect(text).toMatch(/white-space:\s*nowrap/);
      expect(text).toMatch(/overflow:\s*hidden/);
      expect(text).toMatch(/text-overflow:\s*ellipsis/);
    }
    expect(body).toMatch(/flex:\s*1/);
    expect(footer).toMatch(/margin-top:\s*auto/);
  });

  it("keeps execution activity titles and state on one readable row", () => {
    const summaryDeclarations = [...styles.matchAll(/\.runtime-activity \.activity-summary-group > summary\s*\{([^}]*)\}/g)]
      .map((match) => match[1])
      .find((declarations) => declarations?.includes("grid-template-columns"));

    expect(summaryDeclarations).toMatch(/grid-template-columns:\s*8px auto minmax\(0,\s*1fr\) auto 8px/);
    expect(summaryDeclarations).toMatch(/width:\s*100%/);
  });

  it("centers the Workflow Run Conversation composer in the content column", () => {
    const alignmentDeclarations = [...styles.matchAll(/\.composer-layer\.run-composer-layer\s*\{([^}]*)\}/g)]
      .map((match) => match[1])
      .filter((declarations) => declarations?.includes("justify-items"));
    expect(alignmentDeclarations.at(-1)).toMatch(/justify-items:\s*center/);
  });

  it("keeps the Smart Assistant composer inside the viewport", () => {
    const pageLayout = styles.match(
      /\.ai-applications-page--conversation\s*\{([^}]*)\}/,
    )?.[1];
    const conversationLayout = styles.match(
      /\.assistant-conversation-page\s*\{([^}]*)\}/,
    )?.[1];

    expect(pageLayout).toMatch(/height:\s*100dvh/);
    expect(pageLayout).toMatch(/overflow:\s*hidden/);
    expect(conversationLayout).toMatch(/height:\s*100%/);
  });
});
