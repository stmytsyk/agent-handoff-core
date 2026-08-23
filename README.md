# AHP Core

AHP Core is a terminal-native agent handoff daemon and MCP-compatible context transport.

This repository contains the first implementation slice:

- Ed25519 identity generation, local key storage, and 6-digit SAS derivation.
- Payload building from git status, git diff, and agent summaries.
- Four-layer redaction for blocked files, token regexes, entropy scanning, and manifest-level sanitization.
- A Unix socket daemon at `/tmp/ahp.sock`.
- Streamable HTTP MCP-style endpoints for `share_context`, `ingest_context`, and `agent_ask`.
- Plugin skill definitions and helper scripts for share/ingest workflows.

The current transport package exposes a deterministic in-process WebRTC/Noise-shaped channel for tests and local integration. Production signaling and Pion DataChannel wiring can be attached behind the same interfaces.

## Build

```bash
go test ./...
go run ./cmd/ahpd
go run ./cmd/ahp-cli share @peer
```

## Local Test Flow

1. Run the package and E2E tests:

   ```bash
   GOCACHE=/tmp/ahp-go-build go test ./...
   ```

2. Build both commands:

   ```bash
   GOCACHE=/tmp/ahp-go-build go build ./cmd/ahpd ./cmd/ahp-cli
   ```

3. Start the local daemon:

   ```bash
   GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
   ```

4. In another terminal, ask the CLI to create a share envelope:

   ```bash
   GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli share @local-test
   ```

   The response should contain:

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

5. Start a local signaling relay:

   ```bash
   GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-relay --addr 127.0.0.1:8799
   ```

   This MVP relay exposes `/register`, `/signal`, and `/poll` over HTTP. It forwards offer/answer/ICE-style setup messages only; context payloads remain compressed and encrypted by the peers.

The current WebRTC implementation uses non-trickle ICE: each peer gathers candidates into a complete SDP, sends one `offer` or `answer` through the relay, then opens an `ahp` DataChannel. Later TURN fallback and live ICE candidate trickling can be added without changing the payload envelope.

Run the real WebRTC DataChannel integration test explicitly:

```bash
GOCACHE=/tmp/ahp-go-build AHP_WEBRTC_TEST=1 go test ./pkg/transport -run TestWebRTCChannelViaRelay -count=1 -v
```

This test opens local ICE UDP sockets, so restricted sandboxes may need permission.

## Relay-Backed Handoff

Terminal 1, start a relay:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-relay --addr 127.0.0.1:8799
```

Each user initializes a local identity and contact string:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli init --handle @alice --relay http://127.0.0.1:8799
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli contact show
```

Exchange the printed `ahp:...#ed25519:...` contact strings out of band. Add the other user after comparing the displayed SAS:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli contact add --trust 'ahp:bob@http://127.0.0.1:8799#ed25519:...'
```

On Bob's machine, start `ahpd`, then wait for Alice:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli ingest --peer @alice
```

On Alice's machine, start `ahpd`, then share to Bob:

```bash
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahpd
GOCACHE=/tmp/ahp-go-build go run ./cmd/ahp-cli share @bob
```

For two different PCs, replace `127.0.0.1` with the reachable relay host. The receiver should start `ingest` before the sender runs `share`.
