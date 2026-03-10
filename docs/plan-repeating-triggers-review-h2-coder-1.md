# Review: plan-repeating-triggers (h2-coder-1)

- Source doc: `docs/plan-repeating-triggers.md`
- Test harness doc: `docs/plan-repeating-triggers-test-harness.md`
- Reviewed commit: ac503a1
- Reviewer: h2-coder-1

## Findings

### P1 - MaxFirings sentinel value contradictions throughout doc

**Problem**
The doc contains two conflicting definitions of what `MaxFirings=0` means:

1. **Summary (line 12)**: "0 = unlimited, 1 = one-shot default"
2. **Struct comment (lines 98)**: "0 = unlimited, 1 = one-shot (default)"
3. **CLI flags (line 326)**: "0=unlimited, default 1=one-shot"
4. **Acceptance criteria #2 (line 375)**: Uses `--max-firings 0` for unlimited behavior
5. **Revised sentinel values (lines 159-162)**: "0 = default (one-shot, same as 1)", "-1 = unlimited"
6. **effectiveMaxFirings() (lines 149-155)**: Returns 1 when MaxFirings==0 (treats 0 as one-shot)

The "revised sentinel" section and the code are internally consistent (-1=unlimited, 0=default), but the Summary, struct comment, CLI help text, and acceptance criteria #2 all use the OLD convention (0=unlimited). An implementor following the Summary would build the opposite behavior from what the code pseudocode specifies.

**Required fix**
Pick one convention and apply it consistently everywhere. The revised sentinel (-1=unlimited, 0=default one-shot) is the safer choice because it avoids the zero-value footgun. Update:
- Summary line 12: change to "(−1 = unlimited, 0/1 = one-shot default)"
- Struct comment line 98: change to "−1 = unlimited, 0 = default (one-shot), N > 0 = fire N times"
- CLI help line 326: change to "−1=unlimited, default 1=one-shot"
- Acceptance criteria #2 line 375: change `--max-firings 0` to `--max-firings -1`
- TriggerSpec comment line 115: change to match revised sentinel

---

### P1 - processEvent races with evalAndFire on trigger removal

**Problem**
In the `processEvent` pseudocode (lines 236-254), triggers matching the event are collected under the lock, then `evalAndFire` is called outside the lock. `evalAndFire` also acquires the lock to check existence and increment `FireCount`. However, between `processEvent` releasing the lock and `evalAndFire` acquiring it, another concurrent `processEvent` call could also collect the same trigger in its matched list. This is handled by the `_, existed := te.triggers[t.ID]` check in evalAndFire — good.

But there's a subtler issue: the expiry reaping in `processEvent` deletes triggers under the lock, while `evalAndFire` also does expiry deletion under a separate lock acquisition. If a trigger expires between `processEvent`'s reaping pass and `evalAndFire`'s expiry check, it's fine (evalAndFire checks again). But the `t *Trigger` pointer in the `matched` slice still points to the trigger struct after it's been deleted from the map. Since `evalAndFire` checks existence, this is safe, but the doc should explicitly note that the matched slice may contain stale pointers to already-reaped triggers and that the existence check in `evalAndFire` handles this.

**Required fix**
Add a brief comment in the `evalAndFire` pseudocode at the existence check (line 203-206) noting that this handles the concurrent-reap and concurrent-fire cases. This prevents an implementor from "optimizing" away the check.

---

### P2 - triggerFromSpec silently swallows parse errors

**Problem**
In the `triggerFromSpec` conversion (lines 262-277), if `time.Parse(time.RFC3339, s.ExpiresAt)` or `time.ParseDuration(s.Cooldown)` fail, the error is silently dropped and the field is left as zero-value. This means a typo like `expires_at: "2026-03-09T25:00:00Z"` or `cooldown: "5 minutes"` would silently create a trigger with no expiry/cooldown — the opposite of what the user intended.

**Required fix**
`triggerFromSpec` should return an error on parse failure. The caller (`handleTriggerAdd` in listener.go) already returns error responses to the socket client, so this fits naturally. Update the pseudocode to return `(*automation.Trigger, error)`.

---

### P2 - No input validation for MaxFirings and Cooldown ranges

**Problem**
Nothing prevents `MaxFirings = -5` or `Cooldown = -30s`. Negative cooldown would cause `now.Sub(t.LastFiredAt) < t.Cooldown` to always be true (if LastFiredAt is set), permanently blocking the trigger after the first firing.

**Required fix**
Add validation in `triggerFromSpec` (or a `Trigger.Validate()` method called on Add):
- `MaxFirings >= -1`
- `Cooldown >= 0`
- `ExpiresAt` is either zero or in the future (warn if in the past, but don't reject — it'll just be reaped immediately)

---

### P2 - Test harness assumes controllable clock but plan has no clock abstraction

**Problem**
The test harness doc (lines 53-55, 76-81) specifies "mock clock", "controllable clock", and "simulate time jumping forward". The plan uses `time.Now()` directly in both `evalAndFire` and `processEvent`. Without a clock interface, the property-based tests and deterministic simulation tests described in the test harness cannot be implemented as specified.

**Required fix**
Either:
(a) Add a `clock` field to `TriggerEngine` (interface with `Now() time.Time`), defaulting to `time.Now`, injectable in tests. This is a ~10 line change.
(b) Or update the test harness doc to not require mock clocks — use real time with small durations (50ms cooldowns, etc.) and accept test flakiness.

Option (a) is strongly preferred for determinism.

---

### P2 - specFromTrigger omits Cooldown in list response

**Problem**
In the `specFromTrigger` pseudocode (lines 280-296), Cooldown is only included when `t.Cooldown > 0`. This is correct for omitempty, but the `trigger list` CLI output (line 331) shows a COOLDOWN column. If cooldown is zero, the column would be empty, which is fine — but the `Cooldown` field on TriggerSpec uses a string type. The `trigger list` command should display "0s" or "-" for no cooldown, not an empty string.

**Required fix**
Minor: specify the display format for zero-value cooldown in the trigger list table. Recommend "-" for no cooldown.

---

### P3 - resolveExpiresAt location

**Problem**
The plan places `resolveExpiresAt` in `daemon.go` (line 304). This function is a pure time-parsing utility with no daemon dependencies. Placing it in daemon.go mixes concerns.

**Required fix**
Consider placing `resolveExpiresAt` in `internal/automation/automation.go` or a shared utility, since it may also be needed by the CLI's `--expires-at` flag handler (which also accepts relative timestamps per line 329).

---

### P3 - Missing test for concurrent Add during processEvent

**Problem**
The unit test list (lines 389-402) doesn't include a test for adding a new trigger while `processEvent` is iterating the map. Go maps are not safe for concurrent read+write. The existing locking should handle this, but a `-race` test specifically targeting Add-during-processEvent would catch regressions.

**Required fix**
Add a test: `TestTriggerEngine_ConcurrentAddDuringProcessEvent` — one goroutine sends events continuously while another adds triggers. Run with `-race`.

---

## Summary

8 findings: 0 P0, 2 P1, 4 P2, 2 P3

**Verdict**: Approved with revisions

The MaxFirings sentinel contradiction (P1) is the most critical finding — it affects the Summary, struct definition, CLI help, and acceptance criteria. An implementor could reasonably build the wrong behavior by following the Summary instead of the "revised sentinel" section. The other P1 is documentation-only (add a comment about concurrent safety). The P2s are all straightforward fixes that strengthen correctness and testability.
