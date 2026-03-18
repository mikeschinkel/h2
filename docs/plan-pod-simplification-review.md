# Review: plan-pod-simplification

- Source doc: `docs/plan-pod-simplification.md`
- Reviewed commit: `48af191`
- Reviewer: `h2-reviewer`

## Findings

### P1 - Global bridge credential map removes user-level isolation without replacement

**Problem**
The plan moves bridge credentials from `users.<name>.bridges` to a global top-level `bridges` map ([docs/plan-pod-simplification.md:224](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:224)-[245](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:245), [docs/plan-pod-simplification.md:249](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:249)-[251](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:251)) but does not define any authorization boundary for which user/pod can reference which named bridge.

This can unintentionally broaden access to credentials in multi-user/shared environments. Today the plan is effectively "any caller that can name a bridge can use that credential set." 

**Required fix**
Specify and enforce an access model for named bridges (for example: ownership/allowlist per user/pod, or explicit statement that config is single-user and unsupported for shared hosts). Add validation points and rejection behavior in CLI/pod launch for unauthorized bridge references.

---

### P1 - Bridge restart semantics can terminate unrelated bridge daemons

**Problem**
During pod launch, step 2a says to stop any existing bridge daemon with the same bridge name before starting ([docs/plan-pod-simplification.md:268](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:268)-[270](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:270)). This is unsafe when that bridge is currently used by another pod or a standalone bridge process.

The plan mentions adding `Pod` metadata for tracking ([docs/plan-pod-simplification.md:277](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:277)), but does not define conflict policy (reject? steal? share?) or guardrails.

**Required fix**
Define ownership/conflict behavior explicitly:
- how launch behaves if bridge name already active under different owner (pod vs standalone or different pod),
- whether sharing is supported,
- exact operator-visible error/status behavior.

Prefer reject-with-clear-error over implicit takeover unless takeover is explicitly requested.

---

### P1 - Missing failure/rollback semantics for partial pod launch with bridges

**Problem**
The launch sequence starts agents, then forks bridges ([docs/plan-pod-simplification.md:265](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:265)-[271](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:271)), but the plan does not define what happens if one of multiple bridge forks fails after agents are up.

Without explicit failure semantics, pod launch can leave partial runtime state (some agents running, some bridges absent/stopped, ownership metadata half-written), making retry/stop behavior ambiguous.

**Required fix**
Add transactional behavior for launch:
- success criteria,
- rollback strategy on bridge failure (which processes are stopped and in what order),
- idempotent retry behavior,
- surfaced error message/exit code.

Add an integration test for "N bridges configured, bridge K fails to fork" and verify final system state.

---

### P2 - Concierge liveness state transitions are underspecified for manual concierge changes

**Problem**
The plan keeps `set-concierge`/`remove-concierge` behavior "as before" while introducing `conciergeAlive` gating ([docs/plan-pod-simplification.md:287](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:287)-[295](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:295), [docs/plan-pod-simplification.md:391](/Users/dcosson/h2home/projects/h2/docs/plan-pod-simplification.md:391)).

State initialization/update rules for manual `set-concierge` are not specified (for example: should `conciergeAlive` be set false immediately and require next poll, or probed synchronously?). This can create transient misrouting or stale status messaging.

**Required fix**
Document exact state transition rules for manual concierge updates:
- initial `conciergeAlive` value after set/remove,
- whether to perform immediate probe,
- routing behavior during transition window,
- status message sequence.

Add service unit tests for manual set/remove interactions with `resolveDefaultTarget`.

---

## Summary

4 findings: 0 P0, 3 P1, 1 P2, 0 P3

**Verdict**: Approved with revisions
