# Diet Tracker v2: Weekly Check-In System Design Reference

**Created:** 2026-04-30  
**Source:** Obsidian Vault Analysis  
**Status:** Active Design Documentation  
**Vault Root:** `/home/dfgoodfellow2/Obsidian/Brain/`

---

## Quick Navigation

### Core Architecture
- **Project Location:** `/home/dfgoodfellow2/Projects/Personal/Diet_Tracker/v2/`
- **Module:** `github.com/dfgoodfellow2/diet-tracker/v2`
- **Tech Stack:** Go 1.22+ API + SQLite + Svelte 5 PWA + Bubble Tea TUI
- **Deployment:** fly.io

### Key Design Docs in Vault
| Document | Path | Purpose |
|----------|------|---------|
| **INDEX** | `projects/diet-program/INDEX.md` | Architecture overview, feature checklist |
| **Adaptive TDEE** | `projects/diet-program/adaptive-tdee-calorie-targets.md` | ⭐ **CURRENT SPEC** for weekly check-in logic |
| **Calculator** | `projects/diet-program/calculator.md` | TDEE methods, NDS, macro algorithms |
| **Research Notes** | `projects/diet-program/research-notes.md` | MacroFactor philosophy, weight-delta TDEE |
| **API Server** | `Projects/diet-project2/api-server.md` | 35 endpoints, handler structure |
| **Tracker Plan** | `projects/diet-program/tracker-plan.md` | ⚠️ ARCHIVED — superseded by adaptive-tdee-calorie-targets |

---

## Weekly Check-In System Design (CURRENT)

### What It Does
The weekly check-in computes an **observed TDEE** from the user's actual eating and weight patterns over the last 14 days, then intelligently adjusts calorie and macro targets while protecting the user from metabolic shock.

### Key Flow

```
1. User initiates weekly check-in (last 5+ days logged)
    ↓
2. Compute Observed TDEE (tiered: Lin Reg / RLS / EMA based on days of data)
    ↓
3. Calculate Ideal Target = Observed TDEE ± Exercise Calories
    ↓
4. Apply Damping Filter (max ±500 kcal/week swing)
    ↓
5. Enforce 1200 kcal/day floor
    ↓
6. Recalculate macros from final damped calories
    ↓
7. Display old → new with emergency override warning if needed
    ↓
8. User accepts or skips; if accepted, update targets table + history
```

---

## 🔴 Critical Rules (From adaptive-tdee-calorie-targets.md)

### Rule 1: Observed Truth Priority
**"Observed TDEE > Profile Estimates"**

```
Truth Hierarchy:
  Observed TDEE (computed from your data)
    ↓ 
  Damping Filter (±500 kcal/week cap)
    ↓
  Final Target (1200 kcal floor)
    ↓
  Macro Distribution (Protein/Fat/Carbs/Fiber)
    ↓
  Profile Estimate (ONLY for: emergency override baseline, expected weight display)
```

**Code location:** `services/calculator.go` → `ComputeWeeklyTargetsWithExercise()`

---

### Rule 2: Max Weekly Calorie Swing = 500 kcal

**The Rule:**
> A single weekly check-in may NOT move the daily calorie target by more than ±500 kcal in either direction.

**Why?**
- Prevents metabolic shock from logging noise or anomalies
- Unfiltered adaptive TDEE can swing 800–1200 kcal due to cheat days, travel, illness
- 500 kcal/week cap allows 2-week convergence for 1000 kcal gaps
- User stays stable enough to adapt; system stays responsive

**Implementation (lines 506–541 in calculator.go):**

```go
const MaxWeeklyCalorieSwing = 500  // Named constant, no magic numbers

// In ComputeWeeklyTargetsWithExercise():
delta := idealTarget - currentTarget

if delta > MaxWeeklyCalorieSwing {
    clampedDelta = MaxWeeklyCalorieSwing
} else if delta < -MaxWeeklyCalorieSwing {
    clampedDelta = -MaxWeeklyCalorieSwing
} else {
    clampedDelta = delta
}

newCalories := currentTarget + clampedDelta
```

**Edge cases handled:**
- ✅ If `idealTarget == currentTarget`: delta = 0, no change
- ✅ If `delta > 500`: only 500 kcal added (not the full gap)
- ✅ If `delta < -500`: only 500 kcal removed (not the full gap)
- ✅ Floor guard: `newCalories` cannot fall below 1200 kcal

---

### Rule 3: Macros Always Recalculated from Final Damped Calories

**The Rule:**
> Macros (Protein/Fat/Carbs) are ALWAYS computed from the **final damped calorie number**, never from the ideal/pre-damping number.

