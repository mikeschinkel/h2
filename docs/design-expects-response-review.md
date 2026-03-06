# Review: design-expects-response (plan-review)

- Source doc: `docs/design-expects-response.md`
- Reviewed commit: d40438d087d264d5f959a8c2dbe159b3fa6cb3c1
- Reviewer: plan-review

## Findings

### P1 - Expects-response creation is non-atomic and can silently lose obligations

**Problem**
The flow explicitly does message send first, then trigger registration ([docs/design-expects-response.md:53](docs/design-expects-response.md:53)-[54](docs/design-expects-response.md:54), [docs/design-expects-response.md:123](docs/design-expects-response.md:123)-[126](docs/design-expects-response.md:126)).

If `trigger_add` fails after successful message delivery (socket race, daemon restart, validation failure), the user believes response-tracking is active, but no reminder trigger exists. The design does not define rollback/error semantics for this split operation.

**Required fix**
Define transaction/error behavior for the two-step operation. At minimum:
- CLI must return non-zero when `trigger_add` fails,
- output must clearly state that message was delivered but tracking was not created,
- add retry/idempotency behavior or compensating path (e.g., re-attach by ID).

---

### P1 - 8-character trigger ID space is collision-prone and collision handling is unspecified

**Problem**
The identifier is fixed to an 8-char short ID ([docs/design-expects-response.md:22](docs/design-expects-response.md:22), [docs/design-expects-response.md:58](docs/design-expects-response.md:58), [docs/design-expects-response.md:121](docs/design-expects-response.md:121)).

In a long-lived daemon with many obligations, collisions are plausible. The design does not define what happens on ID conflict in TriggerEngine (`add` reject/overwrite/rename), nor how CLI reports it.

**Required fix**
Specify collision-safe ID semantics:
- use a longer globally unique ID for storage (UUID/ULID),
- optionally keep short IDs only as display aliases,
- define deterministic behavior when an add collides (reject + regenerate + retry).

---

### P2 - `--responds-to` target semantics are ambiguous and not self-describing

**Problem**
The command syntax says `h2 send --responds-to <id> [target] ["body"]` ([docs/design-expects-response.md:82](docs/design-expects-response.md:82)), but examples include a required target even for no-body close ([docs/design-expects-response.md:42](docs/design-expects-response.md:42)).

The design stores only trigger ID in the message schema ([docs/design-expects-response.md:151](docs/design-expects-response.md:151)-[155](docs/design-expects-response.md:155)), so there is no canonical source for “original sender/target” when closing obligations.

**Required fix**
Clarify CLI contract and metadata model:
- either make target required whenever body is present, optional otherwise,
- or persist original sender identity in obligation metadata so target can be inferred safely,
- document exact validation/error messages for invalid flag/arg combinations.

---

### P2 - Test plan misses failure-path and recovery cases central to the design

**Problem**
Tests cover only happy-path add/remove/format and basic idle reminder flow ([docs/design-expects-response.md:193](docs/design-expects-response.md:193)-[211](docs/design-expects-response.md:211)).

Critical failure paths are untested in the plan: trigger_add failure after send, trigger_remove failure while still sending body, ID collision behavior, and daemon restart between expects-response and responds-to.

**Required fix**
Add explicit tests for:
- message-delivered/trigger-add-failed outcome and CLI exit behavior,
- trigger ID collision handling,
- responds-to with missing own socket + with body,
- restart/lost-trigger behavior with clear user-visible warning semantics.

---

## Summary

4 findings: 0 P0, 2 P1, 2 P2, 0 P3

**Verdict**: Approved with revisions
