---
name: feishu-cli
display_name: 飞书
description: Route Feishu messages, contacts, tasks, calendars, documents, files, tables, mail, meetings, and other official CLI domains through reviewed Connector capabilities.
version: {{VERSION}}
author: Agent Workspace
---

# Feishu CLI in Agent Workspace

This Skill is the entry point for the complete **documented** `@larksuite/cli@{{VERSION}}` feature set. Its pinned official references are under `references/`. They explain Feishu products and command syntax; the current execution instruction and `cli.json` are the source of truth for what this Connector revision may execute.

1. Match the user's intent to one domain below. Read its `SKILL.md` and the relevant linked reference before choosing arguments. Read [shared CLI conventions](references/lark-shared/SKILL.md) for output and parameter conventions. The platform, rather than the CLI's interactive instructions, handles application setup, login, OAuth, and approval.
2. Find an exact capability in the current execution instruction whose argv prefix matches the chosen command. Copy the conversation's Connector ID and invoke only `agent-cli --connector <connector-id> --capability <capability-id> --identity <reviewed-identity> [--target <target>] -- <reviewed-prefix> <documented-arguments>`. Pass user text as quoted literal arguments; never construct an unreviewed `lark-cli` or raw API call. A domain's presence in this index does not grant its commands.
3. If no matching capability exists, say that the upstream CLI documents the operation but this Connector revision has not made it available. If a scope is missing, use the conversation's authorization action. For high-risk writes, wait for the platform's one-use approval. Report success only from a successful CLI result; after an uncertain write, use the command's documented idempotency mechanism or inspect state before retrying.

## Official CLI domain index

| User intent | Pinned official reference |
| --- | --- |
| Messages, groups, media, reactions | [IM](references/lark-im/SKILL.md) |
| People and user profiles | [Contact](references/lark-contact/SKILL.md) |
| Tasks, tasklists, subtasks, assignments | [Task](references/lark-task/SKILL.md) |
| Calendar, events, availability, rooms | [Calendar](references/lark-calendar/SKILL.md) |
| Documents and document content | [Doc](references/lark-doc/SKILL.md) |
| Drive files, permissions, comments | [Drive](references/lark-drive/SKILL.md) |
| Native Markdown files | [Markdown](references/lark-markdown/SKILL.md) |
| Knowledge spaces and nodes | [Wiki](references/lark-wiki/SKILL.md) |
| Whiteboard content | [Whiteboard](references/lark-whiteboard/SKILL.md) |
| Notes | [Note](references/lark-note/SKILL.md) |
| Multidimensional tables | [Base](references/lark-base/SKILL.md) |
| Spreadsheets | [Sheets](references/lark-sheets/SKILL.md) |
| Presentations | [Slides](references/lark-slides/SKILL.md) |
| Email and drafts | [Mail](references/lark-mail/SKILL.md) |
| Meetings and participants | [Meeting](references/lark-meeting/SKILL.md) |
| Minutes and transcripts | [Minutes](references/lark-minutes/SKILL.md) |
| Video meetings | [VC](references/lark-vc/SKILL.md) |
| In-meeting assistant | [VC Agent](references/lark-vc-agent/SKILL.md) |
| Attendance | [Attendance](references/lark-attendance/SKILL.md) |
| Approvals | [Approval](references/lark-approval/SKILL.md) |
| Goals and key results | [OKR](references/lark-okr/SKILL.md) |
| Realtime events | [Event](references/lark-event/SKILL.md) |
| App creation and publishing | [Apps](references/lark-apps/SKILL.md) |
| OpenAPI command discovery | [OpenAPI Explorer](references/lark-openapi-explorer/SKILL.md) |
| Skill authoring | [Skill Maker](references/lark-skill-maker/SKILL.md) |
| Meeting summary workflow | [Meeting Summary](references/lark-workflow-meeting-summary/SKILL.md) |
| Standup report workflow | [Standup Report](references/lark-workflow-standup-report/SKILL.md) |

The upstream references describe more operations than this Connector currently reviews. For every operation, the exact capability check in step 2 decides whether the agent may proceed. The quick paths below cover common approved commands without an extra documentation lookup.

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
