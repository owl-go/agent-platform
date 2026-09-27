import { spawn } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { OpenAI as BundledOpenAI } from "/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/dist/bundle/chunks/chunk-NUHFSC37.js";
import DependencyOpenAI from "/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/index.mjs";

async function collect(OpenAI, responseBody) {
	const client = new OpenAI({
		apiKey: "smoke-test",
		baseURL: "https://models.example.test/v1",
		fetch: async () =>
			new Response(responseBody, {
				status: 200,
				headers: { "content-type": "text/event-stream" },
			}),
	});
	const stream = await client.responses.create({
		model: "smoke-test",
		input: "reply ok",
		stream: true,
	});
	const events = [];
	for await (const event of stream) {
		events.push(event);
	}
	return events;
}

for (const [implementation, OpenAI] of [
	["dependency", DependencyOpenAI],
	["bundle", BundledOpenAI],
]) {
	const events = await collect(OpenAI, [
		"data:\n\n",
		'data: {"type":"response.created","response":{"id":"response-1","status":"in_progress","output":[]}}\n\n',
		"data: [DONE]\n\n",
	].join(""));

	if (events.length !== 1 || events[0]?.type !== "response.created") {
		throw new Error(`${implementation} emitted unexpected events after empty SSE frame: ${JSON.stringify(events)}`);
	}

	let truncatedError;
	try {
		await collect(OpenAI, 'data: {"type":\n\n');
	} catch (error) {
		truncatedError = error;
	}
	if (truncatedError?.message !== "OpenAI SSE frame ended without complete JSON") {
		throw new Error(`${implementation} did not normalize truncated SSE for PI retry: ${truncatedError}`);
	}
}

function sse(data) {
	return `data: ${typeof data === "string" ? data : JSON.stringify(data)}\n\n`;
}

function runPI(configurationDirectory) {
	return new Promise((resolve, reject) => {
		const child = spawn(
			"pi",
			[
				"--mode", "json",
				"--print",
				"--provider", "agent-workspace",
				"--model", "smoke-test",
				"--session-dir", join(configurationDirectory, "sessions"),
				"--no-extensions",
				"--no-skills",
				"--no-prompt-templates",
				"--no-context-files",
				"--",
				"reply OK",
			],
			{
				env: {
					...process.env,
					OPENAI_API_KEY: "smoke-test",
					PI_CODING_AGENT_DIR: configurationDirectory,
					PI_OFFLINE: "1",
				},
				stdio: ["ignore", "pipe", "pipe"],
			},
		);
		let stdout = "";
		let stderr = "";
		const timeout = setTimeout(() => {
			child.kill("SIGTERM");
			reject(new Error(`PI retry smoke timed out: stdout=${stdout} stderr=${stderr}`));
		}, 20_000);
		child.stdout.on("data", (chunk) => { stdout += chunk; });
		child.stderr.on("data", (chunk) => { stderr += chunk; });
		child.on("error", (error) => {
			clearTimeout(timeout);
			reject(error);
		});
		child.on("close", (code) => {
			clearTimeout(timeout);
			resolve({ code, stdout, stderr });
		});
	});
}

const configurationDirectory = await mkdtemp(join(tmpdir(), "pi-sse-smoke-"));
let requestCount = 0;
const completedItem = {
	id: "message-1",
	type: "message",
	status: "completed",
	role: "assistant",
	content: [{ type: "output_text", text: "OK", annotations: [], logprobs: [] }],
};
const completedResponse = {
	id: "response-1",
	status: "completed",
	incomplete_details: null,
	output: [completedItem],
	usage: {
		input_tokens: 1,
		input_tokens_details: { cached_tokens: 0 },
		output_tokens: 1,
		output_tokens_details: { reasoning_tokens: 0 },
		total_tokens: 2,
	},
};
const successfulResponse = [
	sse({
		type: "response.output_item.added",
		sequence_number: 1,
		output_index: 0,
		item: { ...completedItem, status: "in_progress", content: [] },
	}),
	sse({
		type: "response.output_text.delta",
		sequence_number: 2,
		item_id: "message-1",
		output_index: 0,
		content_index: 0,
		delta: "OK",
		logprobs: [],
	}),
	sse({
		type: "response.output_item.done",
		sequence_number: 3,
		output_index: 0,
		item: completedItem,
	}),
	sse({ type: "response.completed", sequence_number: 4, response: completedResponse }),
	sse("[DONE]"),
].join("");
const server = createServer((request, response) => {
	request.resume();
	request.on("end", () => {
		requestCount++;
		const body = requestCount === 1 ? 'data: {"type":\n\n' : successfulResponse;
		response.writeHead(200, {
			"content-type": "text/event-stream",
			"content-length": Buffer.byteLength(body),
		});
		response.end(body);
	});
});

try {
	await new Promise((resolve, reject) => {
		server.once("error", reject);
		server.listen(0, "127.0.0.1", resolve);
	});
	const address = server.address();
	if (typeof address !== "object" || address === null) {
		throw new Error("PI smoke server did not expose a TCP port");
	}
	await mkdir(join(configurationDirectory, "sessions"), { mode: 0o700 });
	await writeFile(
		join(configurationDirectory, "models.json"),
		JSON.stringify({
			providers: {
				"agent-workspace": {
					baseUrl: `http://127.0.0.1:${address.port}/v1`,
					api: "openai-responses",
					apiKey: "$OPENAI_API_KEY",
					models: [{ id: "smoke-test", name: "smoke-test", input: ["text"] }],
				},
			},
		}),
		{ mode: 0o600 },
	);
	const result = await runPI(configurationDirectory);
	if (result.code !== 0 || requestCount !== 2) {
		throw new Error(`PI retry smoke failed: code=${result.code} requests=${requestCount} stderr=${result.stderr}`);
	}
	for (const expected of [
		'"type":"auto_retry_start"',
		'"delta":"OK"',
		'"type":"auto_retry_end","success":true',
		'"stopReason":"stop"',
	]) {
		if (!result.stdout.includes(expected)) {
			throw new Error(`PI retry smoke omitted ${expected}: ${result.stdout}`);
		}
	}
} finally {
	await new Promise((resolve) => server.close(resolve));
	await rm(configurationDirectory, { recursive: true, force: true });
}
