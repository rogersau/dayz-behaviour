# Admin explorer operations

The admin explorer is a read-only evidence browser layered over normalized PostgreSQL data. It lists pseudonymized player sessions and reconstructs an ordered event timeline containing direct events plus observer/target context. Each entry retains its source authority tier and exposes the normalized payload for manual inspection.

It does not label a player as cheating and cannot send gameplay action to DayZ.

## Configuration

`reviewd` requires:

| Variable | Purpose |
|---|---|
| `DBA_DATABASE_URL` | PostgreSQL connection string |
| `DBA_REVIEW_TOKEN` | Bearer token for scripts and API clients |
| `DBA_PUBLIC_BASE_URL` | Exact public origin, such as `https://behaviour.example.com` |
| `DBA_STEAM_ADMIN_IDS` | Comma-separated SteamID64 allowlist |
| `DBA_SESSION_SECRET` | Random session-signing secret, at least 32 characters |
| `DBA_MAP_DIR` | Optional local map directory; `/app/maps` in the review image |
| `DBA_DEFAULT_MAP` | Map used only when captured events contain no `map_id` |

Steam OpenID verifies the account's 64-bit Steam ID. It does not determine whether that account is an administrator, so a verified account receives access only when it appears in `DBA_STEAM_ADMIN_IDS`. A Steam Web API key is not required because the tool does not retrieve profiles or ownership data.

For local loopback use, `DBA_PUBLIC_BASE_URL=http://127.0.0.1:8082` is sufficient. For any network deployment, terminate HTTPS at a trusted reverse proxy, preserve the external host, and set the public base URL to the exact HTTPS origin. Do not expose PostgreSQL or the ingest sidecar publicly.

## Spatial context

The `review-runtime` image copies only Chernarus, Livonia, Sakhal and Namalsk level-4 assets from a digest-pinned DZMap slim build stage. `reviewd` reads and serves those files itself; there is no DZMap runtime process, network dependency or second map container. Other executables use the map-free default image target.

The map draws:

- exact server snapshot positions as the selected player's route;
- related-player positions in a separate colour;
- Tier B client camera locations in amber;
- visibility `area_cell` evidence as a coarse 100 m cell, never as an exact point.

The captured `map_id` wins over configuration. `DBA_DEFAULT_MAP` is used only when the session contains no map identity. If the requested map is unavailable, the explorer fails closed and does not plot the coordinates on a different map. Tile attribution from the local catalog is displayed below the map.

## Endpoints

- `GET /` — browser explorer;
- `GET /auth/steam/login` and `GET /auth/steam/callback` — Steam OpenID flow;
- `GET /auth/me`, `POST /auth/logout` — browser session operations;
- `GET /v1/explore/sessions?search=&server=&limit=` — session index;
- `GET /v1/explore/timeline?player_session_id=&from_ms=&to_ms=&limit=` — ordered context timeline;
- `GET /v1/map/maps` — local map catalog;
- `GET /v1/map/tiles/{map}/{layer}/{z}/{x}/{y}.webp` — authenticated tile proxy;
- existing `/v1/review-*` endpoints remain available behind the same browser session or bearer token.

The timeline is capped at 2,000 entries per request. API clients can use `from_ms` and `to_ms` to divide larger sessions without dropping ordering context.

## Admin changes

To add or remove an administrator, update `DBA_STEAM_ADMIN_IDS` and restart `reviewd`. Existing cookies belonging to a removed ID stop authorizing immediately after restart because every request is checked against the current allowlist. Rotate `DBA_SESSION_SECRET` to invalidate all browser sessions at once.
