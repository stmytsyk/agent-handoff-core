# Security Policy

Agent Handoff Core is an early prototype. Do not use it as production security software yet.

## Supported Versions

There are no stable releases yet. Security fixes are applied to the `main` branch until versioned releases exist.

## Reporting a Vulnerability

Do not open a public GitHub issue with exploit details, private repository content, credentials, or proof-of-concept payloads.

Until a dedicated security contact is published, open a minimal GitHub issue that says you have a security report and need a private contact channel. Do not include sensitive details in that issue.

## Current Security Caveats

- Redaction can miss secrets.
- Relay operators can observe metadata such as handles, timing, and connection attempts.
- The current Noise-style wrapper is a prototype and needs replacement with a reviewed Noise implementation before production use.
- Contact trust is manual.
- No external security audit has been performed.

## Security-Relevant Areas

Please be especially careful with changes touching:

- `pkg/redaction`
- `pkg/crypto`
- `pkg/transport`
- contact trust handling in `pkg/contacts`
- daemon IPC in `cmd/ahpd`
