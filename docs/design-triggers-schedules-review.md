# Review: design-triggers-schedules (plan-review)

- Source doc: `docs/design-triggers-schedules.md`
- Reviewed commit: 51ee17fb9720f9e549e84b7d0fe86e1c75cf23d8
- Reviewer: plan-review

## Findings

### P1 - Persistence model is internally inconsistent (rc.Triggers/rc.Schedules are referenced but undefined)

**Problem**
The daemon integration section loads automations from `rc.Triggers` and `rc.Schedules` ([docs/design-triggers-schedules.md:457](docs/design-triggers-schedules.md:457), [docs/design-triggers-schedules.md:460](docs/design-triggers-schedules.md:460)), but the data model section never defines where these collections live in persisted runtime config/state ([docs/design-triggers-schedules.md:133](docs/design-triggers-schedules.md:133)-[175](docs/design-triggers-schedules.md:175)).

This leaves a critical gap: there is no specified source of truth for what the daemon should load at start/resume.

**Required fix**
Define an explicit persistence schema for automation specs. At minimum:
- where role-defined triggers/schedules are stored after render,
- where dynamically added entries are stored,
- exact merge/precedence rules at daemon startup,
- how IDs are generated and stabilized across resume.

---

### P1 - Dynamic registration durability/restart semantics are not specified

**Problem**
The design allows runtime add/remove via socket requests ([docs/design-triggers-schedules.md:295](docs/design-triggers-schedules.md:295)-[307](docs/design-triggers-schedules.md:307), [docs/design-triggers-schedules.md:469](docs/design-triggers-schedules.md:469)-[470](docs/design-triggers-schedules.md:470)), but does not define whether those mutations survive daemon restart/resume.

Without explicit durability semantics, operators cannot rely on automations after reconnects/restarts, and behavior can silently diverge from user expectations.

**Required fix**
Specify lifecycle guarantees for dynamic registrations:
- ephemeral (session-memory only) vs durable (persisted),
- if durable, exact write/read path and atomicity expectations,
- if ephemeral, explicit UX warning in CLI output and docs.

---

### P1 - Async exec actions can create unbounded process fan-out

**Problem**
`Exec` actions are defined as asynchronous and non-blocking ([docs/design-triggers-schedules.md:382](docs/design-triggers-schedules.md:382)-[385](docs/design-triggers-schedules.md:385)). Combined with high-frequency RRULEs and frequent event matches, this can spawn unlimited concurrent shells.

That is a correctness and stability risk (resource exhaustion, action pile-up, uncontrolled retries) with no backpressure/rate-limiting strategy specified.

**Required fix**
Add execution-control policy to the design:
- max concurrent exec actions (global and/or per trigger/schedule),
- queue/drop policy when saturated,
- timeout/kill behavior for long-running actions,
- observability counters for dropped/deferred executions.

---

### P2 - One-shot semantics are undefined when action execution fails

**Problem**
Trigger flow says execute then remove ([docs/design-triggers-schedules.md:104](docs/design-triggers-schedules.md:104)-[107](docs/design-triggers-schedules.md:107)); implementation notes also say run then remove ([docs/design-triggers-schedules.md:334](docs/design-triggers-schedules.md:334)).

But failure semantics are unspecified: if action launch fails or exits non-zero, is the trigger consumed or retried? Similar ambiguity exists for schedule `RunOnceWhen` when the condition passes but action fails.

**Required fix**
Define deterministic failure semantics per primitive:
- consume-on-attempt vs consume-on-success,
- retry policy (none/fixed/backoff),
- how failures are surfaced to users (`list`, logs, metrics).

---

### P2 - Heartbeat migration explicitly changes behavior without compatibility guardrails

**Problem**
The doc acknowledges behavior change: old heartbeat required full idle duration, new schedule ticks regardless of state ([docs/design-triggers-schedules.md:501](docs/design-triggers-schedules.md:501)-[504](docs/design-triggers-schedules.md:504)).

This can materially alter operator-visible behavior (e.g., periodic checks while busy plus deferred idle delivery), but no migration controls or validation criteria are defined.

**Required fix**
Add migration guardrails:
- explicit compatibility mode or documented non-equivalence with rollout guidance,
- tests asserting acceptable behavior under long active periods and rapid idle/active transitions,
- clear user-facing release note for changed heartbeat semantics.

---

## Summary

5 findings: 0 P0, 3 P1, 2 P2, 0 P3

**Verdict**: Approved with revisions
