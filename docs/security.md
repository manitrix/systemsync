# Security model

SystemSync defaults to unprivileged read-only collection and offline deterministic analysis. It does not run model-generated commands or invoke a shell. External commands use fixed executable names and structured argument arrays with exact validated service names. Keep PATH trustworthy; a malicious replacement executable can defeat this boundary.

New configuration uses an exclusive-create operation with mode 0600 and parent mode 0700. Existing files are never overwritten by `config init`. Existing directory modes are not changed, and Windows does not enforce POSIX permissions in the same manner. API keys are sourced only from the configured environment variable.

Reports and terminal output contain local observations and may include sensitive journal messages, process names, paths, addresses or container metadata. Report files are created by shell redirection with your shell's umask; use `umask 077` when exporting sensitive reports. Default local reports are not comprehensively anonymized. Sharing reports is a separate decision from consenting to AI.

All application logs, metadata and imported reports are untrusted. JSON encoding escapes control characters in observation data, but copied evidence or AI text in human output should still be treated as untrusted terminal content. Do not pipe recommendations into a shell. Pattern redaction cannot guarantee removal of every secret. AI excludes raw observations and evidence entirely; see the AI integration guide.

Runtime access uses local Unix sockets and GET-only APIs. Redirects are rejected and API responses are capped at 4 MiB. Docker socket membership can confer root-equivalent access independently of SystemSync's read-only implementation. Podman rootless access is scoped by its runtime. SystemSync never enables socket services or changes runtime configuration.

Remediation accepts only a canonical `dns-recheck` plan and explicit approval. Its preconditions validate the hostname and operation; its execution performs a bounded resolver query; its verification reports addresses or errors. It modifies no state, so rollback is unnecessary. No arbitrary commands, service restart, cleanup, firewall alteration or network change is available. Future state-changing operations require separate allowlists, preconditions, approval and verification design.

Collection deadlines cover context-aware operations. Kernel filesystem reads may remain blocked on unhealthy mounts. Resource exhaustion, changing namespaces, procfs races and maliciously large local data remain practical limitations. This is a troubleshooting assistant, not a security scanner or enforcement tool.

Report vulnerabilities privately to repository maintainers through GitHub private vulnerability reporting when enabled; do not publish secrets or exploit details in a public issue.
