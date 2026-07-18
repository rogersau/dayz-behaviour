# Project Scope

DayZ Behaviour collects bounded client and server telemetry and uses an external Go service to identify repeated hidden-awareness outliers for manual administrator review.

## In scope

- client camera and combat-state sampling;
- authoritative server snapshots and combat events;
- bounded live-world visibility classification;
- asynchronous telemetry export and disk spool;
- external timeline reconstruction, matched controls and explainable outlier ranking;
- individual and squad-level review evidence.

## Out of scope

- automatic enforcement;
- proving the presence of DMA hardware;
- exact recreation of rendered perception or audio;
- full-population per-frame pair analysis;
- black-box machine learning in the MVP;
- replacing conventional anti-cheat systems.
