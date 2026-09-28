import { readFileSync, writeFileSync } from "node:fs";

const streamingFiles = [
	"/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/core/streaming.js",
	"/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/openai/core/streaming.mjs",
];
const bundledStreamingFile =
	"/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/dist/bundle/chunks/chunk-NUHFSC37.js";
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
const parseFailureGuards = [
	{
		before: `                        catch (e) {
                            logger.error(\`Could not parse message into JSON:\`, sse.data);
                            logger.error(\`From chunk:\`, sse.raw);
                            throw e;
                        }
`,
		after: `                        catch (e) {
                            // PI retries stream-termination errors but not raw JSON SyntaxErrors.
                            // Normalize only a truncated SSE payload into its retryable stream category.
                            if (e instanceof SyntaxError && e.message === 'Unexpected end of JSON input') {
                                throw new OpenAIError('OpenAI SSE frame ended without complete JSON');
                            }
                            logger.error(\`Could not parse message into JSON:\`, sse.data);
                            logger.error(\`From chunk:\`, sse.raw);
                            throw e;
                        }
`,
	},
	{
		before: `                        catch (e) {
                            console.error(\`Could not parse message into JSON:\`, sse.data);
                            console.error(\`From chunk:\`, sse.raw);
                            throw e;
                        }
`,
		after: `                        catch (e) {
                            if (e instanceof SyntaxError && e.message === 'Unexpected end of JSON input') {
                                throw new OpenAIError('OpenAI SSE frame ended without complete JSON');
                            }
                            console.error(\`Could not parse message into JSON:\`, sse.data);
                            console.error(\`From chunk:\`, sse.raw);
                            throw e;
                        }
`,
	},
];

for (const path of streamingFiles) {
	const source = readFileSync(path, "utf8");
	const occurrences = source.split(parseGuard).length - 1;
	if (occurrences !== 1) {
		throw new Error(`expected one OpenAI SSE parse guard in ${path}, found ${occurrences}`);
	}
	let patched = source.replace(parseGuard, emptyEventGuard);
	for (const { before, after } of parseFailureGuards) {
		const catchOccurrences = patched.split(before).length - 1;
		if (catchOccurrences !== 1) {
			throw new Error(`expected one OpenAI SSE JSON catch in ${path}, found ${catchOccurrences}`);
		}
		patched = patched.replace(before, after);
	}
	writeFileSync(path, patched);
}

const bundleReplacements = [
	{
		before: 'if(sse.data.startsWith("[DONE]")){done=!0;continue}',
		after: 'if(sse.data.startsWith("[DONE]")){done=!0;continue}if(sse.data.trim()===""){continue}',
	},
	{
		before:
			'catch(e){throw logger.error("Could not parse message into JSON:",sse.data),logger.error("From chunk:",sse.raw),e}',
		after:
			'catch(e){if(e instanceof SyntaxError&&e.message==="Unexpected end of JSON input")throw new OpenAIError("OpenAI SSE frame ended without complete JSON");throw logger.error("Could not parse message into JSON:",sse.data),logger.error("From chunk:",sse.raw),e}',
	},
	{
		before:
			'catch(e){throw console.error("Could not parse message into JSON:",sse.data),console.error("From chunk:",sse.raw),e}',
		after:
			'catch(e){if(e instanceof SyntaxError&&e.message==="Unexpected end of JSON input")throw new OpenAIError("OpenAI SSE frame ended without complete JSON");throw console.error("Could not parse message into JSON:",sse.data),console.error("From chunk:",sse.raw),e}',
	},
];

let bundled = readFileSync(bundledStreamingFile, "utf8");
for (const { before, after } of bundleReplacements) {
	const occurrences = bundled.split(before).length - 1;
	if (occurrences !== 1) {
		throw new Error(`expected one bundled OpenAI SSE guard in ${bundledStreamingFile}, found ${occurrences}`);
	}
	bundled = bundled.replace(before, after);
}
writeFileSync(bundledStreamingFile, bundled);
