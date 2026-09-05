import type { Artifact } from "./api/client";

export function displayArtifactNames(content: string, artifacts: Artifact[] | undefined): string {
  const files = [...(artifacts ?? [])]
    .filter((artifact) => artifact.kind === "file" && artifact.path)
    .sort((left, right) => right.path.length - left.path.length);
  return files.reduce((result, artifact) => result.replaceAll(`/workspace/${artifact.path.replace(/^\/+/, "")}`, artifact.name), content);
}
