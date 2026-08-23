---
name: share-handoff
description: Packages local git branch diffs, agent history, and pending tasks, sanitizes secrets, and streams the context payload P2P to a trusted colleague handle.
---

# Share Context Handoff

When the user asks to hand off work, share context, or transfer a task to a colleague (e.g., `/share-handoff @peer`):

1. Execute `git diff HEAD` and `git status --porcelain` to capture active branch changes.
2. Summarize key architecture choices made during this session into a 3-bullet point summary.
3. Call the Go CLI to strip secrets and stream the payload:
   ```bash
   ahp-cli share "$1"
   ```
4. Confirm to the user once the P2P WebRTC connection delivers the payload.
