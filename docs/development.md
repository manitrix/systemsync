# Development and contributions

Go 1.26.5+; Linux is required for live collectors. `make` and GNU tar/gzip/sha256sum are needed for release packaging. A C compiler is needed for Linux race testing. The runtime binary has no mandatory external dependencies for core `/proc` collection.

```sh
go mod download
go test ./...
go vet ./...
go test -race ./...
go test -tags=integration ./test/...
make build
make release VERSION=v0.1.0
```

Unit tests use inline sanitized proc fixtures, injected systemd runners, local listeners and mock HTTP servers. Tests do not invoke root-only operations or require live Docker, Podman, AI or external DNS. Integration tests use real unprivileged Linux observations and a temporary Unix socket server. They are selected with the `integration` build tag. Windows can compile the project; Linux collectors return explicit unsupported errors there.

Before proposing a change, describe the concrete failure and how the new evidence supports the finding. Add a parser fixture or failing behavior test, preserve context propagation, and run formatting, tests and vet. Add a new rule through `rules.Rule`; every finding must carry ID, category, severity, confidence, explanation, evidence, possible causes, next steps and timestamp. Avoid treating absent measurements as zero-risk measurements.

Do not add mutations to collectors. Changes involving remediation need an explicit canonical operation, validation of preconditions, a reviewable plan, approval, postcondition verification and rollback documentation. AI integration must remain optional, payload-minimized and isolated from execution.

## Release inputs

`make build` injects version, commit and commit date. `scripts/release.sh` cross-builds AMD64/ARM64 with `CGO_ENABLED=0`, `-trimpath`, no build VCS metadata and an empty build ID. Archives use sorted entries, numeric owner/group zero, source commit timestamps and gzip without timestamps. Source, dependencies, Go version, VERSION, COMMIT, BUILD_DATE and SOURCE_DATE_EPOCH are reproducibility inputs. Never claim a release is stable based only on cross-compilation.

CI runs tests, vet, race, formatting, live collector integration and release packaging on Linux. Tag pushes matching `v*` run validation and publish release assets. No release or remote commit is created merely by editing this repository.

## Roadmap

- Validate actual supported Docker and Podman version matrices, rootless deployments and multiple Linux distributions.
- Add direct systemd D-Bus transport and more runtime-version fixtures.
- Add PSI and cgroup pressure, continuous CPU/restart sampling and timestamp-aware journal correlation.
- Add socket ownership and service network configuration mapping without guessing expected ports.
- Explore tightly scoped state-changing remediation only with separately reviewed safety requirements.
- Establish release security review and compatibility guarantees before declaring a stable release.
