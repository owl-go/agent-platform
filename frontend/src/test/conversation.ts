import { vi } from "vitest";
import type { ConversationSelection, PlatformApi, SelectedResource, SelectionInput } from "../api/client";

export function emptySelection(): ConversationSelection {
  return { id: "selection-1", expert_id: "", expert_team_id: "", name: "", icon: "", icon_background: "", member_count: 0, skills: [], mcp_servers: [], cli_connectors: [], inherited_skills: [], inherited_mcp_servers: [], inherited_cli_connectors: [], disabled_connectors: [] };
}
export function conversationApiStub(initial = emptySelection()): Partial<PlatformApi> {
  const revisions = new Map([[initial.id, initial]]);
  let count = 1;
  return {
    listSkills: vi.fn(async () => []), listMCPServers: vi.fn(async () => []), listCLIConnectorDefinitions: vi.fn(async () => []), listCLIConnectorEnablements: vi.fn(async () => []),
    getConversationSelection: vi.fn(async () => structuredClone(initial)),
    listConversationFiles: vi.fn(async () => []),
    resolveConversationSelection: vi.fn(async (_scope, input: SelectionInput) => {
      const previous = revisions.get(input.previous_id ?? initial.id) ?? initial;
      const resources = (ids: string[], old: SelectedResource[]) => ids.map((id) => old.find((item) => item.id === id) ?? { id, name: id, revision: "revision-1" });
      const next = { ...previous, id: `selection-${++count}`, skills: resources(input.skill_ids, previous.skills), mcp_servers: resources(input.mcp_server_ids, previous.mcp_servers), cli_connectors: resources(input.cli_connector_ids, previous.cli_connectors), disabled_connectors: [...input.disabled_connectors] };
      if (input.change_expert) { next.expert_id = input.expert_id ?? ""; next.expert_team_id = input.expert_team_id ?? ""; next.name = next.expert_id || next.expert_team_id; }
      revisions.set(next.id, next); return structuredClone(next);
    }),
  };
}
