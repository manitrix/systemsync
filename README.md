# SystemSync (`ssync`)

Intelligent Linux diagnostics and safe automation, written in Go. SystemSync collects real observations and applies deterministic rules to explain system, service, network, and container failures. Core diagnostics work offline without root, a container runtime, or an LLM.

**Initial implementation; not yet a production-validated stable release.** See [implementation status](IMPLEMENTATION_STATUS.md) for verification and limitations. Linux is the target; other platforms build and clearly report unavailable Linux collectors.

## Build and install

Requires Go 1.26.5 or newer. Download dependencies once; subsequent core execution is offline.

```sh
git clone https://github.com/manitrix/systemsync.git
cd systemsync
go build -trimpath -buildvcs=false -o bin/ssync ./cmd/ssync
./bin/ssync doctor
# Or install into GOBIN (defaults to ~/go/bin):
go install ./cmd/ssync
```

`make build` injects version, commit and commit date. `make release VERSION=v0.1.0` produces Linux AMD64/ARM64 archives and SHA-256 checksums under `dist/`. Release builds disable cgo, strip workspace paths, and use deterministic metadata. Rebuilding with the same source, dependencies, Go version and build inputs produces reproducible artifacts.

## Usage

```sh
ssync version
ssync help
ssync doctor
ssync status
ssync diagnose
ssync diagnose --service nginx
ssync diagnose --pid 1234
ssync network
ssync network dns                         # resolver configuration; no DNS query
ssync network dns --domain example.com    # explicit single-name lookup
ssync network ports
ssync network routes
ssync network firewall
ssync network check --host example.com --port 443 --timeout 3s
ssync containers
ssync containers inspect web --runtime docker
ssync containers diagnose web --runtime podman
ssync services
ssync services inspect nginx
ssync logs --service nginx
ssync explain
ssync report --format json > report.json
ssync explain --input report.json
ssync config init
ssync config show
ssync completion bash > ssync.bash
ssync completion zsh > _ssync
```

`--config PATH`, `--format text|json`, and `--verbose` are global flags. Configuration defaults to `~/.config/systemsync/config.yaml` on Linux; `XDG_CONFIG_HOME` is respected. Creation uses mode 0600 and refuses to overwrite existing files. Configure thresholds, sample duration, operation deadlines, log limits, concurrency, disabled collectors and optional AI. See [example config](configs/config.example.yaml).

A report contains schema version `1`, UTC timestamps, observations, collection errors and findings. Every finding contains severity, confidence, evidence, possible causes and next steps. Terminal output uses no ANSI colors; JSON is suitable for automation. A missing or inaccessible collector never implies health. Reports can contain sensitive local data: protect exported files.

Illustrative finding, **not a captured machine result**:

```text
[high] Service reported a port conflict (probable) [service.bind:nginx]
  Evidence: bind() to 0.0.0.0:8080 failed: Address already in use
  Evidence: Current tcp socket 0.0.0.0:8080 inode 123
  Next: Compare current listening sockets with the service listen configuration
```

Exit codes: `0` means a report or requested output was produced; findings do not change the exit code. Aggregate diagnostics preserve partial reports even if every collector is unavailable. A focused command returns `1` on collection failure after printing its report. Invalid arguments, configuration and output errors also return `1`. DNS/TCP check failures are diagnostic observations with findings and return `0`; inspect `outcome` or `findings` in JSON. AI failure preserves the diagnostic report and appears in its explanation.

## Features and architecture

- Sampled aggregate and process CPU; memory/swap capacity, uptime, load, disk space and inodes.
- Process RSS, thread count and FD count with explicit permission warnings.
- Interfaces, addresses, IPv4/IPv6 routes, TCP listeners, bound UDP sockets and namespace identity.
- Bounded DNS/TCP checks distinguishing refusal, timeout and unreachable destinations.
- nftables JSON inspection and iptables backend/version detection; no firewall changes.
- systemd service state and structured journal reads through `systemctl`/`journalctl` adapters.
- Docker and native Podman Unix-socket adapters, inspection, logs, health, restart/exit/OOM evidence, ports, networking and mount metadata. Docker and native Podman resource usage sampling.
- Modular deterministic rules and cross-source service bind-error correlation.
- Optional Ollama and OpenAI-compatible explanations; no model command execution.

`cmd/ssync` → Cobra `internal/cli` → bounded concurrent `internal/diagnostics` collectors → `internal/rules` → `pkg/types.Report` → terminal/JSON `internal/report` → optional sanitized `internal/ai` explanation. No collector runs shell command strings. See [architecture](docs/architecture.md), [diagnostics](docs/diagnostics.md), and [networking](docs/networking.md).

## Permissions and supported environments

Targets Linux 3.14+ with `/proc`, `/sys`, and `statfs`; systemd features require `systemctl`, `journalctl` and a running accessible systemd manager. JSON service listing needs systemd 245+. Non-systemd systems retain other diagnostics. Network data reflects the current namespace; containers and restricted `/proc` mounts may hide host data. Detailed validation on multiple distributions and actual runtime versions is still required.

Run as an ordinary user. Journal access may need the distribution's journal group; FD reads may be restricted; nftables rules typically need `CAP_NET_ADMIN`. SystemSync does not elevate itself. Docker socket access is powerful even though this client sends only GET requests. Rootless Podman uses `$XDG_RUNTIME_DIR/podman/podman.sock`; rootful fallback is `/run/podman/podman.sock`. Override socket paths using `SSYNC_DOCKER_SOCKET` or `SSYNC_PODMAN_SOCKET`. It never enables a runtime socket service itself.

## Optional AI and privacy

`ssync explain --ai` and `ssync diagnose --service nginx --ai` explicitly opt in. Defaults select local Ollama at `http://localhost:11434`; install and provision the model separately. No AI call is made without `--ai`.

For an external OpenAI-compatible endpoint, explicitly configure `ai.provider: openai`, an HTTPS `ai.endpoint` ending in `/v1`, a model, and `ai.external_consent: true`. Set `SSYNC_API_KEY` in the environment. Keys are never written to YAML. Redirects are rejected.

AI receives only finding category, severity, confidence, title, explanation, suggested checks and an unavailable-collector count. Raw observations, logs, evidence, names, addresses, mounts and environment values are excluded. Likely credentials in retained text are redacted. User-supplied reports are untrusted; secret matching cannot recognize every arbitrary secret. AI output is advisory, can be wrong, and is never executed. See [AI integration](docs/ai-integration.md) and [security](docs/security.md).

## Remediation

The initial release is read-only. An experimental approval framework implements a single allowlisted **DNS recheck**, which verifies a hostname without changing machine state:

```sh
ssync remediate --dns-recheck example.com             # display plan and rollback limits
ssync remediate --dns-recheck example.com --approve   # explicitly approve and verify
```

There are no service restarts, cleanup operations, firewall edits, or network changes. State-changing repairs remain pending. Elevated restart counts are historical evidence, not proof of a current crash loop. A TCP timeout does not establish a firewall cause.

## Tests and contributions

```sh
go test ./...
go vet ./...
go test -race ./...                         # Linux C compiler/cgo required
go test -tags=integration ./test/...         # unprivileged real Linux collectors
make check
```

Tests cover parsers, rules, correlation, CLI validation, config, report JSON, mocked container HTTP APIs, injected systemd runners, redaction, AI failures/deadlines and remediation approval. Linux CI also cross-builds both target architectures. No test needs root, an LLM or a container daemon. See [development](docs/development.md) for fixtures, integration tests, contribution guidelines and the roadmap. MIT licensed.
