#!/usr/bin/env node
"use strict";

const { spawn } = require("node:child_process");
const { realpathSync } = require("node:fs");
const path = require("node:path");

const args = process.argv.slice(2);
const publicHelp = args.length === 1 && ["--help", "-h", "--version", "-V"].includes(args[0]);
const commands = [
  ["pages", "get"], ["pages", "create"], ["pages", "edit"], ["pages", "trash"],
  ["datasources", "query"], ["datasources", "resolve"],
  ["api", "v1/search"], ["api", "v1/users/me"],
];
if (!publicHelp && !commands.some((prefix) => prefix.every((part, i) => args[i] === part))) {
  console.error("Notion command is outside the reviewed connector policy");
  process.exit(2);
}
if (args.some((arg) => ["--unsafe-verbose", "--allow-deleting-content",
  "--workers-config-file", "--filter-file", "--env"].some((blocked) =>
  arg === blocked || arg.startsWith(blocked + "=")))) {
  console.error("Notion command includes a disallowed option");
  process.exit(2);
}
if (args[0] === "pages" && args[1] === "create" &&
    (!args.includes("--parent") || !args.includes("--content"))) {
  console.error("Notion page creation requires an explicit parent and content");
  process.exit(2);
}
if (args[0] === "pages" && args[1] === "edit" && !args.includes("--content")) {
  console.error("Notion page editing requires explicit content");
  process.exit(2);
}
if (args[0] === "pages" && args[1] === "trash" && !args.includes("--yes")) {
  console.error("Notion page trash requires --yes after platform approval");
  process.exit(2);
}
if (args[0] === "api" && args[1] === "v1/search" &&
    args.slice(2).some((arg) => !/^(query|start_cursor)=.{1,2000}$/.test(arg) &&
      !/^page_size:=(?:[1-9]|[1-9][0-9]|100)$/.test(arg))) {
  console.error("Notion search accepts only query, page_size, and start_cursor");
  process.exit(2);
}
if (args[0] === "api" && args[1] === "v1/users/me" && args.length !== 2) {
  console.error("Notion identity does not accept additional arguments");
  process.exit(2);
}

const env = { ...process.env };
delete env.CONNECTOR_CREDENTIALS_JSON;
delete env.NOTION_API_TOKEN;
delete env.NOTION_API_BASE_URL;
delete env.NOTION_API_DOCS_BASE_URL;
delete env.NOTION_ENV;
delete env.NOTION_WORKSPACE_ID;
env.NOTION_HOME = "/tmp/notion-connector";
env.XDG_CACHE_HOME = "/tmp/notion-connector/cache";
if (!publicHelp) {
  let credentials;
  try {
    credentials = JSON.parse(process.env.CONNECTOR_CREDENTIALS_JSON || "");
  } catch (_) {
    console.error("Notion authorization is missing or invalid");
    process.exit(2);
  }
  if (!credentials || typeof credentials.token !== "string" ||
      credentials.token.length < 8 || credentials.token.length > 4096) {
    console.error("Notion authorization is missing or invalid");
    process.exit(2);
  }
  env.NOTION_API_TOKEN = credentials.token;
}

const platform = process.platform === "linux" ? `linux-${process.arch}` : "";
if (platform !== "linux-arm64" && platform !== "linux-x64") {
  console.error("Notion connector requires Linux arm64 or x64");
  process.exit(2);
}
const binary = path.join(path.dirname(realpathSync(__filename)), "..", "ntn", "dist", `ntn-${platform}`, "ntn");
const child = spawn(binary, args, { env, stdio: "inherit" });
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => child.kill(signal));
}
child.on("error", () => {
  console.error("Could not start the pinned Notion CLI");
  process.exit(1);
});
child.on("exit", (code, signal) => process.exit(code ?? (signal ? 128 : 1)));
