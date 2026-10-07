---
name: travel-planning
display_name: 旅行规划与行程安排
version: 1.0.0
description: Plan practical trips by collecting constraints, optimizing multi-day itineraries, and verifying time-sensitive travel facts with provenance.
description_zh: 采集旅行约束、优化多日行程，并核验带来源和时间的动态旅行信息。
---

# 旅行规划与行程安排

Use this Skill for travel research, route comparison, itinerary creation, budget estimates, and updates to an existing plan. Keep planning, optimization, and source verification in one coherent pass so the final itinerary does not lose its assumptions or evidence.

## 1. Build the travel brief

Extract destination(s), origin, dates, local time zone, travelers, budget, currency, interests, pace, mobility needs, transport and accommodation preferences, and exclusions.

Classify every item as one of:

- `confirmed`: explicitly supplied by the User;
- `hard_constraints`: conditions that must not be violated;
- `soft_preferences`: preferences that may be traded off;
- `assumptions`: defaults used only when the User accepts them;
- `missing`: information required before a reliable recommendation;
- `conflicts`: incompatible requirements and their impact.

Detect conflicts such as too many cities for the available time, a budget below stated transport needs, or activity intensity above the User's walking limit. Ask for missing information in one compact list and do not ask for information that is not needed for the current decision.

Never request or retain full passport numbers, payment-card data, or other secrets. Record visa or health details only at the level needed to identify a verification task.

## 2. Verify travel facts

Use an authoritative source for each volatile claim: an official operator, venue, government, embassy, or the Connector's documented primary source. For every price, schedule, forecast, opening hour, policy, or availability result, record:

- exact source URL or source identifier;
- retrieval time and local time zone;
- currency when money is involved;
- freshness window or `fresh_until` value.

Separate verified facts, estimates, User-provided facts, recommendations, and unresolved questions. If sources disagree, show the conflict, prefer the more authoritative or newer source, and give the User a direct verification action. If a result is stale, incomplete, unauthorized, or unavailable, label it explicitly and never fill the gap from memory.

For visa, entry, health, or safety questions, identify the relevant official authority and state that its current guidance is the final source. Do not present general travel advice as legal or medical advice.

## 3. Optimize the itinerary

1. Group places by area and identify fixed-time commitments, opening hours, closed days, and required reservations.
2. Decide the city order and accommodation areas before filling individual days.
3. Build each day around one geographic cluster. Include realistic travel, queue, meal, rest, and transfer buffers.
4. Check that every activity fits its opening window, transfers are physically possible, and daily intensity respects the User's pace and mobility constraints.
5. Estimate costs in the requested currency and label estimates separately from quoted prices.
6. Create at least one weather or disruption alternative for every day that depends on outdoor conditions or a fixed transport service.
7. Compare alternatives by total time, cost, transfers, energy, weather sensitivity, and cancellation risk, then recommend one.

For each day return the date and local time zone, ordered stops, activity duration, travel mode, distance or duration, transfer buffer, cost estimate and currency, reservation or opening-hour constraint, and backup plan with its activation condition.

Do not claim a booking, ticket, seat, or live availability unless the Connector explicitly returns that state with provenance.

## 4. Produce and update the hand-off

Return a recommendation first, followed by:

1. the normalized travel brief;
2. the city order and trade-offs;
3. the daily itinerary table;
4. the budget summary;
5. the booking and preparation checklist;
6. risks and alternatives;
7. verified facts, estimates, assumptions, and unresolved items;
8. the source list with URL, retrieval time, and applicable date.

When the User changes a date, budget, place, or pace, recalculate only affected sections and explain downstream changes. For a recurring refresh, keep the baseline and write a dated change summary; do not silently overwrite the original plan.

## 5. External action boundary

This Skill is read-only by default. Calendar creation, booking, payment, cancellation, outbound messaging, bulk edits, and any action that can spend money or change external state must first be represented as a draft with the exact target, content, and side effect. Wait for explicit User confirmation before invoking a write-capable Connector.

Connector credentials are private execution secrets and must not appear in tool output, Runtime events, Workspace files, or the final response.
