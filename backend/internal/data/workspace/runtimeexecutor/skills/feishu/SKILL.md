---
name: feishu-cli
description: Use the selected Feishu CLI Connector to find a group and send a message.
---

# Feishu CLI in Agent Workspace

Use the reviewed `agent-cli` commands shown in the current execution instruction. The Connector ID is specific to this conversation. Copy it from that instruction. The platform supplies credentials and handles authorization and approval; invoke the CLI through `agent-cli` only.

## Send a message to a named group

1. Search for the visible group with `im_chat_search`:

   `agent-cli --connector <connector-id> --capability im_chat_search --identity user -- im +chat-search --query '<group name>' --as user`

   Read the JSON result and select the chat whose name matches the user's intended group. Use its `chat_id` (`oc_...`). If there are multiple plausible matches, ask the user to choose before sending.

2. Send the user's exact text with `im_messages_send`:

   `agent-cli --connector <connector-id> --capability im_messages_send --identity user --target <chat_id> -- im +messages-send --chat-id <chat_id> --text '<message>' --as user`

   Quote the group name and message as literal shell arguments, escaping any quote characters in the user's text. This write operation asks the user for a one-use approval. Wait for the result, then report success only if the command succeeds. Do not retry an uncertain send, because it could duplicate the message.

If the user already supplied a stable `chat_id`, start at step 2. If the platform reports missing Feishu scopes, use the conversation's authorization action and continue after the user authorizes. If the broker rejects a command, report its actionable error instead of probing socket paths, source code, or unrelated CLI binaries.
