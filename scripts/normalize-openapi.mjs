import { readFile, writeFile } from "node:fs/promises";

const [path] = process.argv.slice(2);
if (!path) {
  throw new Error("OpenAPI document path is required");
}

const document = JSON.parse(await readFile(path, "utf8"));

function preserveLegacyNumbers(value) {
  if (Array.isArray(value)) {
    value.forEach(preserveLegacyNumbers);
    return;
  }
  if (!value || typeof value !== "object") {
    return;
  }
  if (value.type === "string" && (value.format === "int64" || value.format === "uint64")) {
    value.type = "integer";
  }
  Object.values(value).forEach(preserveLegacyNumbers);
}

preserveLegacyNumbers(document);
await writeFile(path, `${JSON.stringify(document, null, 2)}\n`);
