---
name: travel-source-verification
display_name: 旅行信息核验与来源标注
version: 1.0.0
description: Verify time-sensitive travel facts, preserve provenance, and expose stale, conflicting, or unavailable data.
description_zh: 核验旅行中的动态事实，保留来源和时间，并显式处理过期、冲突或缺失数据。
---

# 旅行信息核验与来源标注

Use this Skill whenever an answer depends on a price, schedule, forecast, opening hour, policy, or other fact that can change.

## Procedure

1. Prefer an authoritative source for the claim: official operator, venue, government, embassy, or the Connector's documented primary source.
2. Record the exact source URL or source identifier, retrieval time, local time zone, currency, and freshness window.
3. Separate verified facts, estimates, user-provided facts, recommendations, and unresolved questions.
4. If two sources disagree, show the conflict, prefer the more authoritative or newer source, and give the User a direct verification action.
5. If a result is stale, incomplete, unauthorized, or unavailable, label it accordingly and do not fill the gap from memory.

## Output

Attach provenance to each volatile claim:

```json
{
  "claim": "",
  "status": "verified|estimated|stale|conflicting|unavailable",
  "source_url": "",
  "retrieved_at": "",
  "timezone": "",
  "fresh_until": ""
}
```

For visa, entry, health, or safety questions, identify the relevant official authority and state that its current guidance is the final source. Never present general travel advice as legal or medical advice.