**Why?**
Ensures macro ratios stay proportional to the *actual* energy target the user will follow. If user targets 1800 kcal post-damping but we compute macros from 2200 kcal ideal, macros will be overcalculated.

**Implementation:**

```go
finalCalories := newCalories

// Apply minimum floor
if finalCalories < 1200 {
    finalCalories = 1200
}

// Recompute macros from final (damped) calories
recomputed := ComputeTargets(float64(finalCalories), profile)
if recomputed != nil {
    targets = *recomputed
    targets.Calories = finalCalories  // Enforce exact value after goal multiplier
}
```

**Why the `targets.Calories = finalCalories` override?**
- `ComputeTargets(tdee, profile)` applies goal multiplier: `tdee * goalMultiplier`
- For maintenance (mult=1.0): no-op. For cut/bulk: rescales the already-adjusted value
- Override ensures macros are computed at right energy level while exact final calorie is preserved

---

### Rule 4: Maintenance Goal Special Case

**The Problem:**
User on 'maintenance' goal whose observed TDEE is higher than profile-estimated TDEE would receive a calorie target based on lower estimate → unintentional deficit.

**The Fix (lines 501–513 in calculator.go):**

```go
var idealTarget int

if profile != nil && profile.Goal == "maintenance" {
    // For maintenance: use observed TDEE directly (no goal multiplier)
    idealTarget = observedTDEE + dailyExerciseCals
} else {
    // For cut/bulk: apply eat-back logic
    if eatBackExercise {
        idealTarget = observedTDEE + dailyExerciseCals
    } else {
        idealTarget = observedTDEE - dailyExerciseCals
    }
}
```

**Effect:**
- If Observed TDEE > Profile Estimate → targets.Calories is *raised* ✅
- If Observed TDEE < Profile Estimate → targets.Calories is *lowered* ✅
- Non-maintenance goals still get the goal multiplier via `ComputeTargets()`

---

### Rule 5: Emergency Override (15% Threshold)

**The Rule:**
If observed TDEE diverges from estimated TDEE by **>15%** AND confidence is not "low", unlock the 5-day timing gate to allow immediate target changes.

**Single Source of Truth (lines 580–593 in calculator.go):**

```go
func ShouldTriggerEmergencyOverride(observedTDEE int, estimatedTDEE int, confidence string) bool {
    if observedTDEE <= 0 || estimatedTDEE <= 0 {
        return false
    }
    if confidence == "low" {
        return false  // Never override with low confidence
    }
    tdeeDiffPct := math.Abs(float64(observedTDEE-estimatedTDEE)) / float64(estimatedTDEE)
    return tdeeDiffPct > 0.15
}
```

**What It Unlocks:**
- Normally: must wait 5 days between check-ins
- With override: can check-in immediately
- UI displays warning-coloured status with exact % divergence and kcal values
- Footer: "Emergency override: accepting will apply new targets"

**Important:** Emergency override does **NOT** bypass damping. The 500 kcal/week swing cap is always active.

---

## Observed TDEE Computation (Tiered Method)

**Source:** `ComputeObservedTDEETiered()` in `services/calculator.go`

The system uses **logged calorie history** (not weight-delta formula), making it adherence-sensitive but fast to converge for consistent trackers.

| Days of Data | Method | Confidence | Use Case |
|---|---|---|---|
| 1–3 days | Linear Regression | LOW | Just started tracking |
| 4–6 days | Linear Regression | MEDIUM | Getting traction |
| 7–13 days | RLS Filter (λ=0.98) | MEDIUM | Good data, still volatile |
| 14–29 days | RLS Filter (λ=0.98) | HIGH | Strong foundation |
| ≥30 days | EMA (α=0.10) | HIGH | Stable, long-term view |

**Input:** Smoothed average of **logged calories** (not weight-delta-derived TDEE)

**Known Limitation (LOW priority):**
The tiered method estimates TDEE from *intake* alone, not from classic `weight_delta * 3500 / days` formula. This means:
- For cut/bulk goals: will underestimate or overestimate true TDEE
- Does not affect correctness of macro scaling, damping, or override logic
- Status: accepted design tradeoff for simplicity

---

## Eat-Back Toggle Cache Invalidation

When user toggles "Eat Back Exercise" on the check-in screen:

```go
case "left", "right":
    if s.focusIndex == 1 {
        prev := s.eatBackExercise
        s.eatBackExercise = !s.eatBackExercise
        if s.computedTargets != nil && prev != s.eatBackExercise {
            s.computedTargets = nil   // ← Cache cleared
        }
    }
```

This forces a recompute on the next `View()` render to ensure targets reflect the new setting.

---

## API Endpoint: Weekly Check-In Calculation

