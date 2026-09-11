import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ConnectorIcon from "./ConnectorIcon.vue";

describe("ConnectorIcon", () => {
  it("renders uploaded image data and keeps a preset fallback", () => {
    const image = "data:image/png;base64,AAAA";
    const uploaded = mount(ConnectorIcon, { props: { icon: image, size: 24 } });
    expect(uploaded.find("img").attributes("src")).toBe(image);

    const preset = mount(ConnectorIcon, { props: { icon: "terminal" } });
    expect(preset.find("img").exists()).toBe(false);
    expect(preset.find(".profile-icon").exists()).toBe(true);
  });
});
