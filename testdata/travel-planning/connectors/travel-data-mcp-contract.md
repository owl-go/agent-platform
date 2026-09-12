# Travel Data MCP Contract

This is a contract for a private, read-only MCP Connector used by the Travel Planning Expert. It is not a provider-specific implementation. The Connector may aggregate multiple providers, but it must preserve provenance for every volatile result.

## Tools

### `search_places`

Input: `query`, `city`, optional `category`, optional `limit`.

Return each place with `place_id`, `name`, `location`, `category`, `source_url`, and `retrieved_at`.

### `get_place_details`

Input: `place_id`.

Return `name`, `address`, `timezone`, `opening_hours`, `closed_dates`, `ticket_notes`, `source_url`, `retrieved_at`, and `fresh_until`.

### `get_route`

Input: `origin`, `destination`, `mode`, optional `departure_time`, optional `arrival_time`.

Return one or more options with `duration_minutes`, `distance_meters`, `transfers`, `estimated_cost`, `currency`, `source_url`, `retrieved_at`, and `fresh_until`.

### `get_weather`

Input: `location`, `start_date`, `end_date`.

Return local-time forecast intervals with `timezone`, `condition`, `temperature`, `precipitation_probability`, `source_url`, `retrieved_at`, and `fresh_until`.

### `search_transit`

Input: `origin`, `destination`, `departure_time`, optional `arrival_time`, optional `passengers`.

Return `service`, `departure`, `arrival`, `duration_minutes`, `transfers`, `estimated_cost`, `currency`, `availability_status`, `source_url`, and `retrieved_at`.

### `get_exchange_rate`

Input: `base_currency`, `quote_currency`, optional `at_time`.

Return `rate`, `as_of`, `source_url`, and `retrieved_at`.

## Safety and freshness

- This Connector is read-only. It must not create bookings, modify calendars, send messages, or spend money.
- Every price, schedule, weather, opening-hour, policy, or availability result must include provenance. Missing provenance means `unavailable` for planning purposes.
- The Connector must expose stale data explicitly instead of silently refreshing from an unknown source.
- Provider errors should be returned as structured `unavailable` results with a short diagnostic; do not fabricate empty success responses.
- Credentials, if any, are private Connector secrets and must never appear in tool output, Runtime events, Workspace files, or final responses.
- Use HTTPS Streamable HTTP or a fixed-version `npx`/`uvx` MCP transport accepted by Agent Workspace. Do not use `latest`, arbitrary host commands, or unreviewed write capabilities.
