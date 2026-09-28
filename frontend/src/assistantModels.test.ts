import { describe, expect, it } from "vitest";
import type { ModelProviderConnection } from "./api/client";
import { assistantModelOptions } from "./assistantModels";

const connection = (protocols: string[], available: boolean, api_key_configured = true): ModelProviderConnection => ({
  id: "connection-1", name: "模型服务", provider_type: "openai", endpoint: "https://example.test/v1", protocols, api_key_configured,
  verification_status: "unverified", custom_endpoint: true,
  models: [{ id: "model-1", connection_id: "connection-1", model_id: "chat-model", display_name: "聊天模型", available, manually_added: false, compatibility: [] }],
  created_at: "2026-09-23T00:00:00Z", updated_at: "2026-09-23T00:00:00Z", version: 1,
});

describe("assistantModelOptions", () => {
  it("offers only available models with keys and openai_responses support", () => {
		expect(assistantModelOptions([connection(["openai_chat"], true), connection(["openai_responses"], false), connection(["openai_responses"], true, false)])).toEqual([]);
		expect(assistantModelOptions([connection(["openai_chat", "openai_responses"], true)])).toEqual([{ value: "model-1", label: "模型服务 / 聊天模型" }]);
  });
});
