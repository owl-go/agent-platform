# Credits and Usage

## Problem Statement

Agent Workspace currently lets every authenticated User start Session responses and Workflow Runs without a product-level usage allowance. Users cannot see how much platform execution a model invocation consumed, redeem additional allowance, or understand why a later execution should be blocked. Administrators cannot configure per-User daily allowance, manage model-specific consumption rates, issue Redemption Codes, or make an auditable correction.

The Runtime contract can carry input Tokens, output Tokens, and optional provider cost, but the current execution path discards that Usage before it reaches the Workspace application, persistence, API, or UI. Usage reporting also differs between Runtime Engines: some report per-invocation Tokens, some may report cumulative native-session values, and some report no Tokens. A parser recognizing a Usage field is not conformance evidence for a Runtime image.

The product needs an anti-abuse entitlement mechanism rather than monetary Provider billing. Users continue to supply and pay for their own Model Provider Connections. Credits govern access to Agent Workspace model execution, must remain explainable under retries and concurrency, and must preserve the existing rule that an Administrator cannot inspect User-owned conversations, Workflows, models, or execution-level consumption.

## Solution

Introduce an independent Credits bounded context that owns each User's Credit Ledger, Credit Balance projections, Daily Credit Allocation, Redeemed Credit Balance, Redemption Codes, Model Credit Rate revisions, and reasoned Credit Adjustments. Account supplies User identity, while Workspace requests admission and settlement through Credits application ports rather than directly updating balances.

Every User receives a default Daily Credit Allocation of 600 Credits. Daily Credits reset at midnight in the User's effective Personal Settings time zone, do not carry forward, and are consumed before persistent redeemed Credits. At a Model Credit Rate of 1.00, one Credit represents 10,000 input or output Tokens. Each actual model invocation freezes a versioned input multiplier, output multiplier, and missing-Usage fallback, then settles its current-invocation Token delta to two decimal places. The default multipliers are 1.00 and the default missing-Usage fallback is 10.00 Credits.

A positive Credit Balance is required immediately before a model invocation starts. Model invocations are serialized per User, but final Usage is settled after execution and may make the balance negative. The negative amount carries forward and blocks further model invocation until a later Daily Credit Allocation, Redemption Code, or Credit Adjustment covers it. Each Expert Team stage admits and settles independently; completed stages remain charged even if a later stage cannot start or the overall turn fails.

Users see their total balance in the avatar menu, can redeem a code, inspect their own Credit Ledger, and see a collapsed consumption summary on every terminal Assistant response and Workflow Run. Administrators manage Users, Model Credit Rates, and Redemption Codes from three tabs in User Management without gaining access to User-owned execution details.

## User Stories

