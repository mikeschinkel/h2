---
name: plan-draft
description: Write a detailed plan doc and its companion test harness doc for a single component. Use after plan-architect has created the plan index.
user-invocable: true
allowed-tools: Bash Read Write Edit Grep Glob WebSearch WebFetch Task
argument-hint: "[doc-name] [scope-description]"
---

# Plan Draft

Write a plan doc and its companion test harness doc for a single component.

## Inputs

- `$0`: Doc identifier (e.g., `04d-oltp-sql-engine`)
- `$1` (optional): Scope description — what the plan should cover
- Plans directory: `docs/plans/` (or the project's established plans directory)

## Phase 1: Read Context

1. Read `docs/plans/00-plan-index.md` to understand dependencies and scope
2. Read `docs/plans/00-architecture.md` for architectural context
3. Read all prerequisite/dependency docs listed in the plan index for this component
4. Read any shaping docs or API contracts referenced

## Phase 2: Write Plan Doc

Write `docs/plans/$0.md` following project conventions:

- Architecture summary with mermaid diagrams (component, sequence, state where relevant)
- Package/module structure and import flow
- API/interface definitions (fully specified, not hand-waved)
- Major types, structs, components with properties and key methods
- Detailed design for all non-trivial algorithms and protocols
- Connected Components — list which other components this plan connects to (its seams), with the interface at each boundary (function signatures, wire protocols, shared types, message formats)
- Acceptance Criteria — 3-8 end-user-facing scenarios that prove the component works in the real system, not just in isolation. Each scenario must cross at least one component boundary. Format: scenario name, steps using the end-user interface (CLI, API, web UI, mobile — whatever the product exposes), expected outcome. These are NOT internal API tests; they verify behavior as a user would experience it.
- Testing section (unit, component, integration strategy). For every test category, specify: (1) where test files will live (exact package/directory paths), (2) which make target or test suite they roll up into, (3) whether they run in CI PR checks, nightly, or on-demand. Tests without a home and a runner don't get run.
- URP (Unreasonably Robust Programming) section — concrete commitments, not wishlists. Each item must specify exactly what will be built, where it fits in the implementation, and how it will be tested. If it's not concrete enough to implement directly from the plan, cut it or make it concrete.
- Extreme Optimization section — same standard: commit to specific techniques with design details, or cut them.
- Alien Artifacts section — same standard: commit to specific techniques with design details, or cut them.

Commit the plan doc and record the hash.

## Phase 3: Write Test Harness Doc

Write `docs/plans/$0-test-harness.md` covering:

- Property-based tests (invariants that must always hold)
- Fault injection / chaos engineering tests
- Comparison oracle tests (if a reference implementation exists)
- Deterministic simulation tests
- Benchmarks and performance tests (with specific targets)
- Stress / soak tests (long-running stability)
- Security tests (if applicable)
- Manual QA plan (tests requiring human/agent judgment)
- CI tier mapping (which tests run in which CI stage)
- Exit criteria (what must pass before implementation is considered done)

For every test category above, specify: (1) where test files will live in the codebase (exact package/directory paths), (2) which make target they roll up into, (3) whether they run in CI PR checks, nightly, or on-demand only. A test described without a location and runner is not a real commitment.

Commit the test harness doc and record the hash.

## Phase 4: Report

Report both commit hashes. The docs are now ready for `/plan-review`.
