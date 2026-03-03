---
name: plan-work-completion-signoff
description: Verify implemented plan docs against their actual code, add completion signoff metadata, and surface any gaps as new beads. Designed for scheduler/concierge agents coordinating multi-agent signoff after implementation is done.
user-invocable: true
allowed-tools: Bash Read Write Edit Grep Glob Task
argument-hint: "[plans-dir] [code-base-path]"
---

# Plan Work Completion Signoff

Verify that implemented plan docs match their actual code. For each plan doc, an agent compares every specified feature, API, type, data structure, and test category against the real implementation. Complete docs get a signoff section appended; docs with gaps generate a report so the orchestrator can create follow-up beads.

This skill is a structured decision framework for the scheduler/concierge agent — it orchestrates multi-agent verification work. Individual agents do the actual comparison and signoff.

## Inputs

- `$0` (optional): Plans directory (default: `docs/plans/`)
- `$1` (optional): Code base path (default: repo root)

## Phase 1: Discover Implemented Plans

1. Read the plan index (`docs/plans/00-plan-index.md` or equivalent)
2. Identify which plan docs have been **implemented** — look for:
   - Closed implementation beads/epics referencing those plans
   - Existing code packages that correspond to plan components
   - Plan doc status markers (e.g., "Implementation complete" in the index)
3. Build a list of plan doc pairs to verify: each plan doc + its companion test harness doc (if exists)
4. Exclude docs that already have a `## Completion Signoff` section (already verified in a prior pass)

Decision: Communicate the discovered doc list to the user or concierge for confirmation before proceeding. If the list looks wrong (too many or too few docs), clarify before creating beads.

## Phase 2: Create Beads and Assign Agents

1. Create an epic bead: `bd create "Plan completion signoff" --type epic --labels project={project}`
2. Group plan docs into tasks — aim for 2-4 docs per task, grouped by component area:
   - Group a plan doc with its companion test harness doc in the same task
   - Related components can share a task (e.g., a storage layer plan + its test harness plan)
   - Don't make tasks too small (one doc each) or too large (8+ docs)
3. Create task beads under the epic, one per group
4. Assign tasks to available agents. Prefer agents who:
   - Wrote the implementation (they know the code best)
   - Reviewed the implementation (they know the gaps)
   - If original agents are unavailable, any agent can do it — the plan docs and code are self-documenting

Each task bead description should include:
- The specific doc paths to verify
- The corresponding code packages to compare against
- The signoff section format (see Phase 3)
- Instructions to report gaps via h2 message

## Phase 3: Agent Verification Work

Each assigned agent does the following for every doc in their task:

### Step 1: Read the Plan Doc

Read the plan doc thoroughly. Build a mental checklist of:
- Every specified type, struct, interface, or enum
- Every specified function, method, or API endpoint
- Every specified algorithm or protocol
- Every specified configuration option or tunable
- Every specified error handling path
- Every specified test category (for test harness docs)
- Every URP/EO/AA claim

### Step 2: Compare Against Code

For each checklist item:
1. Search the codebase for the corresponding implementation
2. Verify the implementation matches the spec (names, signatures, behavior)
3. Note any deviations — things implemented differently than specified
4. Note any missing items — things specified but not implemented
5. Note any extras — things implemented but not in the spec (these are fine, just document them)

For test harness docs, additionally verify:
- Each specified test category has corresponding test files/functions
- Test coverage matches what was planned (e.g., "10 property-based tests" → are there actually 10?)
- CI integration works as specified (e.g., `make test-*` commands run the right tests)

### Step 3: Run Verification Tests

Run the relevant test suite to confirm everything passes:
```bash
go test ./path/to/package/... -count=1
```

For race-sensitive packages, also run:
```bash
go test -race ./path/to/package/... -count=1
```

### Step 4: Add Signoff or Report Gaps

**If the plan is fully implemented** (possibly with minor, documented deviations), append this section at the very bottom of the doc:

```markdown
---

## Completion Signoff

- **Status**: Complete
- **Date**: {YYYY-MM-DD}
- **Branch**: {branch-name}
- **Commit**: {HEAD commit hash}
- **Verified by**: {agent-name}
- **Test verification**: `{test command}` — PASS
- **Deviations from plan**:
  - {List any ways the implementation differs from the spec, or "None"}
- **Additions beyond plan**:
  - {List any features implemented that weren't in the original spec, or "None"}
```

**If there are significant gaps** (missing features, unimplemented test categories, broken tests), do NOT add a signoff section. Instead:
1. Add a partial signoff noting what IS complete:

```markdown
---

## Completion Signoff

- **Status**: Partial
- **Date**: {YYYY-MM-DD}
- **Branch**: {branch-name}
- **Verified by**: {agent-name}
- **Completed items**: {list of what's done}
- **Outstanding gaps**:
  - {Gap 1: description, severity estimate}
  - {Gap 2: description, severity estimate}
```

2. Report the gaps to the orchestrating agent via `h2 send` with specifics:
   - What's missing
   - How significant it is (blocking vs nice-to-have)
   - Suggested bead description for the follow-up work

### Step 5: Commit and Report

1. Commit the updated plan docs: `git add docs/plans/ && git commit -m "docs: add completion signoff for {doc-names}"`
2. Push to the working branch
3. Report completion to the orchestrating agent via `h2 send`:
   - Which docs were signed off
   - Which docs have gaps (if any)
   - Commit hash

## Phase 4: Orchestrator Processes Results

As agents report back, the orchestrating agent:

1. **For complete signoffs**: Close the task bead
2. **For gaps**: Evaluate each reported gap:
   - Is it real missing work, or an intentional deviation / future scope?
   - If real: Create a new task bead for the follow-up work, assign to an available agent
   - If intentional: Update the signoff to "Complete" with the deviation documented
3. Track progress: how many docs signed off vs gaps remaining
4. When all task beads are closed (signoffs done, follow-up beads created for any gaps), close the epic

## Phase 5: Report to User/Concierge

Send a final summary:
- Total plan docs verified: N
- Fully complete: N
- Complete with deviations: N (list deviations)
- Gaps requiring follow-up: N (list beads created)
- All tests passing: yes/no

## Beads Integration

```
Epic: "Plan completion signoff"
  ├── Task: "Signoff: {component-group-1} ({doc-list})"
  ├── Task: "Signoff: {component-group-2} ({doc-list})"
  ├── ...
  └── (follow-up beads created for any gaps found)
```

## Checking Status

A companion script reports signoff status across all plan docs:

```bash
python3 "$(dirname "$0")/signoff-status.py" docs/plans/
```

Output formats:
- `--format text` (default): Human-readable summary with counts and per-doc status
- `--format json`: Machine-readable for CI integration
- `--format markdown`: Markdown table for embedding in reports

The script traverses the plans directory (recursing into subdirectories), finds all plan docs (excluding index, architecture, shaping, summary, and review files), and checks each for a `## Completion Signoff` section. It reports:
- Total docs, complete count, partial count, not-started count
- Per-doc details: path, type (plan vs test-harness), status, date, verified-by
- Type breakdown: plan docs vs test harness docs

Exit code 0 on success; exit code 1 only on errors (e.g., invalid directory).

Run this after the signoff process to verify everything is covered, or in CI to gate merges on plan completion.

## What Requires Judgment

The orchestrating agent makes these calls:

1. **Which docs to include** — only implemented plans, not future/unstarted plans
2. **How to group docs into tasks** — balance between parallelism and cognitive coherence
3. **Whether a gap is real work or intentional** — not every plan deviation needs a follow-up bead
4. **How to handle unavailable agents** — reassign to whoever is online
5. **Whether to re-verify after gap fixes** — if follow-up beads are created, should the signoff be re-run after they're closed?
