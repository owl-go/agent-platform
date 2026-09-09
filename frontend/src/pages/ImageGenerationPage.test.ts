// @vitest-environment jsdom
import { flushPromises, mount } from "@vue/test-utils";
import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import { platformApiKey, type ImageModel, type PlatformApi } from "../api/client";
import { authContextKey, type AuthContext } from "../auth/session";
import { createAppI18n } from "../i18n";
import ImageGenerationPage from "./ImageGenerationPage.vue";

const model: ImageModel = {
  id: "image-model-1", revision_id: "revision-1", display_name: "Studio", api_key_configured: true,
  provider_model_id: "gpt-image-1", api_protocol: "openai_images",
  modes: ["generate"], sizes: ["1024x1024"], qualities: ["high"], formats: ["png"], backgrounds: ["opaque"],
  default_size: "1024x1024", default_quality: "high", default_format: "png", default_background: "opaque",
  rates: [{ size: "1024x1024", quality: "high", amount_hundredths: 5000 }], state: "available",
  verified_at: "2026-09-08T00:00:00Z", created_at: "2026-09-08T00:00:00Z", updated_at: "2026-09-08T00:00:00Z", version: 2,
};

function auth(administrator = false): AuthContext {
  return { isCallback: false, session: { state: ref({ kind: "authenticated", currentUser: { id: "user-1", username: "user", email: "u@example.test", display_name: "User", administrator, settings_ready: true } }), accessToken: () => "token", initialize: vi.fn(async () => {}), signIn: vi.fn(async () => {}), signOut: vi.fn(async () => {}), dispose: vi.fn() } };
}

function mountPage(api: PlatformApi, administrator = false) {
  return mount(ImageGenerationPage, { global: { plugins: [createAppI18n({ getItem: () => "zh-CN" }, "zh-CN")], provide: { [platformApiKey as symbol]: api, [authContextKey as symbol]: auth(administrator) } } });
}

