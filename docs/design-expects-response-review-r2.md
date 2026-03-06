# Review: design-expects-response (round-2)

- Source doc: `docs/design-expects-response.md`
- Reviewed commit: cde2079b2acc66dfd833b0a886faea895943a26f
- Reviewer: plan-review

## Findings

### P1 - `--responds-to` clears the reminder before confirming response delivery

**Problem**
The responds-to flow removes the trigger first, then sends the response body if present ([docs/design-expects-response.md:73](docs/design-expects-response.md:73)-[76](docs/design-expects-response.md:76), [docs/design-expects-response.md:127](docs/design-expects-response.md:127)-[131](docs/design-expects-response.md:131)).

If response send fails after successful `trigger_remove`, the obligation is closed even though no response was delivered. This creates a silent false-success path and loses the only reminder signal.

**Required fix**
For body-present mode, make close semantics atomic with response delivery intent:
- send response first, then remove trigger on success, or
- implement compensating behavior (recreate trigger on send failure),
- and define exact CLI outcome when send fails after partial progress.

---

### P2 - Obligation closure is not bound to any sender/thread metadata

**Problem**
Closure is keyed only by short trigger ID (`--responds-to <id>`) with removal against own daemon ([docs/design-expects-response.md:82](docs/design-expects-response.md:82)-[89](docs/design-expects-response.md:89), [docs/design-expects-response.md:151](docs/design-expects-response.md:151)-[155](docs/design-expects-response.md:155)). No metadata binding is defined between obligation ID and original sender/target context.

This allows accidental wrong-ID closure of unrelated obligations without detection.

**Required fix**
Add minimal obligation metadata validation on close:
- store expected sender/target context with the trigger (or in trigger name/labels),
- on `--responds-to`, validate context when body/target is provided,
- return a clear warning/error if ID exists but context mismatches.

---

### P2 - Non-zero exit after message delivery can cause duplicate outbound messages in automation scripts

**Problem**
When `trigger_add` fails after message delivery, design specifies non-zero exit ([docs/design-expects-response.md:143](docs/design-expects-response.md:143)-[147](docs/design-expects-response.md:147)).

In scripted environments that retry non-zero commands, this can duplicate the original message while still not guaranteeing tracking creation.

**Required fix**
Document/define retry-safe behavior:
- include a distinct exit code for “delivered without tracking”,
- and/or emit a machine-readable status that allows callers to avoid naive retries,
- and add guidance in CLI docs for handling this partial-success case.

---

## Summary

3 findings: 0 P0, 1 P1, 2 P2, 0 P3

**Verdict**: Approved with revisions
