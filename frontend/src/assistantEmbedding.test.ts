import { afterEach, describe, expect, it } from "vitest";
import { assistantEmbedSnippet } from "./assistantEmbedding";

const share = { enabled: true, width: "400px", height: 600, embed_type: "floating" as const, widget_default_open: false, widget_icon: "private-object-key" };
const labels = { open: "打开聊天窗口", close: "收起聊天窗口" };
function run(snippet: string) {
  const script = snippet.slice("<script>".length, -"</script>".length);
  // Execute the actual generated host-site snippet, rather than the installer in isolation.
  new Function(script)();
  return document.getElementById("assistant-widget-test-token")!.shadowRoot!;
}
afterEach(() => { document.getElementById("assistant-widget-test-token")?.remove(); });
describe("Assistant embedding", () => {
  it("creates a traditional iframe that fills the containing area", () => {
    const html = assistantEmbedSnippet("https://platform.example.test", "test-token", '助手"<测试>', { ...share, embed_type: "fullscreen", width: "100%" }, labels);
    const root = document.createElement("div"); root.innerHTML = html;
    const frame = root.querySelector("iframe")!;
    expect(frame.src).toBe("https://platform.example.test/embed/assistant/test-token");
    expect(frame.title).toBe('助手"<测试>');
    expect(frame.style.width).toBe("100%");
    expect(frame.style.height).toBe("100%");
    expect(frame.style.minHeight).toBe("600px");
    expect(root.querySelector("script")).toBeNull();
  });
  it.each([false, true])("starts open=%s and preserves the iframe across collapse and reopen", (defaultOpen) => {
    const code = assistantEmbedSnippet("https://platform.example.test", "test-token", "助手", { ...share, widget_default_open: defaultOpen }, labels);
    const shadow = run(code);
    const panel = shadow.querySelector(".panel") as HTMLElement;
    const launcher = shadow.querySelector(".launcher") as HTMLButtonElement;
    const frame = shadow.querySelector("iframe")!;
    expect(panel.hidden).toBe(!defaultOpen);
    expect(launcher.getAttribute("aria-expanded")).toBe(String(defaultOpen));
    expect(launcher.querySelector("img")?.src).toBe("https://platform.example.test/api/v1/public/assistants/test-token/widget-icon");
    launcher.click(); expect(panel.hidden).toBe(defaultOpen);
    launcher.click(); expect(panel.hidden).toBe(!defaultOpen);
    (shadow.querySelector(".close") as HTMLButtonElement).click(); expect(panel.hidden).toBe(true);
    launcher.click(); expect(panel.hidden).toBe(false);
    expect(shadow.querySelector("iframe")).toBe(frame);
    run(code); expect(document.querySelectorAll("#assistant-widget-test-token")).toHaveLength(1);
    expect(code).not.toContain("private-object-key");
  });
  it("escapes script termination text and falls back when the custom image fails", () => {
    const name = '</script><img src=x onerror="alert(1)">';
    const code = assistantEmbedSnippet("https://platform.example.test", "test-token", name, share, labels);
    expect(code.match(/<\/script>/g)).toHaveLength(1);
    const shadow = run(code);
    expect(shadow.querySelector("iframe")?.title).toBe(name);
    shadow.querySelector("img")!.dispatchEvent(new Event("error"));
    expect(shadow.querySelector(".launcher svg")).not.toBeNull();
    expect(document.querySelector("img[onerror]")).toBeNull();
  });
});
