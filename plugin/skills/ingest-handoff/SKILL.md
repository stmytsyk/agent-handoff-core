---
name: ingest-handoff
description: Receives a trusted AHP context payload, verifies pairing, and imports sanitized git diffs and task context into the local agent session.
---

# Ingest Context Handoff

When the user asks to ingest or receive a handoff:

1. Ask the daemon to accept an inbound P2P handoff:
   ```bash
   ahp-cli ingest --peer "$1"
   ```
2. Verify the 6-digit SAS with the sender before trusting the payload.
3. Import the sanitized manifest into the current agent context.
4. Report the received branch, changed files, and pending tasks.
