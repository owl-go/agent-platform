import { ref } from "vue";

export interface CommandApprovalPlacement { executionKind: "session" | "run"; executionID: string }

export const embeddedCommandApproval = ref<CommandApprovalPlacement>();

export function placeCommandApproval(executionKind: CommandApprovalPlacement["executionKind"], executionID: string) {
  embeddedCommandApproval.value = { executionKind, executionID };
}

export function clearCommandApproval(executionKind: CommandApprovalPlacement["executionKind"], executionID: string) {
  const current = embeddedCommandApproval.value;
  if (current?.executionKind === executionKind && current.executionID === executionID) embeddedCommandApproval.value = undefined;
}
