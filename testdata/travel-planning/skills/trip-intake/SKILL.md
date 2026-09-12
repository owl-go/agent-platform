---
name: trip-intake
display_name: 旅行需求采集与约束建模
version: 1.0.0
description: Turn an unstructured travel request into a concise brief with hard constraints, preferences, and missing information.
description_zh: 将自然语言旅行需求整理为旅行简报，区分硬约束、软偏好和待补信息。
---

# 旅行需求采集与约束建模

Use this Skill when a user is starting or materially changing a travel plan.

## Procedure

1. Extract destination(s), origin, dates, local time zone, travelers, budget, currency, interests, pace, mobility needs, transport and accommodation preferences, and exclusions.
2. Classify each item as a hard constraint, a soft preference, a user assumption, or missing information.
3. Detect conflicts such as too many cities for the available time, budget below the stated transport needs, or activity intensity above the user's walking limit.
4. Ask for missing information in one compact list. Do not ask for information that is not needed for the current planning decision.
5. Produce a travel brief that another planning step can use without interpreting free-form prose.

## Output

Return:

- `confirmed`: facts explicitly supplied by the user;
- `hard_constraints`: conditions that must not be violated;
- `soft_preferences`: preferences that may be traded off;
- `assumptions`: defaults used only when the user accepts them;
- `missing`: information required before a reliable recommendation;
- `conflicts`: detected incompatibilities and their impact.

Never request or retain full passport numbers, payment-card data, or other secrets. Visa or health details should be recorded only at the level needed to identify a verification task.
