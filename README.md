# JANE - Attestation Engine

This is the source for the Jane Attestation Engine, a fork and major rewrite of the former A10 Nokia Attestation Engine.

Jane is the remote attestation engine of a trusted or confidential computing environment: it holds the known-good values for devices and other elements, asks their trust agents for evidence (TPM 2.0 quotes, measured-boot and IMA logs, UEFI variables, ...), and verifies it with rules. It explores mechanisms for remote attestation and implements ideas from the IETF RATS architecture.

| Directory | Contents |
|---|---|
| `janeserver/` | the attestation engine: Web UI, REST API and x3270 terminal interface |
| `tarzan/` | tarzan, the reference trust agent |
| `provisioner/` | jp, which registers an element with Jane, provisions its TPM and attests it |
| `etc/` | packaging (`etc/debbuild`), standard intents, `addjaneuser`, scripts |
| `docs/` | the documentation |

Start with the [Quick start](docs/quickstart.md), then read [Hardening](docs/hardening.md): out of the box Jane runs without TLS or logins, and it is research software, provided as-is. All documentation: [contents](docs/contents.md). Changes by release: [changelog](CHANGELOG.md).
