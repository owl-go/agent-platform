import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ConnectorIcon from "./ConnectorIcon.vue";

describe("ConnectorIcon", () => {
  it("renders uploaded image data and keeps a preset fallback", () => {
    const image = "data:image/png;base64,AAAA";
    const uploaded = mount(ConnectorIcon, { props: { icon: image, size: 24 } });
    expect(uploaded.find("img").attributes("src")).toBe(image);

    const feishu = mount(ConnectorIcon, { props: { icon: "feishu" } });
    expect(feishu.find("img").attributes("src")).toContain("feishu.png");

    const modao = mount(ConnectorIcon, { props: { icon: "modao" } });
    expect(modao.find("img").attributes("src")).toContain("modao.png");
    const notion = mount(ConnectorIcon, { props: { icon: "notion" } });
    expect(notion.find("img").attributes("src")).toContain("Notion%20connector");

    const openboost = mount(ConnectorIcon, { props: { icon: "openboost" } });
    expect(openboost.find("img").attributes("src")).toContain("openboost.svg");

    const preset = mount(ConnectorIcon, { props: { icon: "terminal" } });
    expect(preset.find("img").exists()).toBe(false);
    expect(preset.find(".profile-icon").exists()).toBe(true);
  });
});
