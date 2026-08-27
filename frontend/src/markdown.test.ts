import { describe, expect, it } from "vitest";
import { renderMarkdown } from "./markdown";

describe("renderMarkdown", () => {
  it("renders common Markdown structures", () => {
    const html = renderMarkdown("## 天气\n\n- **温度**：28°C\n- `湿度`：82%");

    expect(html).toContain("<h2>天气</h2>");
    expect(html).toContain("<ul>");
    expect(html).toContain("<strong>温度</strong>");
    expect(html).toContain("<code>湿度</code>");
  });

  it("does not execute raw HTML, unsafe links, or remote images", () => {
    const html = renderMarkdown('<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n![tracker](https://example.com/pixel.gif)');

    expect(html).not.toContain("<script>");
    expect(html).not.toContain('href="javascript:');
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("tracker");
  });

  it("hardens rendered links opened in a new tab", () => {
    const html = renderMarkdown("[OpenAI](https://openai.com)");

    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noopener noreferrer"');
  });
});
