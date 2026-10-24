# Optional AI integration

No AI request occurs without `--ai`. Core findings always come from deterministic rules and remain available if the provider fails.

## Ollama

Provision Ollama and the model separately, then use the default config or choose a model:

```yaml
ai:
  provider: ollama
  endpoint: http://localhost:11434
  model: llama3.2
  external_consent: false
  key_env: SSYNC_API_KEY
```

The client calls `/api/chat` with streaming disabled. Local inference can still use significant resources. A loopback address does not prove that a locally configured proxy or provider never forwards data.

## OpenAI-compatible

```yaml
ai:
  provider: openai
  endpoint: https://your-provider.example/v1
  model: your-model
  external_consent: true
  key_env: SSYNC_API_KEY
```

Set the named environment variable to the API credential; do not put credentials in YAML or the endpoint URL. The client appends `/chat/completions`. External endpoints require both explicit configuration consent and an explicit `--ai` invocation. External plaintext HTTP, URL credentials and redirects are rejected. Provider calls inherit the configured deadline; responses are capped at 1 MiB and HTTP errors never echo response bodies or credentials.

## Data and trust

The serialized payload contains schema version, unavailable-collector count and only finding category, severity, confidence, title, explanation and next steps. It excludes raw observations, evidence, logs, service/container IDs and names, addresses, mounts and environment values. Credential patterns in retained string fields are redacted before JSON encoding so redaction cannot corrupt the JSON envelope.

The system instruction treats supplied text as untrusted data, asks for separation of evidence and speculation, and grants no tools. Input reports may contain prompt injection; there is no guarantee that a model will follow instructions. Safety depends on the absence of any execution bridge and the minimized payload, not on the prompt alone. Secret-pattern detection is best effort, especially for arbitrary user-authored reports. Review what you send and apply your provider's retention policy.

AI output is clearly labeled advisory and cannot initiate remediation. Provider interface implementations can be independently tested or replaced without changing collectors.