1. As a User, I want to receive a daily Credit allowance, so that I can use Agent Workspace without arranging a separate payment.
2. As a User, I want the default daily allowance to be 600 Credits, so that I begin each day with a predictable execution entitlement.
3. As a User, I want unused daily Credits to expire at the end of my Credit Day, so that the allowance behaves as a daily anti-abuse control rather than an accumulating asset.
4. As a User, I want my Credit Day to follow my configured time zone, so that the reset occurs at my local midnight.
5. As a User, I want a time-zone change to take effect only at the next Credit Day boundary, so that changing settings cannot restore my allowance twice.
6. As a newly created User, I want the current day's full allocation immediately, so that I can start using the product without waiting until midnight.
7. As an existing User at rollout, I want the current day's full allocation, so that enabling Credits does not unexpectedly lock me out.
8. As a User, I want to enter a Redemption Code, so that I can obtain additional Credits after using my daily allocation.
9. As a User, I want redeemed Credits to persist across Credit Days, so that a code retains value until I consume it.
10. As a User, I want daily Credits to be consumed before redeemed Credits, so that expiring allowance is not wasted while persistent Credits are available.
11. As a User, I want a successful redemption to show my new Credit Balance, so that I know the code was applied.
12. As a User, I want invalid, expired, void, and already-used codes to return the same unavailable message, so that another User's redemption activity is not exposed.
13. As a User, I want concurrent redemption attempts for one code to produce only one success, so that a code cannot be duplicated by racing requests.
14. As a User, I want to see my current total Credit Balance in the avatar menu, so that I can judge whether I can start more work.
15. As a User, I want the balance row to open a Credit panel, so that the global navigation remains compact.
16. As a User, I want the Credit panel to separate today's remaining allocation from redeemed Credits, so that I understand what expires.
17. As a User, I want to see today's consumption and the next allocation time, so that I can plan additional executions.
18. As a User, I want to read my complete Credit Ledger, so that every balance change is explainable.
19. As a User, I want ledger entries for daily allocation, redemption, adjustment, and execution consumption, so that different sources are distinguishable.
20. As a User, I want every terminal Assistant response to show its total Credit Consumption, so that I can relate usage to a response.
21. As a User, I want every terminal Workflow Run turn to show its total Credit Consumption, so that Workflow execution is as transparent as Session execution.
22. As a User, I want an Expert Team's displayed total to include every member invocation, so that multi-stage work does not hide consumption.
23. As a User, I want each terminal response to show only its total Credit Consumption, so that the conversation stays compact and does not expose Token accounting detail.
24. As a User, I want fallback accounting to remain an internal settlement detail, so that an estimated charge is not presented as precise Token measurement in the conversation.
25. As a User, I want one Credit at multiplier 1.00 to represent 10,000 Tokens, so that the consumption scale is stable and understandable.
26. As a User, I want input and output Tokens to have independent multipliers, so that Model Credit Rates can reflect their different usage profiles.
27. As a User, I want each invocation rounded to two decimal places with a minimum non-zero charge of 0.01 Credits, so that small real usage remains visible and deterministic.
28. As a User, I want only the current invocation's Token delta charged, so that resumed native-session history is not charged repeatedly.
29. As a User, I want model work that consumed Tokens to be charged even when it later fails, is cancelled, or times out, so that the allowance reflects actual platform usage.
30. As a User, I want failures before any Provider Model invocation to remain uncharged, so that validation and infrastructure preparation do not consume Credits.
31. As a User, I want a retry to have its own consumption record, so that repeated Provider Model usage is not hidden.
32. As a User, I want MCP testing, queueing, and Runtime preparation to remain uncharged, so that only actual model invocation consumes Credits.
33. As a User, I want execution to be rejected when my balance is not positive, so that I cannot unknowingly start work without entitlement.
34. As a User, I want one admitted invocation to finish and record its actual Usage even if it makes my balance negative, so that the product does not invent a partial charge.
35. As a User, I want negative Credit Balance to carry forward, so that crossing a Credit Day does not forgive excess consumption.
36. As a User, I want later daily or redeemed Credits to offset negative balance, so that I can regain access without manual data repair.
37. As a User, I want my model invocations serialized and rechecked immediately before start, so that concurrent requests cannot multiply an overdraft.
38. As a User running an Expert Team, I want each member to recheck balance before invoking its model, so that an earlier member can exhaust Credits before later work starts.
39. As a User running an Expert Team, I want completed members to remain charged when a later member cannot start, so that recorded consumption matches actual work.
40. As a Workflow owner, I want temporary Workspace and staged Native Session changes discarded when a later Expert Team member cannot start, so that failed execution preserves the existing rollback contract.
41. As an interactive User, I want insufficient Credits reported before a Session response or manual Workflow execution record is created, so that my history is not cluttered by work that never started.
42. As a Workflow API caller, I want a stable insufficient-credits error with the current balance and next allocation time, so that my integration can decide when to retry.
43. As a Workflow API caller, I want insufficient Credits represented as HTTP 429, so that quota exhaustion is distinguishable from validation and server failure.
44. As a Workflow owner, I want an insufficient-credit Scheduled Trigger to create an uncharged failed Run, so that missed automation remains auditable.
45. As an Administrator, I want to configure a User-specific Daily Credit Allocation, so that different Users can receive different daily entitlement.
46. As an Administrator, I want an allocation change to take effect on the next Credit Day, so that today's balance is not retroactively rewritten.
47. As an Administrator, I want an allocation of zero to disable future daily Credits, so that I can stop recurring entitlement without disabling the account.
48. As an Administrator, I want to see each User's balance, today's consumption, and configured daily allocation, so that I can manage account-level usage.
49. As an Administrator, I want to add or subtract persistent Credits with a required reason, so that corrections and compensation remain auditable.
50. As an Administrator, I want my own executions and adjustments to follow the same rules, so that there is no hidden unlimited account.
51. As an Administrator, I want to create Model Credit Rate revisions keyed by Provider type, API Protocol, and exact Model identifier, so that matching does not require access to User-owned connections.
52. As an Administrator, I want rate changes to affect only subsequently queued invocations, so that historical and active execution costs remain stable.
53. As an Administrator, I want an unmatched model to use a versioned platform default rate, so that a missing rule never creates accidental free usage.
54. As an Administrator, I want to configure an explicit zero multiplier or fallback, so that making selected usage free is an intentional audited policy.
55. As an Administrator, I want to generate one to one hundred globally single-use Redemption Codes in a batch, so that I can distribute Credits efficiently.
56. As an Administrator, I want a batch to share a fixed Credit value and optional expiry, so that distribution policy is consistent.
57. As an Administrator, I want generated code plaintext shown only once and available for copy or CSV download, so that the platform does not retain recoverable bearer codes.
58. As an Administrator, I want to void an unused Redemption Code, so that a leaked or obsolete code can be disabled.
59. As an Administrator, I want to inspect redemption and adjustment records without seeing execution-level Session, Workflow, model, or Token details, so that I can support accounts without accessing private content.
60. As an operator, I want daily allocation, redemption, adjustment, and settlement operations to be idempotent, so that retries cannot duplicate Credits or charges.
61. As an operator, I want terminal execution state and Credit settlement committed atomically, so that a result cannot exist without its corresponding charge or vice versa.
62. As an operator, I want Credit Ledger records retained for the life of the User, so that every balance projection can be reconstructed and audited.
63. As an operator, I want Runtime-reported Token values used only as anti-abuse accounting unless conformance exists, so that the product does not make unsupported precision claims.
64. As an operator, I want a fixed fallback when a Runtime reports no Token values, so that missing Usage cannot become a free execution path.

