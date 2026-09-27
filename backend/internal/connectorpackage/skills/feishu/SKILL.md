---
name: feishu-cli
display_name: 飞书
description: Use the selected Feishu CLI Connector to find a group, send a message, resolve a person, or create a task.
version: {{VERSION}}
author: Agent Workspace
---

# Feishu CLI in Agent Workspace

Use the reviewed `agent-cli` commands shown in the current execution instruction. The Connector ID is specific to this conversation. Copy it from that instruction. The platform supplies credentials and handles authorization and approval; invoke the CLI through `agent-cli` only.

Use each step only when its capability appears in the current execution instruction. If a capability is unavailable, report that limitation.

## Send a message to a named group

1. Search for the visible group with `im_chat_search`:

   `agent-cli --connector <connector-id> --capability im_chat_search --identity user -- im +chat-search --query '<group name>' --as user`

   Read the JSON result and select the chat whose name matches the user's intended group. Use its `chat_id` (`oc_...`). If there are multiple plausible matches, ask the user to choose before sending.

2. Send the user's exact text with `im_messages_send`:

   `agent-cli --connector <connector-id> --capability im_messages_send --identity user --target <chat_id> -- im +messages-send --chat-id <chat_id> --text '<message>' --as user`

   Quote the group name and message as literal shell arguments, escaping any quote characters in the user's text. This write operation asks the user for a one-use approval. Wait for the result, then report success only if the command succeeds. Do not retry an uncertain send, because it could duplicate the message.

If the user already supplied a stable `chat_id`, start at step 2. If the platform reports missing Feishu scopes, use the conversation's authorization action and continue after the user authorizes. If the broker rejects a command, report its actionable error instead of probing socket paths, source code, or unrelated CLI binaries.

## Create a task

1. Resolve the requested assignee to a Feishu `open_id` (`ou_...`). If the user provided an `open_id`, use it directly. For a name, use `contact_search_user`:

   `agent-cli --connector <connector-id> --capability contact_search_user --identity user -- contact +search-user --query '<name>' --as user`

   Match the returned name and any available department. Ask the user to choose if several people match. For “assign to me”, search with `--user-ids me`; do not infer that the named person is the signed-in user. A task can be created without `--assignee` only when the user explicitly wants an unassigned task.

2. Resolve a relative deadline against the current date in the user's time zone. If “30 日前” leaves the month or year unclear, clarify before creating. Use `YYYY-MM-DD` for an all-day deadline.

3. Create the task with `task_create`, using the user's title and confirmed fields:

   `agent-cli --connector <connector-id> --capability task_create --identity user --target <assignee-open-id> -- task +create --summary '<title>' --assignee <assignee-open-id> --due <YYYY-MM-DD> --idempotency-key <unique-key> --as user`

   Omit `--assignee` and `--target` for an explicitly unassigned task; omit `--due` when no deadline is requested. Use a distinct idempotency key for each task, and reuse it after an uncertain result. This write operation pauses for one-use approval. Report success only when the CLI returns `ok: true`; include `data.guid` and `data.url` when present. If authorization is missing, use the conversation's authorization action and resume after the user grants the required scope. Do not claim a task was created while authorization or approval is pending.
