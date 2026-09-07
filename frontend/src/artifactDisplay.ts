import type { Artifact } from "./api/client";

const markdownLink = /!?\[[^\]]*\]\(\s*(<[^>]+>|(?:\\.|[^)\s])+)(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)/g;

function normalizedLocalTarget(value: string): string | undefined {
  let target = value.trim();
  if (target.startsWith("<") && target.endsWith(">")) target = target.slice(1, -1);
  target = target.replace(/\\([\\()[\]_*])/g, "$1");
  if (/^[a-z][a-z\d+.-]*:/i.test(target) || target.startsWith("//") || target.startsWith("#")) return undefined;
  target = target.split(/[?#]/, 1)[0] ?? "";
  try { target = decodeURIComponent(target); } catch { return undefined; }
  target = target.replace(/^\.\//, "").replace(/^\/?workspace\//, "").replace(/^\/+/, "");
  return target || undefined;
}

function inlineCode(value: string): string {
  const longestRun = Math.max(0, ...(value.match(/`+/g) ?? []).map((run) => run.length));
  const fence = "`".repeat(longestRun + 1);
  const padding = value.startsWith("`") || value.endsWith("`") ? " " : "";
  return `${fence}${padding}${value}${padding}${fence}`;
}

export function displayArtifactNames(content: string, artifacts: Artifact[] | undefined): string {
  const files = [...(artifacts ?? [])]
    .filter((artifact) => artifact.kind === "file" && artifact.path)
    .sort((left, right) => right.path.length - left.path.length);
  const withoutArtifactLinks = content.replace(markdownLink, (source, destination: string) => {
    const target = normalizedLocalTarget(destination);
    const artifact = target && files.find((item) => target === item.path.replace(/^\/+/, "") || target === item.name);
    return artifact ? inlineCode(artifact.name) : source;
  });
  return files.reduce((result, artifact) => result.replaceAll(`/workspace/${artifact.path.replace(/^\/+/, "")}`, artifact.name), withoutArtifactLinks);
}
