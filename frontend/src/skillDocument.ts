export interface SkillDocumentMetadata {
  displayName?: string;
  version?: string;
  descriptions: Record<string, string>;
  body: string;
}

function unquote(value: string) {
  const trimmed = value.trim();
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    try { return JSON.parse(trimmed) as string; } catch { return trimmed.slice(1, -1); }
  }
  if (trimmed.startsWith("'") && trimmed.endsWith("'")) return trimmed.slice(1, -1).replace(/''/g, "'");
  return trimmed;
}

export function parseSkillDocument(content: string): SkillDocumentMetadata {
  const normalized = content.replace(/\r\n/g, "\n");
  if (!normalized.startsWith("---\n")) return { descriptions: {}, body: content };
  const end = normalized.indexOf("\n---", 4);
  if (end < 0) return { descriptions: {}, body: content };

  const lines = normalized.slice(4, end).split("\n");
  const values: Record<string, string> = {};
  for (let index = 0; index < lines.length; index += 1) {
    const match = lines[index]!.match(/^([A-Za-z0-9_-]+):(?:\s*(.*))?$/);
    if (!match) continue;
    const key = match[1]!;
    const raw = match[2]?.trim() ?? "";
    if (raw === "|" || raw === ">" || raw.startsWith("|-") || raw.startsWith(">-")) {
      const block: string[] = [];
      while (index + 1 < lines.length && (/^\s+/.test(lines[index + 1]!) || lines[index + 1] === "")) {
        index += 1;
        block.push(lines[index]!.replace(/^\s{1,2}/, ""));
      }
      values[key] = raw.startsWith(">") ? block.join(" ").trim() : block.join("\n").trim();
    } else {
      values[key] = unquote(raw);
    }
  }

  const descriptions = Object.fromEntries(Object.entries(values).filter(([key, value]) => key === "description" || key.startsWith("description_") && Boolean(value)));
  return {
    displayName: values.display_name?.trim() || undefined,
    version: values.version?.trim() || undefined,
    descriptions,
    body: normalized.slice(end + 4).replace(/^\n/, ""),
  };
}

export function localizedSkillDescription(metadata: SkillDocumentMetadata, locale: string) {
  const normalized = locale.toLowerCase().replace("-", "_");
  const language = normalized.split("_")[0]!;
  return metadata.descriptions[`description_${normalized}`]
    || metadata.descriptions[`description_${language}`]
    || (language === "en" ? metadata.descriptions.description : undefined)
    || metadata.descriptions.description
    || Object.values(metadata.descriptions)[0]
    || "";
}
