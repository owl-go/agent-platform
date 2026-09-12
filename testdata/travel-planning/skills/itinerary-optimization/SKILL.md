---
name: itinerary-optimization
display_name: 多日行程优化
version: 1.0.0
description: Build practical multi-day itineraries by balancing geography, opening hours, transport, cost, energy, and contingency time.
description_zh: 在地理顺路、营业时间、交通、预算、体力和备用时间之间优化多日行程。
---

# 多日行程优化

Use this Skill after a travel brief has enough confirmed constraints to compare or build routes.

## Procedure

1. Group places by area and identify fixed-time commitments, opening hours, closed days, and required reservations.
2. Decide the city order and accommodation areas before filling individual days.
3. Build each day around one geographic cluster. Include realistic travel, queue, meal, rest, and transfer buffers.
4. Check that every activity fits its opening window, that transfers are physically possible, and that the daily intensity respects the user's pace and mobility constraints.
5. Estimate costs in the requested currency and label all estimates separately from quoted prices.
6. Create at least one weather or disruption alternative for every day that depends on outdoor conditions or a fixed transport service.
7. Compare alternatives by total time, cost, transfers, energy, weather sensitivity, and cancellation risk, then recommend one.

## Output

For each day return:

- date and local time zone;
- ordered stops and activity duration;
- travel mode, distance or duration, and transfer buffer;
- cost estimate and currency;
- reservation or opening-hour constraint;
- backup plan and the condition that activates it.

Do not claim a booking, ticket, seat, or live availability unless the Connector explicitly returns that state with provenance.
