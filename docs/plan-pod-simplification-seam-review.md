# Seam Review: pod-simplification (horizontal) — h2-coder-2

- Mode: horizontal (single plan doc, checking internal component seams against existing codebase interfaces)
- Seam: pod-bridge-config-integration
- Reviewed commit: 9f8d95d
- Reviewer: h2-coder-2
- Plan docs reviewed:
  - `docs/plan-pod-simplification.md`
- Existing code reviewed:
  - `internal/bridgeservice/fork.go` — ForkBridge interface
  - `internal/cmd/bridge_daemon.go` — _bridge-service daemon command
  - `internal/cmd/bridge.go` — resolveUser(), bridge create flow
  - `internal/bridgeservice/service.go` — Service struct, concierge tracking
  - `internal/session/message/protocol.go` — BridgeInfo struct
  - `internal/config/config.go` — Config/UserConfig/BridgesConfig structs
  - `internal/bridgeservice/factory.go` — FromConfig()
  - `internal/cmd/agent_setup.go` — setupAndForkAgentQuiet
  - `internal/config/override.go` — ApplyOverrides

## Seam Boundaries Analyzed

### Seam: Pod Launch → ForkBridge → Bridge Daemon

**Plan doc**: §4 (Pod Launch Bridge Integration)
**Existing code**: `bridgeservice/fork.go`, `cmd/bridge_daemon.go`

| Category | Status | Details |
|----------|--------|---------|
| Interface signatures | FAIL | ForkBridge(user, concierge) takes a `user` string. Plan requires passing a named bridge config key, not a user name. |
| Data formats/types | FAIL | Daemon uses `--for <user>` flag and calls `resolveUser()` to get `UserConfig.Bridges`. Plan removes `UserConfig.Bridges` entirely. |
| Lifecycle ordering | PASS | Plan correctly specifies bridges launch after agents. |
| Configuration contracts | FAIL | Socket path uses `bridge.<user>.sock` naming. Plan introduces named bridge configs which are not users — socket naming scheme undefined. |
| Error handling | PASS | Plan specifies failure semantics for bridge fork. |

---

### Seam: Bridge Daemon → Config Resolution

**Plan doc**: §3 (Global Bridge Config Extraction)
**Existing code**: `cmd/bridge_daemon.go`, `config/config.go`, `bridgeservice/factory.go`

| Category | Status | Details |
|----------|--------|---------|
| Interface signatures | FAIL | `FromConfig(cfg *config.BridgesConfig)` takes a single `BridgesConfig`. Plan changes `Config.Bridges` to `map[string]*BridgesConfig`. The daemon needs a new flag to select which named config to use. |
| Data formats/types | PASS | `BridgesConfig` struct itself is unchanged (contains Telegram, MacOSNotify). |
| Lifecycle ordering | PASS | No ordering issues. |
| Configuration contracts | FAIL | Plan says replace `resolveUser()` with `resolveBridgeConfig(cfg, name)` but doesn't specify the daemon-side changes needed — daemon must also switch from `--for <user>` to `--bridge <name>`. |
| Error handling | PASS | N/A |

---

### Seam: Bridge Service → BridgeInfo (Pod Tracking)

**Plan doc**: §4 (pod tracking via BridgeInfo.Pod)
**Existing code**: `session/message/protocol.go`, `bridgeservice/service.go`

| Category | Status | Details |
|----------|--------|---------|
| Interface signatures | PASS | Adding `Pod string` to BridgeInfo and Service is straightforward. |
| Data formats/types | PASS | JSON serialization will pick up new field automatically. |
| Lifecycle ordering | PASS | N/A |
| Configuration contracts | FAIL | Plan says add Pod to bridge daemon's args but doesn't specify the flag name, and doesn't update ForkBridge signature to accept pod. |
| Error handling | PASS | N/A |

---

### Seam: Pod Launch → ApplyOverrides → setupAndForkAgentQuiet

**Plan doc**: §2 (Overrides mechanism)
**Existing code**: `config/override.go`, `cmd/agent_setup.go`

| Category | Status | Details |
|----------|--------|---------|
| Interface signatures | PASS | ApplyOverrides(role, []string) matches plan's described usage. |
| Data formats/types | PASS | map[string]string → []string conversion is trivial. |
| Lifecycle ordering | PASS | Plan correctly applies overrides before fork. |
| Configuration contracts | PASS | setupAndForkAgentQuiet's `overrides` param records metadata only (comment: "caller is responsible for applying overrides"). Pod launch should still pass the override strings for RuntimeConfig recording. |
| Error handling | PASS | N/A |

---

### Seam: Bridge Service → Concierge Auto-Reassociation

**Plan doc**: §5 (Bridge Concierge Auto-Reassociation)
**Existing code**: `bridgeservice/service.go`

