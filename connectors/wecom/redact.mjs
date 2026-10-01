export function redact(value, sensitiveValues) {
  return sensitiveValues.filter(Boolean).reduce((text, sensitive) => text.split(sensitive).join('[REDACTED]'), value ?? '');
}
