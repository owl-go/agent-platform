import OpenAI from "/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/index.mjs";

const responseBody = [
	"data:\n\n",
	'data: {"type":"response.created","response":{"id":"response-1","status":"in_progress","output":[]}}\n\n',
	"data: [DONE]\n\n",
].join("");

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

if (events.length !== 1 || events[0]?.type !== "response.created") {
	throw new Error(`unexpected events after empty SSE frame: ${JSON.stringify(events)}`);
}
