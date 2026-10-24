# Architecture

SystemSync separates orchestration from evidence acquisition and interpretation.

1. Cobra validates arguments and loads configuration into a per-command application.
2. The coordinator schedules independent collectors behind a bounded semaphore. Each collector receives a context with its own timeout; cancellation propagates to network operations and child processes. Observation order follows registration, independently of completion order.
3. Collectors return typed observations plus errors. Partial evidence remains available. Native reads use `/proc`, interfaces and `statfs`; command adapters invoke fixed executables with argument arrays, never a shell.
4. Independently registerable `rules.Rule` implementations produce findings with evidence, severity, confidence, timestamp, possible causes and next steps. Correlation augments service bind findings when an explicitly named port is currently occupied, and adds matching service PID information.
5. A schema-versioned report carries observations, collection errors and findings to the terminal or JSON renderer.
6. Optional AI receives a restricted, redacted projection of findings. It has no tool or execution interface.
7. Experimental remediation verifies an allowlisted plan against its canonical form, checks explicit approval, performs a bounded DNS recheck and returns verification results. No system mutation is implemented.

## Package boundaries

- `pkg/types`: public report and observation types.
- `internal/config`: secure initialization, YAML defaults and validation.
- `internal/collectors/{system,process,network,services,containers}`: observation acquisition and parsers.
- `internal/diagnostics`: deadlines, bounded collection and rule dispatch.
- `internal/rules`: deterministic interpretation and cross-source correlation.
- `internal/report`: JSON and terminal output.
- `internal/ai`: restricted payload, redaction and provider HTTP clients.
- `internal/remediation`: approval and operation allowlist.
- `internal/cli` and `cmd/ssync`: command wiring and signal handling.

systemd is accessed through `systemctl show` key/value output, JSON unit lists and JSON journal lines. This keeps optional systemd dependencies out of the core process. A direct D-Bus transport can later implement the same adapter boundary. Docker uses a daemon-advertised version; Podman uses its distinct native Libpod API. Adapter tests use injected runners and HTTP servers.

## Boundaries and guarantees

A collection timeout bounds context-aware external work, not an uninterruptible kernel filesystem operation. Native reads can still block on unhealthy filesystems. Linux files and namespaces can change between observations; this is a snapshot with a CPU sampling interval, not an atomic transaction. Process CPU sampling checks start time to reject PID reuse. Container list commands collect summaries; full health/log/usage inspection is explicit.

The rule engine does not use unavailable values as proof of health. Severity describes impact and confidence describes evidence strength. Adding a collector does not authorize adding a mutation. Adding an AI provider does not confer machine access.
