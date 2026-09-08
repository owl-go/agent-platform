import { ref } from "vue";

export const embeddedSessionApprovalID = ref<string>();

export function placeSessionApproval(executionID?: string) {
  embeddedSessionApprovalID.value = executionID;
}

export function clearSessionApproval(executionID?: string) {
  if (!executionID || embeddedSessionApprovalID.value === executionID) embeddedSessionApprovalID.value = undefined;
}
