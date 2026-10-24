# Diagnostics

`status`, `diagnose`, `report` and `explain` share the collection and rule pipeline. `--pid` focuses process collection; `--service` replaces service listing with unit properties and recent journal entries. Other system observations remain available for correlation. Disabled collectors apply to aggregate reports; explicit focused commands request their named collector directly.

## Measurements

Aggregate CPU sums user, nice, system, idle, iowait, IRQ, softirq and steal counters. Guest counters are excluded to avoid double counting. Busy utilization excludes idle and iowait, and uses differences over `sample_interval`; counter resets and zero deltas are errors. High load plus high sampled CPU yields a probable sustained-contention finding, not proof of CPU-only demand. Load includes uninterruptible I/O tasks.

Memory uses MemTotal and MemAvailable, not MemFree alone. Swap occupancy is reported separately and does not establish active thrashing. Disk capacity uses blocks available to ordinary users; inode exhaustion uses free/total inode counts. Pseudo filesystems are skipped. `statfs` errors appear as warnings.

Processes expose RSS bytes, threads, FD count, cumulative user+system CPU ticks and sampled CPU percentage. CPU percentage is the process tick delta divided by aggregate tick delta, multiplied by logical CPU count and 100; a multithreaded process may exceed 100%. Start-time comparison rejects PID reuse. Unavailable samples omit CPU percentage. FD count `-1` means inaccessible. Processes that exit during collection may disappear or produce warnings.

## Rules

| Rule | Evidence and confidence |
| --- | --- |
| Memory pressure | Confirmed low MemAvailable relative to configured threshold; not proof of an OOM kill |
| Swap capacity | Confirmed less than 10% swap free; activity unmeasured |
| Disk capacity | Confirmed low available capacity on an observed mount |
| Inodes | Confirmed 5% or fewer free inodes |
| CPU | Confirmed high sample, or probable sustained contention when five-minute load is also elevated |
| Service failure | Confirmed systemd failed state |
| Service OOM | Confirmed systemd `Result=oom-kill`; exit 137 alone is insufficient |
| Bind conflict | Probable application journal evidence; corroboration requires an explicitly named current socket port |
| DNS/TCP | Confirmed outcome of one bounded check; underlying cause may remain unknown |
| Container OOM/health/exit | Runtime-reported state |
| Container restart loop | Probable from cumulative restarts >=3; events over time needed to confirm a loop |
| Container memory | Observed usage above 90% of an explicit limit |

Thresholds for memory, disk and CPU are configurable; each percentage must be in `(0,100]`. Findings use `warning` or `high` severity and `confirmed` or `probable` confidence. Potential filtering is an explicitly unverified hypothesis within timeout causes, never a confirmed firewall finding.

No global kernel-journal OOM parser, continuous monitor or historical database is implemented. Service OOM and container OOM evidence are supported. Configuration values and live observations are not retained between invocations.
