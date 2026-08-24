# Contributing

Contributions are welcome while the project is still shaping its design.

## Development Setup

```bash
git clone git@github.com:stmytsyk/agent-handoff-core.git
cd agent-handoff-core
go test ./...
go build ./cmd/ahpd ./cmd/ahp-cli ./cmd/ahp-relay
```

Or run the full local quality gate:

```bash
make check
```

Run WebRTC integration tests explicitly:

```bash
AHP_WEBRTC_TEST=1 go test ./cmd/ahpd ./pkg/transport -run 'TestSendReceiveEnvelopeViaRelayWebRTC|TestWebRTCChannelViaRelay' -count=1 -v
```

## Pull Request Checklist

- Keep changes focused.
- Add or update tests for behavioral changes.
- Run `gofmt`.
- Run `make check`.
- Update README, SECURITY.md, or THIRD_PARTY_NOTICES.md when trust boundaries, setup, or dependencies change.

## Design Priorities

The project favors:

- local-first trust
- explicit contact verification
- secret minimization before transport
- small, auditable payload formats
- simple defaults that can be tested locally

## Security Work

For security issues, follow [SECURITY.md](SECURITY.md). Do not put exploit details or private data in public issues or pull requests.
