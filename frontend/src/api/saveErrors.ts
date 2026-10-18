import { ApiError } from "./client";

type Translate = (key: string) => string;
const reasons: Record<string, string> = {
  model_provider_name_conflict: "providerNameConflict",
  version_conflict: "versionConflict",
  credit_conflict: "versionConflict",
  invalid_request_body: "invalidRequestBody",
  resource_conflict: "resourceConflict",
  insufficient_credits: "insufficientCredits",
  assistant_model_unavailable: "modelUnavailable",
  workflow_credential_secret_unavailable: "workflowCredentialUnavailable",
};
const validationDetails: Record<string, string> = {
  "Provider Connection name must contain 1-100 characters": "providerNameInvalid",
  "Model Endpoint must be an absolute HTTP or HTTPS URL": "endpointInvalid",
  "at least one Model API Protocol is required": "protocolRequired",
  "unsupported Model API Protocol": "protocolInvalid",
  "duplicate Model API Protocol": "protocolDuplicate",
  "API Key is required": "apiKeyRequired",
  "API Key is too large": "apiKeyTooLarge",
  "unsupported model provider": "providerInvalid",
};

/** Format a save failure without exposing raw exceptions or provider responses. */
export function saveErrorMessage(cause: unknown, t: Translate, fallback = "errors.generic"): string {
  if (!(cause instanceof ApiError)) return `${t(fallback)} ${t("saveErrors.unknown")}`;
  let message: string;
  if (cause.detail) message = Object.hasOwn(validationDetails, cause.detail) ? t(`saveErrors.${validationDetails[cause.detail]}`) : cause.detail;
  else if (cause.code === "invalid_request_body") message = `${t(fallback)} ${t("saveErrors.invalidRequestBody")}`;
  else if (Object.hasOwn(reasons, cause.code)) message = t(`saveErrors.${reasons[cause.code]}`);
  else if (cause.status === 413) message = `${t(fallback)} ${t("saveErrors.tooLarge")}`;
  else message = `${t(fallback)} ${t(`saveErrors.${cause.kind}`)}`;
  if (cause.requestID) message += ` ${t("saveErrors.requestID")}: ${cause.requestID}`;
  return message;
}