## Implementation Decisions

- Credits is a new independent bounded context. It owns Credit Ledger entries, Credit Balance projections, Daily Credit Allocations, Redeemed Credit Balance, Redemption Codes, Model Credit Rate revisions, Credit Adjustments, admission, and settlement.
- Account remains responsible for OIDC identity and account lifecycle. It exposes User identity to Credits but does not own or mutate Credit state.
- Workspace remains responsible for Sessions, Workflows, Runs, snapshots, and execution. It calls Credits application ports for stage admission and settlement instead of accessing Credit persistence directly.
- Credits and Workspace may share PostgreSQL, but Domain and Application interfaces must not expose GORM models, HTTP DTOs, or database-specific types.
- Credit amounts are represented as integer hundredths at persistence and domain boundaries. Floating-point arithmetic is not used for balance, ledger, adjustment, redemption, or final consumption.
- Rate multipliers use an exact non-negative decimal representation. The calculation divides input and output Token counts by 10,000, applies their frozen multipliers, adds the results, rounds half-up once per invocation to two decimal places, and applies a minimum non-zero charge of 0.01 Credits.
- The platform default Model Credit Rate has input multiplier 1.00, output multiplier 1.00, and missing-Usage fallback 10.00 Credits.
- An exact Model Credit Rate matches Provider type, Model API Protocol, and exact Provider Model identifier. User identity, connection name, Endpoint, and Runtime Engine are not match dimensions.
- A Model Credit Rate update creates an immutable revision. Every Execution Stage Snapshot stores the resolved revision identity and values so later edits cannot change queued or historical consumption.
- An Administrator may intentionally configure zero input multiplier, output multiplier, or fallback. Only an explicit matching or default revision can make usage free.
- Runtime-reported provider cost is not used in Credit calculations. Credits are an Agent Workspace execution entitlement and not a pass-through Provider invoice.
- Runtime Usage must reach the Workspace application as per-invocation input and output Token counts plus a flag indicating whether fallback was required.
- Runtime Adapters normalize cumulative native-session Usage to the delta for the current invocation. Existing Driver parsing is not itself proof of a verified Runtime Capability.
- When both Token counts are absent, the frozen missing-Usage fallback is charged. When either count is present, absent individual counts are treated as zero only if the Runtime contract can distinguish zero from missing; otherwise the whole invocation uses fallback.
- A single Expert or no-Expert response has one Execution Stage. An Expert Team has one ordered Stage per member, and each Stage freezes and settles its own model rate.
- Each User has at most one actively executing credit-consuming model invocation. Waiting Session, Workflow, and Expert Team stages re-run admission immediately before model invocation.
- Admission transactionally materializes the Daily Credit Allocation for the current Credit Day if necessary and requires total Credit Balance greater than zero.
- The Credit Day is pinned when a model invocation starts. Completion after midnight settles against that pinned day.
- A Personal Settings time-zone change becomes effective for Credits at the next Credit Day boundary. The current Credit Day retains its original zone and boundary.
- Daily allocation is materialized lazily on the first balance read or admission after a boundary. A unique User and Credit Day identity prevents duplicate materialization.
- Daily Credits do not carry forward. Redeemed Credits and Credit Adjustments affect a persistent balance. Consumption uses remaining daily Credits first, then persistent Credits, and any excess becomes persistent negative balance.
- New Users receive the full current-day allocation when created. The rollout migration gives every existing User the current day's 600 Credits and initializes redeemed balance to zero.
- The Credit Ledger is append-only and records allocation, redemption, adjustment, and consumption with stable source identities, signed amounts, resulting balance data, timestamps, and required metadata.
- Balance, current daily remainder, and today's consumption are transactionally maintained projections of the Credit Ledger and cannot be directly overwritten.
- A Credit Adjustment requires a non-empty Administrator reason and creates an immutable ledger entry. Administrators may adjust their own account under the same audit rules.
- A Redemption Code is a cryptographically random bearer value with a fixed positive two-decimal Credit value, optional expiry, and active, redeemed, or void lifecycle.
- Redemption Code plaintext is returned only by the creation response. Persistence stores a one-way verifier and non-secret display identifier; plaintext is excluded from logs, events, later API responses, and ordinary browser storage.
- Redemption atomically verifies state, marks the code redeemed, records the redeeming User and time, appends the ledger grant, and updates projections. The code's globally unique redemption transition prevents concurrent reuse.
- Administrator batch generation accepts between one and one hundred codes with the same value and optional expiry. The one-time response supports direct copy and client-generated CSV download.
- Invalid, expired, void, malformed, and redeemed codes share one public unavailable error. The response does not expose the previous redeemer or code lifecycle state.
- Each consumption settlement has a unique source composed from execution identity and Execution Stage position. Replaying a Worker completion returns the original settlement without another debit.
- An Execution Stage terminal state, its Credit Ledger entry, and balance projections commit in one PostgreSQL transaction. A single-stage execution or final Expert Team Stage also commits the Assistant Message or Run terminal state in that transaction.
- Successful, failed, cancelled, and timed-out stages charge reported Usage or fallback if Provider Model invocation began. Failures before Provider Model invocation create no consumption entry.
- Expert Team stages settle independently. A later insufficient-credit failure preserves earlier charges while following existing rollback rules for temporary Workspace and staged Native Session state.
- Interactive insufficient-credit requests fail without creating a new Assistant execution or manual Run. Workflow API requests expose the stable code insufficient_credits as HTTP 429 with current balance and next allocation time.
- Scheduled Trigger admission failure creates a terminal failed Run with an insufficient-credit reason and no Credit Consumption.
- The public User credit summary includes total balance, current Daily Credit Allocation, remaining daily Credits, redeemed or persistent balance, today's consumed Credits, Credit Day, and next allocation time.
- The User ledger API is owner-scoped, paginated, newest-first, and exposes safe entry type, amount, balance, time, and User-visible execution reference where applicable.
- Administrator User metadata includes balance, today's usage, configured next-day allocation, and effective date. It never includes execution-level ledger entries, model identifiers, Token counts, Session IDs, Workflow IDs, prompts, or results.
- Administrator APIs provide User allocation update, reasoned adjustment, Model Credit Rate listing and revision, Redemption Code batch creation, safe code status listing, and voiding.
- Current User, Session Message, Run, and Expert Stage API representations gain the Credit fields needed by their respective interfaces. Terminal SSE snapshots include final Credit Consumption only after settlement commits.
- The avatar menu gains a compact Credit Balance row. Activating it opens the Credit panel containing balance breakdown, next allocation time, redemption form, and paginated Credit Ledger.
- Terminal Session and Run messages show a compact localized total Credit Consumption only. Per-stage Tokens, multipliers, rate revisions, and fallback details remain outside the conversation interface.
- User Management retains its existing entry from the avatar menu and gains Users, Model Rates, and Redemption Codes tabs. It does not add a fifth primary navigation item.
- All new visible text, errors, amounts, dates, and accessibility labels are localized in Chinese and English. Desktop and mobile preserve the same information order and core actions.
- Credit Ledger, Redemption Code records, Credit Adjustments, and Model Credit Rate revisions are retained for the life of the User and cannot be deleted in the first version.
- The feature must be introduced with immutable additive database migrations. Existing migrations are not edited.
- Existing Runtime Usage fields are not advertised as conformance evidence. Any future verified Usage Capability still requires exact-image black-box evidence under the existing Runtime rules.

