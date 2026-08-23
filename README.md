# Agent Handoff Core

[![CI](https://github.com/stmytsyk/agent-handoff-core/actions/workflows/ci.yml/badge.svg)](https://github.com/stmytsyk/agent-handoff-core/actions/workflows/ci.yml)
[![CodeQL](https://github.com/stmytsyk/agent-handoff-core/actions/workflows/codeql.yml/badge.svg)](https://github.com/stmytsyk/agent-handoff-core/actions/workflows/codeql.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Agent Handoff Core is an experimental local daemon and CLI for handing off coding-agent context between developers.

It packages the current Git work state, removes likely secrets, compresses the result, and sends it to a trusted peer over an encrypted peer-to-peer WebRTC DataChannel.

The goal is simple: let one developer say "continue this work from here" without copy-pasting chat logs, screenshots, raw diffs, or private project files into a third-party service.

## Status

This project is an early prototype.

Implemented:

- local daemon listening on `/tmp/ahp.sock`
- CLI for init, contact exchange, sharing, and ingesting
- Ed25519 local identity
- local contact book
- 6-digit SAS verification code
- Git diff/status payload builder
- secret redaction
- zstd compression envelope
- HTTP signaling relay
- WebRTC DataChannel transport
- Noise-style payload wrapper over the DataChannel
- MCP-style HTTP handler package
- agent skill folders for handoff workflows

Not production-ready yet:

- relay is HTTP, not hardened WSS
- TURN fallback is not fully wired into the CLI flow
- contact trust UX is basic
- daemon install/service management is not packaged
- MCP server package exists, but there is not yet a polished standalone MCP command
- the Noise wrapper is a prototype and should be replaced with a reviewed Noise implementation before production use

## Why This Exists

Modern coding agents often build useful context that is hard to transfer:

- what changed in the working tree
- why the code changed
- what still needs to be done
- which files are relevant
- what should not be sent because it may contain secrets

Agent Handoff Core turns that state into a sanitized payload that another local agent can ingest.

## How It Works

```text
Alice CLI
  -> Alice daemon
  -> build Git payload
  -> redact secrets
  -> compress with zstd
  -> encrypt/wrap payload
  -> WebRTC DataChannel
  -> Bob daemon
  -> Bob CLI / agent
```

A small relay helps peers exchange WebRTC setup messages:

```text
Alice daemon -> relay: offer
Bob daemon   -> relay: answer
Alice/Bob    -> direct WebRTC DataChannel when ICE succeeds
```

The relay forwards connection setup messages. It should not receive plaintext handoff content.

## What Gets Shared

The payload currently includes:

- current branch name
- `git status --porcelain`
- `git diff HEAD`
- optional agent/session summary
- optional pending tasks

The payload is encoded as an `ahp-payload/v1` manifest, then wrapped in an `ahp-envelope/v1` zstd-compressed envelope.

## Secret Redaction

Before transfer, Agent Handoff Core applies these protections:

- blocks sensitive file paths such as `.env`, `.env.*`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, `id_rsa`, and `id_ed25519`
- redacts common token patterns such as `sk-ant-*`, `sk-*`, `ghp_*`, and `AKIA*`
- redacts high-entropy strings with Shannon entropy `>= 4.5`
- sanitizes manifest text before compression

Redaction is a safety layer, not a guarantee. Review what you share before using this on sensitive repositories.

## Requirements

- Go 1.24+
- Git
- macOS/Linux shell environment
- network access between peers and relay for real handoffs

## Quick Start

Clone and test:

```bash
git clone git@github.com:stmytsyk/agent-handoff-core.git
cd agent-handoff-core
GOCACHE=/tmp/ahp-go-build go test ./...
```

Build the commands:

```bash
GOCACHE=/tmp/ahp-go-build go build ./cmd/ahpd ./cmd/ahp-cli ./cmd/ahp-relay
```

## Local Packaging Test

Terminal 1:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
```

Terminal 2:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli share @local-test
```

This does not contact a peer. It returns a compressed envelope so you can verify packaging and redaction.

Expected shape:

```json
{
  "ok": true,
  "envelope": {
    "schema_version": "ahp-envelope/v1",
    "encoding": "base64",
    "compression": "zstd"
  }
}
```

## Peer Handoff Demo

Start a relay:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-relay --addr 127.0.0.1:8799
```

Start the daemon:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
```

Initialize Alice:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli init --handle @alice --relay http://127.0.0.1:8799
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli contact show
```

Initialize Bob on another machine, another user account, or a separate test config:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli init --handle @bob --relay http://127.0.0.1:8799
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli contact show
```

Exchange the printed contact strings out of band:

```text
ahp:alice@http://127.0.0.1:8799#ed25519:...
ahp:bob@http://127.0.0.1:8799#ed25519:...
```

Add the other user after comparing the displayed SAS:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli contact add --trust 'ahp:bob@http://127.0.0.1:8799#ed25519:...'
```

The receiver starts first:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli ingest --peer @alice
```

The sender shares:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli share @bob
```

For two different PCs, replace `127.0.0.1` with a relay host both machines can reach.

## CLI Commands

Initialize local identity:

```bash
go run ./cmd/ahp-cli init --handle @alice --relay http://relay.example.com:8799
```

Show your contact string:

```bash
go run ./cmd/ahp-cli contact show
```

Add a trusted contact:

```bash
go run ./cmd/ahp-cli contact add --trust 'ahp:bob@http://relay.example.com:8799#ed25519:...'
```

Wait for a handoff:

```bash
go run ./cmd/ahp-cli ingest --peer @alice
```

Send a handoff:

```bash
go run ./cmd/ahp-cli share @bob
```

## Testing

Run default tests:

```bash
GOCACHE=/tmp/ahp-go-build go test ./...
```

Run WebRTC integration tests explicitly:

```bash
GOCACHE=/tmp/ahp-go-build AHP_WEBRTC_TEST=1 go test ./cmd/ahpd ./pkg/transport -run 'TestSendReceiveEnvelopeViaRelayWebRTC|TestWebRTCChannelViaRelay' -count=1 -v
```

The WebRTC integration tests open local HTTP and ICE UDP sockets.

## Repository Layout

```text
cmd/
  ahpd/       local daemon
  ahp-cli/    user CLI
  ahp-relay/  signaling relay
pkg/
  contacts/   local identity and contact book
  crypto/     keys, SAS, Noise wrapper
  mcp/        MCP-style HTTP handlers
  payload/    Git payload builder and zstd envelope
  redaction/  secret sanitizer
  transport/  relay, WebRTC, and secure channel interfaces
plugin/
  skills/     handoff skill definitions and helper scripts
tests/        end-to-end tests
```

## License

Agent Handoff Core is licensed under the MIT License. See [LICENSE](LICENSE).

This project has third-party dependencies with their own licenses. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Direct dependencies currently include:

| Dependency | Purpose | License |
| --- | --- | --- |
| `github.com/klauspost/compress` | zstd compression | Apache-2.0 |
| `github.com/pion/webrtc/v4` | WebRTC DataChannel | MIT |
| `github.com/pion/ice/v4` | ICE networking | MIT |

Transitive dependencies include packages from Pion, Go `x/*`, and Google UUID. Check `go.mod` and `go.sum` for the exact dependency graph used by a given build.

License references:

- Pion WebRTC is MIT licensed.
- Klauspost compress is Apache-2.0 licensed.
- Google UUID uses a BSD-style license.

This README is not legal advice. Review dependency licenses before redistributing binaries or embedding this code in a commercial product.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and pull request expectations.

Please also read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). For vulnerabilities, follow [SECURITY.md](SECURITY.md).

## Security

Do not treat this as production security software yet.

Known security caveats:

- redaction can miss secrets
- public relay metadata is visible to the relay operator
- the prototype Noise wrapper should be replaced with a reviewed implementation
- contact trust is currently manual
- no external security audit has been performed

If you find a security issue, do not open a public issue with exploit details. Contact the maintainer privately first.

## Roadmap

Near-term work:

- package daemon installation
- add WSS relay mode
- add TURN fallback configuration
- improve contact verification UX
- expose a polished MCP server command
- add release artifacts
- add CI
- replace prototype Noise code with a reviewed library
