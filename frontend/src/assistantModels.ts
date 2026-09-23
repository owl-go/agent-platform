import type { ModelProviderConnection } from "./api/client";

export function assistantModelOptions(connections: ModelProviderConnection[]) {
  return connections
    .filter((connection) => connection.api_key_configured && connection.protocols.includes("openai_chat"))
    .flatMap((connection) => connection.models
      .filter((model) => model.available)
      .map((model) => ({ value: model.id, label: `${connection.name} / ${model.display_name || model.model_id}` })));
}