## Testing Decisions

- Tests assert public behavior and domain outcomes rather than SQL statement order, private helper calls, Vue component internals, or implementation-specific event sequences.
- The primary acceptance seam is a real browser through the HTTP API and PostgreSQL with a deterministic Runtime test implementation. It covers Administrator configuration, code creation, User redemption, balance display, Session and Workflow consumption, Expert Team stage totals, insufficient-credit behavior, Chinese and English, and mobile layout.
- A Credits Application contract suite is the main deterministic backend seam. It runs through public Credits use cases against the PostgreSQL Adapter with a controlled clock and stable User identities.
- The Credits Application suite covers default and overridden daily allocation, zero allocation, lazy materialization, uniqueness, time-zone rollover, a time-zone change, cross-midnight execution, expiration of unused daily Credits, redemption persistence, debit order, negative carry-forward, and rollout initialization.
- The same suite covers exact rate matching, default rate fallback, explicit zero rates, immutable revisions, snapshot stability, 10,000-Token conversion, independent input and output multipliers, half-up rounding, minimum non-zero charge, and fixed missing-Usage charge.
- The same suite covers concurrent Redemption Code use, expiry, voiding, malformed and generic unavailable errors, batch limits, one-time plaintext, adjustment reasons, Administrator self-adjustment, ledger pagination, and projection reconstruction.
- The same suite covers positive-balance admission, User-level serialization, admission recheck, idempotent settlement, Worker replay, negative balance, and the atomic relationship between terminal Stage state, ledger entry, and projections.
- Workspace Worker tests use deterministic Executor and Credits fakes at existing application ports to cover successful, failed, cancelled, and timed-out Usage settlement without invoking external models.
- Workspace Worker tests cover no-Expert, single Expert, and mixed-model or mixed-Runtime Expert Team execution. They assert per-Stage admission, earlier-stage charging, later-stage exhaustion, overall failure, and Workspace and Native Session rollback.
- Runtime Driver tests continue to verify each CLI's supported Usage parsing. They include true zero, absent values, malformed values, cumulative native-session data, and current-invocation normalization.
- Shared Runtime Adapter and Executor tests verify that Usage crosses the Runtime boundary once, secrets never enter Usage or Credit records, provider cost is ignored, and absent Token counts select the frozen fallback.
- HTTP service tests cover owner authorization, Administrator authorization, request validation, generic Redemption Code errors, one-time plaintext responses, Credit response fields, pagination, and 429 insufficient_credits mapping.
- HTTP tests verify that insufficient-credit responses expose only the requesting User's balance and next allocation time and never expose internal rate data or another account.
- Persistence integration tests use real PostgreSQL to exercise row locking, unique daily allocation, unique redemption, unique Stage settlement, concurrent admission, transaction rollback, and immutable history constraints.
- Migration tests verify additive schema application on the current baseline and initialization of existing Users without modifying historical migrations.
- Frontend Vitest tests extend the existing App, API client, Sessions, Workflow detail, and User Management test patterns. They assert observable text, state, actions, API requests, accessibility roles, and responsive information order.
- Frontend tests cover the compact avatar balance row, Credit panel breakdown, redemption success and generic failure, ledger pagination, terminal total-consumption summary, Administrator tabs, rate revision forms, batch generation, safe code status, voiding, and reasoned adjustments.
- Frontend tests cover negative, zero, and positive balances; disabled submission; queued work that later fails admission; failed and cancelled charged stages; and totals with two decimal places.
- Security tests use canary Redemption Codes and existing secret-scanning patterns to verify that plaintext codes do not enter logs, later API reads, SSE, persisted ordinary DTOs, Workspace files, Artifacts, or error messages.
- The browser-to-API acceptance flow is the required feature evidence. Unit and component tests alone do not satisfy the product acceptance boundary.
- Targeted Credits, Workspace Worker, Runtime Adapter, HTTP service, and frontend tests run first. The full backend test and build gates, web typecheck, web production build, and diff check run afterward.
- Runtime image conformance is required only for claims of verified Usage Capability. Anti-abuse fallback behavior must be tested even when no such evidence is available.
- Prior art includes the existing Object Store conformance suite, Runtime Adapter contract tests, Workspace Worker and Runtime Executor fakes, HTTP test server tests, frontend page-level Vitest tests, and browser OIDC acceptance flow.

