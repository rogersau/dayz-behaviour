# Implementation Guardrails

1. Do not add or document an engine hook unless it exists in the target DayZ script version or Milestone 0 proves a replacement.
2. Always call the vanilla/super implementation when extending mission, player or weapon behaviour unless the target method is intentionally terminal and documented.
3. Client camera data is untrusted. Do not build authoritative enforcement on it.
4. Server `PlayerIdentity` attribution wins over any payload identity.
5. Never call `RestContext.POST_now` or another blocking external operation from gameplay code.
6. Never run full player-pair raycasts. Candidate selection and global budgets are mandatory.
7. Treat vegetation and uncertain raycast results as `UNKNOWN` until controlled testing proves otherwise.
8. No automatic bans, kicks, damage changes or other gameplay enforcement.
9. Preserve raw telemetry and algorithm versions so analysis remains reproducible.
10. Feature output must expose raw counts, controls, confidence and uncertainty rather than a purported cheat probability.
