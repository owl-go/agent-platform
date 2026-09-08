// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type ImageModel, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ImageGenerationPage from "./ImageGenerationPage.vue";

const model: ImageModel = {
  id: "image-model-1", revision_id: "revision-1", display_name: "Studio", connection_id: "connection-1",
  connection_version: 1, connection_name: "OpenAI", provider_model_id: "gpt-image-1", api_protocol: "openai_images",
  modes: ["generate"], sizes: ["1024x1024"], qualities: ["high"], formats: ["png"], backgrounds: ["opaque"],
  default_size: "1024x1024", default_quality: "high", default_format: "png", default_background: "opaque",
  rates: [{ size: "1024x1024", quality: "high", amount_hundredths: 125 }], state: "available",
  verified_at: "2026-09-08T00:00:00Z", created_at: "2026-09-08T00:00:00Z", updated_at: "2026-09-08T00:00:00Z", version: 2,
};

function auth(administrator = false): AuthContext {
  return { isCallback: false, session: { state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "u@example.test", display_name: "User", administrator, settings_ready: true } }), accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn() } };
}

function mountPage(api: PlatformApi, administrator = false) {
  return mount(ImageGenerationPage, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth(administrator) } } });
}

describe("ImageGenerationPage", () => {
  it("keeps the workbench visible and explains an empty Image Model catalog", async () => {
    const api = { getImageGenerationOptions: vi.fn(async () => ({ image_models: [], prompt_optimization_models: [] })), listImageGenerations: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mountPage(api);
    await flushPromises();
    expect(wrapper.text()).toContain("管理员尚未配置可用的图片模型");
    expect(wrapper.text()).toContain("生成结果");
    wrapper.unmount();
  });

  it("submits the selected compatible options and shows the reservation estimate", async () => {
    const submit = vi.fn(async (input) => ({ id: "record-1", image_model_id: model.id, image_model_revision_id: model.revision_id, image_model_name: model.display_name, connection_name: model.connection_name, prompt: input.prompt, mode: input.mode, size: input.size, quality: input.quality, format: input.format, background: input.background, requested_count: input.count, validated_count: 0, reservation_hundredths: 250, consumption_hundredths: 0, state: "pending", images: [], created_at: "2026-09-08T00:00:00Z", version: 1 }));
    const api = { getImageGenerationOptions: vi.fn(async () => ({ image_models: [model], prompt_optimization_models: [] })), listImageGenerations: vi.fn(async () => []), submitImageGeneration: submit } as unknown as PlatformApi;
    const wrapper = mountPage(api);
    await flushPromises();
    await wrapper.get("textarea").setValue("一艘黄铜飞船");
    const count = wrapper.findComponent({ name: "ElInputNumber" });
    count.vm.$emit("update:modelValue", 2);
    await flushPromises();
    expect(wrapper.text()).toContain("2.50");
    await wrapper.get(".image-primary-action").trigger("click");
    await flushPromises();
    expect(submit).toHaveBeenCalledWith(expect.objectContaining({ image_model_id: model.id, prompt: "一艘黄铜飞船", count: 2, size: "1024x1024" }));
    wrapper.unmount();
  });
});
