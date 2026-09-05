#!/usr/bin/env node
import net from "node:net";

// The broker limits decoded stdout and stderr to 8 MiB; JSON/base64 adds overhead.
const MAX_RESPONSE_BYTES = 12 * 1024 * 1024;

function fail(message, code = 2) {
  process.stderr.write(`agent-cli: ${message}\n`);
  process.exit(code);
}

function parseArguments(values) {
  const command = { connector_id: "", capability: "", identity: "", arguments: [] };
  let index = 0;
  while (index < values.length && values[index] !== "--") {
    const option = values[index++];
    const value = values[index++];
    if (!value) fail(`missing value for ${option}`);
    if (option === "--connector") command.connector_id = value;
    else if (option === "--capability") command.capability = value;
    else if (option === "--identity") command.identity = value;
    else if (option === "--target") command.target = value;
    else fail(`unknown option ${option}`);
  }
  if (values[index] !== "--") fail("missing command separator --");
  command.arguments = values.slice(index + 1);
  if (!command.connector_id || !command.capability || !command.identity || command.arguments.length === 0) fail("incomplete CLI command");
  return command;
}

const socketPath = process.env.AGENT_PLATFORM_CLI_SOCKET;
if (!socketPath) fail("CLI broker is unavailable");
const command = parseArguments(process.argv.slice(2));
const socket = net.createConnection(socketPath);
let response = Buffer.alloc(0);

socket.on("connect", () => socket.end(`${JSON.stringify(command)}\n`));
socket.on("data", (chunk) => {
  response = Buffer.concat([response, chunk]);
  if (response.length > MAX_RESPONSE_BYTES) socket.destroy(new Error("broker response exceeds limit"));
});
socket.on("error", (error) => fail(error.message));
socket.on("end", () => {
  let result;
  try {
    result = JSON.parse(response.toString("utf8"));
  } catch {
    fail("invalid broker response");
  }
  if (result.error_code) fail(`${result.error_code}: ${result.error_message ?? "command rejected"}`);
  if (result.stdout_base64) process.stdout.write(Buffer.from(result.stdout_base64, "base64"));
  if (result.stderr_base64) process.stderr.write(Buffer.from(result.stderr_base64, "base64"));
  const exitCode = Number(result.exit_code ?? 0);
  process.exit(Number.isInteger(exitCode) && exitCode >= 0 && exitCode <= 255 ? exitCode : 2);
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => socket.destroy());
}