## Out of Scope

- Selling Credits for money, payment processing, invoices, taxes, refunds, chargebacks, or financial accounting.
- Paying Model Providers on behalf of Users or converting provider-reported USD cost into Credits.
- Exchanging Credits for goods, subscriptions, memberships, or non-execution benefits.
- Organization, Team, pooled, transferable, or shared Credit Balance.
- Reusable campaign codes, per-User reusable codes, referral codes, promotional rules, or externally generated redemption systems.
- User-authored Model Credit Rates or rates attached to private Model Provider Connections.
- Administrator access to User-owned Provider Connections, Sessions, Workflows, prompts, results, execution-level Credit Consumption, models, or Token details.
- Preauthorizing a maximum Token amount, reserving an estimated maximum charge, terminating a model stream exactly when Credits reach zero, or forgiving excess charge.
- Concurrent credit-consuming model invocation for one User.
- Low-balance email, Toast, webhook, or configurable threshold notifications.
- Deleting, rewriting, or retroactively recalculating Credit Ledger entries or historical consumption.
- Backfilling historical Session and Run Usage from before the Credits rollout.
- Claiming Runtime Usage accuracy or Capability conformance without exact-image production evidence.
- Advanced billing analytics, Administrator execution-level reports, financial exports, or external accounting integrations.
- Adding a Credits item to the four-entry primary product navigation.

## Further Notes

- The Credits product amendment and ADR-0024 are accepted design, not current implementation. Completion reports must distinguish implemented behavior, locally executed tests, skipped environment gates, and exact Runtime evidence.
- The current codebase has no Credit schema, domain module, API, or UI. Runtime Usage exists only in the low-level contract and selected Drivers, is not enabled as a verified Capability, and is currently dropped before the Workspace application result.
- OpenClaw currently has no Token Usage parsing and therefore exercises the fixed fallback path unless its Driver and exact image later gain evidence-backed support.
- The Credit mechanism is deliberately an anti-abuse entitlement. The UI should use Credits and consumption language, not price, payment, bill, cost in currency, or Provider reimbursement.
- The reference presentation is a compact total such as 共消耗 ✧ 79.05 beneath a terminal response and an avatar-menu row labeled 积分余额 with the current amount and a disclosure affordance.
- Publishing this specification to GitHub Issues was explicitly skipped. The repository copy is the authoritative output of the to-spec synthesis.
