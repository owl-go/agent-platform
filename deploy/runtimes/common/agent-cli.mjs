#!/usr/bin/env node
import net from "node:net";

// The broker limits decoded stdout and stderr to 8 MiB; JSON/base64 adds overhead.
const MAX_RESPONSE_BYTES = 12 * 1024 * 1024;

function fail(message, code = 2) {
  process.stderr.write(`agent-cli: ${message}\n`);
  process.exit(code);
}

function parseArguments(values) {
	if (values[0] === "describe") {
		if (values.length !== 3 || values[1] !== "--connector" || !values[2]) fail("usage: agent-cli describe --connector <id>");
		return { kind: "describe", connector_id: values[2] };
	}
  const command = { kind: "execute", connector_id: "", capability: "", identity: "" };
  let index = 0;
	while (index < values.length) {
    const option = values[index++];
    const value = values[index++];
    if (!value) fail(`missing value for ${option}`);
    if (option === "--connector") command.connector_id = value;
    else if (option === "--capability") command.capability = value;
    else if (option === "--identity") command.identity = value;
    else if (option === "--target") command.target = value;
		else if (option === "--input") {
			try { command.input = JSON.parse(value); }
			catch { fail("--input must be valid JSON"); }
			if (command.input === null || Array.isArray(command.input) || typeof command.input !== "object") fail("--input must be a JSON object");
		}
    else fail(`unknown option ${option}`);
  }
	if (!command.connector_id || !command.capability || !command.identity) fail("incomplete CLI command");
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