describe("ImageGenerationPage", () => {
  it("reconnects a cleanly closed event stream while the generation is still active", async () => {
    vi.useFakeTimers();
    const running = {
      id: "record-1", image_model_id: model.id, image_model_revision_id: model.revision_id, image_model_name: model.display_name,
      prompt: "a circle", mode: "generate", size: "1024x1024", quality: "high", format: "png", background: "opaque",
      requested_count: 1, validated_count: 0, reservation_hundredths: 5000, consumption_hundredths: 0,
      state: "running", images: [], created_at: "2026-09-09T00:00:00Z", version: 2,
    } as Awaited<ReturnType<PlatformApi["listImageGenerations"]>>[number];
    const streamImageGeneration = vi.fn(async () => undefined);
    const api = {
      getImageGenerationOptions: vi.fn(async () => ({ image_models: [model] })),
      listImageGenerations: vi.fn(async () => [running]),
      getImageGeneration: vi.fn(async () => running),
      streamImageGeneration,
    } as unknown as PlatformApi;
    const wrapper = mountPage(api);
    await flushPromises();

    expect(streamImageGeneration).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1_600);
    expect(streamImageGeneration).toHaveBeenCalledTimes(2);

    wrapper.unmount();
    vi.useRealTimers();
  });

	it("keeps rendering when an empty generation omits repeated JSON fields", async () => {
		const record = {
			id: "record-1", image_model_id: model.id, image_model_revision_id: model.revision_id, image_model_name: model.display_name,
			prompt: "a circle", mode: "generate", size: "1024x1024", quality: "high", format: "png", background: "opaque",
			requested_count: 1, validated_count: 0, reservation_hundredths: 5000, consumption_hundredths: 0,
			state: "outcome_unknown", created_at: "2026-09-09T00:00:00Z", version: 1,
		} as unknown as Awaited<ReturnType<PlatformApi["listImageGenerations"]>>[number];
		const api = { getImageGenerationOptions: vi.fn(async () => ({ image_models: [model] })), listImageGenerations: vi.fn(async () => [record]) } as unknown as PlatformApi;
		const wrapper = mountPage(api);
		await flushPromises();
		expect(wrapper.text()).toContain("生成结果");
		expect(wrapper.text()).toContain("a circle");
		expect(wrapper.text()).toContain("0.00 Credits");
		expect(wrapper.text()).not.toContain("NaN");
		expect(wrapper.find('[role="alert"]').exists()).toBe(false);
		wrapper.unmount();
	});

  it("keeps the workbench visible and explains an empty Image Model catalog", async () => {
    const api = { getImageGenerationOptions: vi.fn(async () => ({ image_models: [], prompt_optimization_models: [] })), listImageGenerations: vi.fn(async () => []) } as unknown as PlatformApi;
    const wrapper = mountPage(api);
    await flushPromises();
    expect(wrapper.text()).toContain("管理员尚未配置可用的图片模型");
    expect(wrapper.text()).toContain("生成结果");
    wrapper.unmount();
  });

  it("submits the selected compatible options and shows the reservation estimate", async () => {
    const submit = vi.fn(async (input) => ({ id: "record-1", image_model_id: model.id, image_model_revision_id: model.revision_id, image_model_name: model.display_name, prompt: input.prompt, mode: input.mode, size: input.size, quality: input.quality, format: input.format, background: input.background, requested_count: input.count, validated_count: 0, reservation_hundredths: 10000, consumption_hundredths: 0, state: "pending", images: [], created_at: "2026-09-08T00:00:00Z", version: 1 }));
    const api = { getImageGenerationOptions: vi.fn(async () => ({ image_models: [model], prompt_optimization_models: [] })), listImageGenerations: vi.fn(async () => []), submitImageGeneration: submit } as unknown as PlatformApi;
    const wrapper = mountPage(api);
    await flushPromises();
    await wrapper.get("textarea").setValue("一艘黄铜飞船");
    const count = wrapper.findComponent({ name: "ElInputNumber" });
    count.vm.$emit("update:modelValue", 2);
    await flushPromises();
    expect(wrapper.text()).toContain("100.00");
    await wrapper.get(".image-primary-action").trigger("click");
    await flushPromises();
    expect(submit).toHaveBeenCalledWith(expect.objectContaining({ image_model_id: model.id, prompt: "一艘黄铜飞船", count: 2, size: "1024x1024" }));
    wrapper.unmount();
  });

  it("keeps Image Model credentials separate from Model Provider Connections", async () => {
    const unverified = { ...model, endpoint: "https://images.example.test/v1", state: "unverified" as const, verified_at: undefined };
    const api = {
      getImageGenerationOptions: vi.fn(async () => ({ image_models: [], prompt_optimization_models: [] })),
      listImageGenerations: vi.fn(async () => []),
      listImageModels: vi.fn(async () => [unverified]),
      listPromptOptimizationCandidates: vi.fn(async () => []),
      verifyImageModel: vi.fn(async () => ({ ...unverified, state: "disabled" as const, verified_at: "2026-09-09T00:00:00Z", version: 2 })),
    } as unknown as PlatformApi;
    const wrapper = mountPage(api, true);
    await flushPromises();
    await wrapper.findAll("button").find((button) => button.text().includes("配置图片模型"))!.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("API 地址");
    expect(wrapper.text()).toContain("API Key");
    expect(wrapper.text()).toContain("提示词优化 Prompt");
    expect(wrapper.text()).not.toContain("模型供应商连接");
    expect(wrapper.text()).not.toContain("modes");
    await wrapper.findAll("button").find((button) => button.text() === "测试")!.trigger("click");
    await flushPromises();
    expect(wrapper.findAll<HTMLButtonElement>("button").find((button) => button.text() === "启用")!.element.disabled).toBe(false);
    wrapper.unmount();
  });
});
