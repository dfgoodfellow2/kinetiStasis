# Diet Tracker v2: Quick Reference Card

## 🎯 The Five Immutable Rules of Weekly Check-In

### Rule 1: Observed > Profile
Observed TDEE (computed from YOUR data) always beats profile estimates.

### Rule 2: Max ±500 kcal/week
No single check-in moves targets more than 500 kcal in either direction.
```go
const MaxWeeklyCalorieSwing = 500
```

### Rule 3: Macros from Final Calories
Macros are ALWAYS recalculated from the final damped calorie number (post-floor).

### Rule 4: Maintenance Special Case
If goal == "maintenance", use observed TDEE directly (skip goal multiplier).

### Rule 5: Emergency Override ≤ Timing Only
If TDEE diverges >15% (and confidence ≠ "low"), unlock the 5-day wait gate.
**BUT: Damping always applies.**

---

## 📊 Check-In Flow (Copy-Paste for Docs)

```
User initiates
    ↓
Fetch last 14 days (nutrition + weight)
    ↓
Compute Observed TDEE (tiered: LinReg/RLS/EMA)
    ↓
Calculate Ideal = Observed ± Exercise
(Special case: maintenance → skip goal multiplier)
    ↓
Clamp delta to ±MaxWeeklyCalorieSwing
    ↓
Enforce 1200 kcal floor
    ↓
Recalculate macros from final calories
    ↓
Check: tdeeDiffPct > 15% && confidence ≠ "low"?
    ├─ YES: emergency override active (unlock timing gate)
    └─ NO: normal flow
    ↓
Display: old → new, delta, warnings
    ↓
User: Accept / Skip
    ↓
ACCEPT: Update targets table + snapshot to history
SKIP: No change
```

---

## 🔧 Code Locations (Go)

| What | Where | Lines |
|-----|-------|-------|
| Observed TDEE | `services/calculator.go` | 50–150 |
| Damping Logic | `services/calculator.go` | 506–541 |
| Emergency Override | `services/calculator.go` | 580–593 |
| Macro Distribution | `services/nutrition/macros.go` | Full file |
| API Handlers | `internal/handlers/calculations.go` | All `/v1/calc/*` |
| Target Snapshot | `internal/handlers/targets.go` | PUT `/v1/targets` |
| TUI Check-In Screen | `cmd/tui/ui/checkin.go` | 400+ lines |

---

## 📱 API Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/v1/dashboard` | Full check-in data (TDEE, macros, stats) |
| GET | `/v1/calc/tdee?days=N` | Observed TDEE for last N days |
| GET | `/v1/calc/macros` | Current macro targets |
| GET | `/v1/targets` | Current targets snapshot |
| PUT | `/v1/targets` | Save new targets + snapshot old |
| GET | `/v1/targets/history` | All past targets (with dates) |

---

## 🧮 Tiered TDEE Confidence

| Days | Method | Confidence |
|-----|--------|-----------|
| 1–3 | LinReg | LOW |
| 4–13 | LinReg / RLS | MEDIUM |
| 14–29 | RLS | HIGH |
| 30+ | EMA | HIGH |

---

## 🛡️ Safety Checklist

- [ ] Max weekly swing = 500 kcal ✅
- [ ] Minimum floor = 1200 kcal ✅
- [ ] Macros recalculated post-damping ✅
- [ ] Damping always active (even with override) ✅
- [ ] No magic numbers (all named constants) ✅
- [ ] Single source of truth for override ✅

---

## ⚠️ Emergency Override Trigger

```
IF (|observedTDEE - estimatedTDEE| / estimatedTDEE > 0.15)
   AND confidence ≠ "low"
THEN unlock 5-day timing gate
```

**Effect:** Allows immediate check-in instead of waiting 5 days.  
**Does NOT:** Bypass 500 kcal damping cap.

---

## 🐛 Troubleshooting

**Q: My target jumped +800 kcal**  
A: Check damping logic (should clamp to ±500). Verify `ComputeWeeklyTargetsWithExercise()`.

**Q: Macros don't add up**  
A: Macros are recalculated from final damped calories. Math: `P*4 + C*4 + F*9 ≈ calories`.

**Q: Why "Medium" confidence?**  
A: You have 7–13 days. RLS filter active. Need 14+ for "High".

**Q: Can I override the 500 kcal cap?**  
A: No. It's a named constant. Change it → rebuild. But it's designed for safety (metabolic shock prevention).

**Q: Maintenance goal not working**  
A: Check: `if profile.Goal == "maintenance" { idealTarget = observedTDEE + exercise }`

---

## 📈 Macro Algorithm Order (ComputeTargets)

1. **Protein first** (1.2–2.2 g/kg based on lifter/endurance status)
2. **Runner carbs** (if 15+ mi/week, enforce 5+ g/kg minimum)
3. **Fat** (height-based minimum: 30g + (h-150)*0.5 if h≥150cm)
4. **Carbs** (remainder after protein + fat)
5. **Fiber** (14g per 1000 kcal)

---

## 🎮 User Interaction (TUI)

**Dashboard → Check-In Screen:**
1. Display observed TDEE, confidence, old → new targets
2. Toggle "Eat Back Exercise" (invalidates cache if changed)
3. Show emergency override warning if triggered (15%+ TDEE divergence)
4. Accept / Skip

**On Accept:**
- Update `macro_targets` table
- Snapshot old targets to `target_history` table
- Return to dashboard

---

## 📚 Vault References

**CURRENT SPEC:**
- `projects/diet-program/adaptive-tdee-calorie-targets.md` ⭐

**Implementation Details:**
- `projects/diet-program/calculator.md` (algorithms)
- `projects/diet-program/research-notes.md` (MacroFactor methodology)
- `Projects/diet-project2/api-server.md` (endpoints)

**Archived (Historical):**
- `projects/diet-program/tracker-plan.md` (v1 design — TDEE section outdated)

---

## ✅ Latest Audits

**Date:** 2026-04-15  
**Auditor:** QA Walk-Through  
**Status:** Full Damping Sync ✅  
**Build:** Clean (zero errors/warnings)

---

**Last Updated:** 2026-04-30  
**Maintained by:** Librarian (Vault Truth Index)
