// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Attachment } from "../api/client";
import { createAppI18n } from "../i18n";
import ConversationAttachments from "./ConversationAttachments.vue";

const attachments: Attachment[] = [
  { id: "image-1", name: "first.png", content_type: "image/png", size: 1024, sha256: "one", image: true },
  { id: "image-2", name: "second.jpg", content_type: "image/jpeg", size: 2048, sha256: "two", image: true },
  { id: "file-1", name: "notes.txt", content_type: "text/plain", size: 512, sha256: "three", image: false },
];

describe("ConversationAttachments", () => {
  beforeEach(() => {
    let nextURL = 0;
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => `blob:attachment-${++nextURL}`) });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
  });

  afterEach(() => {
    document.body.innerHTML = "";
    delete (URL as { createObjectURL?: unknown }).createObjectURL;
    delete (URL as { revokeObjectURL?: unknown }).revokeObjectURL;
    vi.restoreAllMocks();
  });

  function mountGallery(loadAttachment = vi.fn(async (id: string) => new Blob([id]))) {
    return {
      loadAttachment,
      wrapper: mount(ConversationAttachments, {
        attachTo: document.body,
        props: { attachments, loadAttachment },
        global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")] },
      }),
    };
  }

  it("previews images without downloading and navigates through every image", async () => {
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const { wrapper } = mountGallery();
    await flushPromises();

    await wrapper.findAll(".turn-attachment")[0]!.trigger("click");
    await flushPromises();
    let preview = document.body.querySelector<HTMLElement>(".image-preview-layer")!;
    expect(preview.getAttribute("role")).toBe("dialog");
    expect(preview.querySelector<HTMLImageElement>(".image-preview-canvas img")?.alt).toBe("first.png");
    expect(preview.textContent).toContain("第 1 张，共 2 张");
    expect(anchorClick).not.toHaveBeenCalled();

    preview.querySelector<HTMLButtonElement>(".image-preview-nav.next")!.click();
    await flushPromises();
    preview = document.body.querySelector<HTMLElement>(".image-preview-layer")!;
    expect(preview.querySelector<HTMLImageElement>(".image-preview-canvas img")?.alt).toBe("second.jpg");
    expect(preview.textContent).toContain("第 2 张，共 2 张");

    preview.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowUp", bubbles: true }));
    await flushPromises();
    expect(document.body.querySelector<HTMLImageElement>(".image-preview-canvas img")?.alt).toBe("first.png");
    wrapper.unmount();
  });

  it("keeps non-image attachments as explicit downloads", async () => {
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const { loadAttachment, wrapper } = mountGallery();
    await flushPromises();
    vi.mocked(loadAttachment).mockClear();

    await wrapper.findAll(".turn-attachment")[2]!.trigger("click");
    await flushPromises();

    expect(loadAttachment).toHaveBeenCalledWith("file-1");
    expect(anchorClick).toHaveBeenCalledOnce();
    expect(document.body.querySelector(".image-preview-layer")).toBeNull();
    wrapper.unmount();
  });
});