**Endpoint:** `GET /v1/dashboard`

Returns aggregated data for the dashboard/check-in screen:

```json
{
  "profile": { ... },
  "tdee_observed": 2100,
  "tdee_estimated": 2000,
  "confidence": "high",
  "target_calories": 1680,
  "target_macros": {
    "protein_g": 168,
    "fat_g": 56,
    "carbs_g": 168,
    "fiber_g": 28
  },
  "weekly_stats": { ... },
  "can_change_targets": false,  // false if < 5 days since last check-in
  "days_until_checkin": 2
}
```

Also available:
- `GET /v1/calc/tdee?days=N` — Get observed TDEE for last N days
- `GET /v1/calc/macros` — Get macro targets for current goal
- `PUT /v1/targets` — Save new targets + snapshot old to history

---

## Safety & Stability Checklist

| Property | Status | Evidence |
|----------|--------|----------|
| Max weekly calorie change | ✅ SAFE — ≤500 kcal | `MaxWeeklyCalorieSwing = 500` + clamp logic |
| Minimum calorie floor | ✅ SAFE — ≥1200 kcal/day | Double floor check (pre + post damping) |
| Macros consistent with final calories | ✅ SAFE | `ComputeTargets()` called on `finalCalories` (post-damping) |
| Emergency override bypasses damping | ✅ NO — damping always active | Override only removes 5-day timing gate |
| Magic numbers in damping | ✅ NONE — all named constants | No hardcoded values in calc logic |
| Single source of truth for override | ✅ YES | `ShouldTriggerEmergencyOverride()` helper |

---

## Key Files & Locations

### Backend (Go)

| File | Lines | Purpose |
|------|-------|---------|
| `v2/services/calculator.go` | 600+ | `ComputeObservedTDEETiered()`, `ComputeWeeklyTargetsWithExercise()`, damping logic, `ShouldTriggerEmergencyOverride()` |
| `v2/services/nutrition/macros.go` | 200+ | `ComputeTargets()`, macro distribution (Protein/Fat/Carbs/Fiber) |
| `v2/internal/handlers/calculations.go` | 150+ | `/v1/calc/*` endpoint handlers (TDEE, Macros, Readiness, Dashboard) |
| `v2/internal/handlers/targets.go` | 100+ | `/v1/targets` GET/PUT, target history snapshots |
| `v2/internal/db/migrations/001_init.sql` | 300+ | DB schema: `macro_targets`, `target_history`, tables |

### Frontend (TUI — Bubble Tea)

| File | Lines | Purpose |
|------|-------|---------|
| `v2/cmd/tui/ui/checkin.go` | 400+ | Weekly check-in screen: display, toggle eat-back, accept/skip logic |
| `v2/cmd/tui/ui/dashboard.go` | 300+ | Dashboard: TDEE, macros, readiness, weekly stats |

### Frontend (PWA — Svelte)

| File | Lines | Purpose |
|------|-------|---------|
| `v2/ui/web/src/routes/dashboard/+page.svelte` | 200+ | Dashboard page: displays TDEE, targets, weekly stats |
| `v2/ui/web/src/routes/checkin/+page.svelte` | 200+ | Check-in page (PWA version of TUI check-in) |

---

## Related Vault Documents

### Active (Current Implementation)
- **[[adaptive-tdee-calorie-targets.md]]** ⭐ — CURRENT SPEC with all rules, damping logic, priority hierarchy
- **[[INDEX.md]]** — Project overview, architecture, feature checklist
- **[[calculator.md]]** — Phase 1 & 2 calculation algorithms (1RM, NDS, macro order-of-operations)
- **[[api-server.md]]** — All 35 endpoints, route map, handlers structure
- **[[go-rewrite-changelog]]** — Tiered TDEE implementation history

### Reference (Research)
- **[[research-notes.md]]** — MacroFactor philosophy, weight-delta TDEE methodology, calorie-per-lb guidelines
- **[[Workout-Logging 1.md]]** — Workout data integration with check-in (exercise calories for eat-back)

### Archived (Historical Context)
- **[[tracker-plan.md]]** — Original TUI design notes; TDEE algorithm section now outdated
- **[[tracker-built.md]]** — Python v1 implementation docs
- **[[calculator.md]]** — Phase 2 strength analysis (1RM, NDS formulas)

---

## Summary: Check-In Flow for Developers

