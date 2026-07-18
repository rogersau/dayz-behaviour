# Decision Log

## D-001: Manual review only

The system ranks and explains behaviour. It does not automatically ban, kick or punish.

## D-002: External analysis service

DayZ collects live-engine facts and bounded visibility observations. Historical pairwise analysis, matched controls and ranking run in an external Go service.

## D-003: DMA/radar behaviour over shot validation

The MVP prioritises hidden-threat readiness, sector selection, pre-exposure behaviour, route convergence/avoidance and squad information propagation. Conventional shot/reflex validation is not the primary value proposition.

## D-004: Real script surfaces only

Every DayZ integration must be verified against `BohemiaInteractive/DayZ-Script-Diff` and proven on the target dedicated-server version in Milestone 0.
