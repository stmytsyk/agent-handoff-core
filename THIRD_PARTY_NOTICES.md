# Third-Party Notices

Agent Handoff Core depends on third-party open-source packages. This file is a human-readable summary, not a substitute for the complete license texts in upstream repositories and module caches.

## Direct Dependencies

| Dependency | Purpose | License |
| --- | --- | --- |
| `github.com/klauspost/compress` | zstd compression | Apache-2.0 |
| `github.com/pion/webrtc/v4` | WebRTC DataChannel | MIT |
| `github.com/pion/ice/v4` | ICE networking | MIT |

## Notable Transitive Dependencies

| Dependency family | License notes |
| --- | --- |
| Pion packages such as `datachannel`, `dtls`, `sctp`, `srtp`, `stun`, `turn`, `transport` | MIT-style licenses |
| `golang.org/x/*` packages | BSD-style Go project licenses |
| `github.com/google/uuid` | BSD-style license |

## Updating Notices

When adding or replacing dependencies:

1. Check the upstream license.
2. Update this file if the dependency is direct or security-relevant.
3. Run `go mod tidy`.
4. Include the dependency/license impact in the pull request.