```
[Weekly Check-In Initiated]
    │
    ├─ Fetch last 14 days nutrition logs + weight
    ├─ Compute Observed TDEE (ComputeObservedTDEETiered)
    ├─ Get Profile (goal, exercise data)
    ├─ Estimate TDEE (Mifflin + activity multipliers)
    │
    ├─ Calculate Ideal Target = Observed TDEE ± Exercise
    │   (Special case: if goal == maintenance, use observed directly)
    │
    ├─ [DAMPING] Clamp delta to ±MaxWeeklyCalorieSwing (500 kcal)
    ├─ [FLOOR] Enforce 1200 kcal/day minimum
    │
    ├─ [MACROS] ComputeTargets(finalCalories, profile)
    │   → Protein (lifter status, BF%, deficit, runner volume)
    │   → Fat (height-based minimum)
    │   → Carbs (remainder)
    │   → Fiber (per 1000 kcal)
    │
    ├─ [EMERGENCY CHECK] ShouldTriggerEmergencyOverride(observed, estimated, confidence)
    │   → If tdeeDiffPct > 15% AND confidence != "low": unlock 5-day gate
    │
    ├─ Display: old → new, delta, warnings, override status
    │
    └─ User Accept/Skip
        ├─ ACCEPT: Update macro_targets table + snapshot to target_history
        └─ SKIP: No changes
```

---

## Example Scenario

**User:** 75 kg female, 170 cm, 35yo, maintenance goal, lifting + running (20 mi/week)

**Last 14 days:**
- Avg logged intake: 2100 kcal/day
- Avg weight: 74.8 kg (slight downward trend)
- 3 workouts (lifting), 4 runs (20 mi total)

**Computation:**
1. Observed TDEE (from intake): 2100 kcal/day (confidence: high, 14 days data)
2. Exercise calories: 100 kcal/day average (eating back)
3. Ideal Target = 2100 + 100 = 2200 kcal
4. Current Target: 2000 kcal (from last check-in)
5. Delta: 2200 - 2000 = +200 kcal (within 500 kcal cap ✅)
6. Damped: 2200 kcal (no clamping needed)
7. Floor: 2200 kcal (above 1200 ✅)
8. Macros (from ComputeTargets at 2200 kcal):
   - Protein: 150 g (1.2 g/kg, runner adjusted)
   - Fat: 55 g (height-based min: ~50g, OK)
   - Carbs: 337 g (remainder, covers 20 mi/week carb need)
   - Fiber: 31 g (per 1000 kcal)
9. Emergency check: |2100 - 2050| / 2050 ≈ 2.4% (< 15%, no override)
10. UI: "2000 → 2200 kcal (+200). No warning. Accept?"

**User accepts** → targets updated, history logged.

---

## Troubleshooting Guide

### Q: Why is the check-in showing "Medium" confidence instead of "High"?
**A:** You have 7–13 days of data. RLS filter (λ=0.98) is active. Confidence is "medium" until you reach 14+ days.

### Q: The system won't let me check-in until 5 days have passed. Can I force it?
**A:** Only if emergency override triggers (>15% TDEE divergence + not low confidence). Otherwise, designed to prevent reactive adjustments from noisy data.

### Q: My target jumped +800 kcal. That's too much!
**A:** System should have damped to +500 kcal max. If not, check:
1. Was the last check-in more than 5 days ago? (if yes, damping resets)
2. Is your observed TDEE truly that different from your last target?
3. Consider running `ShouldTriggerEmergencyOverride()` manually to verify override logic

### Q: My macros don't add up to my calorie target.
**A:** 
- Macros are always recalculated from final damped calories post-floor
- Thermic Effect of Food (TEF ≈ 10%) is NOT subtracted from target
- Math should be: `protein*4 + carbs*4 + fat*9 ≈ calories` (within rounding)
- If off by >50 kcal, check `ComputeTargets()` for a bug

### Q: What if I'm on a cut goal but my observed TDEE is higher than my target?
**A:** 
1. System calculates ideal target = observed TDEE * 0.80 (e.g., for 20% cut)
2. If that's higher than current target, delta is clamped to +500 kcal
3. You'll gradually approach the ideal over multiple check-ins
4. This is correct behavior — means your observed TDEE is higher than the original estimate

---

## Future Enhancements (Brainstorm)

- [ ] Confidence interval display (range around observed TDEE)
- [ ] Weekly check-in history view (see all past check-ins + target adjustments)
- [ ] "Predicted weight after 4 weeks at this rate" display
- [ ] Allow user to manually override damping (with warning)
- [ ] Creatine/glycogen shift detection (flag at first log after diet switch)
- [ ] Menstrual cycle awareness (weight trend filter)
- [ ] Adaptive damping (smaller if trending toward goal, larger if away)
- [ ] "Skip this check-in" button if data is anomalous

---

**Last Updated:** 2026-04-30  
**Reviewed:** ✅ Vault Truth Sync pending (run `mempalace mine .` after commits)
