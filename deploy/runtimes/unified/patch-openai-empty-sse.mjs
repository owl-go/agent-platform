import { readFileSync, writeFileSync } from "node:fs";

const streamingFiles = [
	"/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/core/streaming.js",
	"/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/core/streaming.mjs",
];
const parseGuard = `                    if (sse.data.startsWith('[DONE]')) {
                        done = true;
                        continue;
                    }
`;
const emptyEventGuard = `${parseGuard}                    // SSE metadata-only events have no data and are valid per the protocol.
                    if (sse.data.trim() === '') {
                        continue;
                    }
`;

for (const path of streamingFiles) {
	const source = readFileSync(path, "utf8");
	const occurrences = source.split(parseGuard).length - 1;
	if (occurrences !== 1) {
		throw new Error(`expected one OpenAI SSE parse guard in ${path}, found ${occurrences}`);
	}
	writeFileSync(path, source.replace(parseGuard, emptyEventGuard));
}