| Category | Status | Details |
|----------|--------|---------|
| Interface signatures | PASS | Changes are internal to Service struct. |
| Data formats/types | PASS | New `conciergeAlive` field is internal state. |
| Lifecycle ordering | PASS | State transitions well-specified after R1 incorporation. |
| Configuration contracts | PASS | N/A |
| Error handling | PASS | Fallback routing specified. |

## Acceptance Criteria Cross-Reference

| Acceptance criterion (source) | Seams touched | Status | Details |
|-------------------------------|---------------|--------|---------|
| Pod launch starts bridges from pod YAML | PodLaunch→ForkBridge→Daemon | FAIL | ForkBridge interface mismatch — cannot pass named bridge config through current interface |
| Pod stop also stops pod bridges | BridgeService→BridgeInfo | FAIL | Pod field must flow through ForkBridge→daemon→Service→BridgeInfo, but ForkBridge doesn't accept pod param |
| Bridge auto-reassociates concierge | Service internal | PASS | Self-contained within bridge service |
| Inline role overrides work | PodLaunch→ApplyOverrides→Fork | PASS | Interface compatible, metadata recording needs override strings passed through |

## Seam Compatibility Matrix

| Seam | Signatures | Data | Lifecycle | Config | Errors | Overall |
|------|-----------|------|-----------|--------|--------|---------|
| PodLaunch → ForkBridge → Daemon | FAIL | FAIL | PASS | FAIL | PASS | FAIL |
| Daemon → Config Resolution | FAIL | PASS | PASS | FAIL | PASS | FAIL |
| Service → BridgeInfo (Pod) | PASS | PASS | PASS | FAIL | PASS | FAIL |
| PodLaunch → Overrides → Fork | PASS | PASS | PASS | PASS | PASS | PASS |
| Service → Concierge Reassoc | PASS | PASS | PASS | PASS | PASS | PASS |

## Findings

### P1 [IG] - ForkBridge interface incompatible with named bridge configs

**Seam**: Pod Launch → ForkBridge → Bridge Daemon
**Category**: Interface signatures, Data formats, Configuration contracts

**Problem**
The plan specifies pod launch calls `ForkBridge(bridgeName, concierge)` (§4 line 270), but the existing `ForkBridge(user, concierge string)` signature passes `user` to the daemon via `--for <user>`. The daemon then calls `resolveUser()` to look up `UserConfig.Bridges`. The plan removes `UserConfig.Bridges` entirely (§3), but does not specify:

1. New `ForkBridge` signature (needs bridge config name, pod name)
2. New daemon flags (`--bridge <name>` replacing `--for <user>`, plus `--pod <name>`)
3. How the daemon resolves the named bridge config internally
4. New socket naming scheme — currently `bridge.<user>.sock`, but with named configs the socket should be `bridge.<config-name>.sock`

All four touch points (ForkBridge caller, ForkBridge implementation, daemon command, socket naming) must change in lockstep or the bridge won't start.

**Required fix**
Add to §3 or §4:
- New `ForkBridge(bridgeName, concierge, pod string) error` signature
- New daemon flags: `--bridge <name>`, `--concierge <name>`, `--pod <name>` (replacing `--for`)
- Daemon config resolution: `cfg.Bridges[bridgeName]` direct map lookup
- Socket naming: `bridge.<bridgeName>.sock`
- Update `stopExistingBridgeIfRunning` to use bridge name instead of user

---

### P2 - Override strings should be passed to setupAndForkAgentQuiet for RuntimeConfig recording

**Seam**: Pod Launch → ApplyOverrides → setupAndForkAgentQuiet
**Category**: Configuration contracts

**Problem**
The plan says to call `ApplyOverrides(role, overrideSlice)` then pass the overridden role to `setupAndForkAgentQuiet()` (§2 lines 207-211). However, `setupAndForkAgentQuiet` also accepts an `overrides []string` parameter which it parses into `RuntimeConfig.Overrides` for metadata recording. If the pod launch passes `nil` for overrides (since it already applied them), the RuntimeConfig won't record what overrides were applied.

The plan should specify that pod launch passes the override strings to `setupAndForkAgentQuiet` for metadata recording, even though they've already been applied to the role. The comment on line 70 of `agent_setup.go` confirms: "The caller is responsible for loading the role and applying any overrides" — the overrides param is purely for metadata.

**Required fix**
Update §2 step 4 to: "Pass the overridden role **and** the override strings to `setupAndForkAgentQuiet()` so RuntimeConfig records applied overrides."

---

## Summary

5 seam boundaries analyzed, 2 findings: 0 P0, 1 P1, 1 P2, 0 P3

Seam compatibility: 2/5 seams fully compatible

**Verdict**: Seams compatible with revisions — the P1 is a documentation gap (the plan doesn't specify how the existing ForkBridge/daemon interface changes to support named configs), not a fundamental design flaw. The required interface changes are mechanical once specified.
