# Decision Log

## D-001: Manual review only

The system ranks and explains behaviour. It does not automatically ban, kick or punish.

## D-002: External analysis service

DayZ collects live-engine facts and bounded visibility observations. Historical pairwise analysis, matched controls and ranking run in an external Go service.

## D-003: Hidden-awareness behaviour over shot validation

The first operational release prioritises hidden-threat readiness, concealed-sector selection and pre-exposure readiness. Route and squad propagation remain later research because their map, group and communication assumptions are materially weaker. Conventional shot/reflex validation is not the primary value proposition.

## D-004: Real script surfaces only

Every DayZ integration must be verified against `BohemiaInteractive/DayZ-Script-Diff` and proven on the target dedicated-server version in Milestone 0.

## D-005: Strong visibility fails closed

Head-origin occlusion is never camera occlusion in third person. Strong visibility evidence requires a controlled first-person validation ID, repeated occlusion duration, acceptable timing, Tier A geometry and successful calibration. Missing client data cannot increase suspicion.

## D-006: Raw identity boundary

Restricted raw telemetry retains server-required identity for joining and deletion. Normalized and review stores use deterministic pseudonyms. Retention classes are independently configurable and player deletion is auditable.

## D-007: Steam identity with separate admin authorization

Browser administrators authenticate through Steam OpenID 2.0. A successful Steam assertion supplies identity only; authorization is a separate, required SteamID64 allowlist. Browser sessions are short-lived signed cookies, while the existing bearer token remains available for non-browser API clients. The explorer is a module over a two-operation repository interface, with PostgreSQL and memory adapters, so timeline presentation is testable without database or Steam dependencies.

## D-008: Selected DZMap assets inside the review image

The build copies only Chernarus, Livonia, Sakhal and Namalsk assets from a digest-pinned DZMap slim image. A filesystem adapter replaces the HTTP adapter at the existing map seam, so no second runtime container or map network dependency remains. Spatial evidence preserves provenance and precision: exact server positions, untrusted client positions and coarse 100 m cells remain visually distinct. An unavailable captured map fails closed rather than silently plotting coordinates on another terrain.
