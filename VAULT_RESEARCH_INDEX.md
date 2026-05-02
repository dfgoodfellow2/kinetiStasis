# Obsidian Vault Research Report: Exercise Tracking System
**Generated**: 2026-05-02  
**Librarian**: Semantic Research Agent  
**Status**: Complete Documentation Audit ✓

---

## Executive Summary

Comprehensive audit of the Obsidian vault for exercise tracking documentation found **5 primary authoritative sources** plus 3 secondary documents. The system implements a **unified duration/reps pipeline** with explicit design decisions documented back to the original feature concept.

### Key Findings:
1. **Duration vs. Reps Classification**: Governed by `Reps == 1` AND `TUTSeconds > 0` flag (CRITICAL RULE)
2. **NDS Calculation**: Active since 2026-03-27, with recent fixes for surface multiplier (2026-04-09)
3. **YAML Format**: Standardized specification with mutually-exclusive `reps` and `duration` fields
4. **Pattern Inference**: Hierarchical priority from user-provided → inferred from name
5. **Surface Awareness**: Multiplier system (0.7× to 1.2×) applied to effective load display

---

## Document Map

### PRIMARY SOURCES (★★★★★ Authoritative)

#### 1. **calculator.md** — NDS & Duration Rules
- **Location**: `/projects/diet-program/calculator.md`
- **Status**: Active (updated 2026-03-27)
- **Authority**: Authoritative for NDS formula and duration logic
- **Key Content**:
  - Lines 119-170: Complete NDS formula with all factors
  - **Lines 139-146**: CRITICAL RULE for `Reps == 1` flag
  - Lines 148-169: Unified pipeline for duration/reps interchangeability
- **Related Rules**:
  - Intensity recalculation at save() (lines 113-117)
  - TUT auto-calculation from tempo (lines 158-162)
  - 1RM formulas (Brzycki/Epley, lines 89-100)

#### 2. **current-exercises-six-day-pattern.md** — YAML Specification
- **Location**: `/topics/health/fitness/movement-practice/current-exercises-six-day-pattern.md`
- **Status**: Active (continuously updated)
- **Authority**: Authoritative for YAML format and field definitions
- **Key Content**:
  - Lines 215-318: Complete YAML template documentation
  - **Lines 246-247**: Mutually exclusive `reps` vs `duration` fields
  - Lines 239-306: Comprehensive field reference table
  - Lines 280-307: Surface multiplier coefficient table
- **Field Reference**:
  - `duration`: Conditioning format ("35 sec", "2:30 min", "10 sec")
  - `reps`: Strength format (integer reps per set)
  - `tempo`: "2-0-2-0" (eccentric-pause-concentric-pause)
  - `load`: "BW|50 lbs|35+35 lbs"

#### 3. **exercise-routes.md** — Decision Trees & Routing
- **Location**: `/topics/health/fitness/movement-practice/exercise-routes.md`
- **Status**: Active (maintained)
- **Authority**: Comprehensive exercise progression and limitation-based routing
- **Size**: 794 lines with JavaScript logic
- **Key Content**:
  - Lines 10-122: Five major decision tree diagrams
  - Lines 337-508: Limitation-based routing (ankle, knee, hip, back, shoulder)
  - Lines 589-664: `LimitationRouter` JavaScript implementation

#### 4. **Workout-Logging 1.md** — Feature Documentation
- **Location**: `/projects/diet-program/Workout-Logging 1.md`
- **Status**: Active (updated 2026-04-02)
- **Authority**: Good but may lag 1-2 weeks behind code
- **Key Content**:
  - Lines 20-33: WorkoutEntry data model
  - Lines 35-110: MET calculation and HR-based override
  - Lines 81-201: AI parsing and exercise lookup table (45+ exercises)
- **Latest Updates**: AI parser enhancement (2026-04-08)

#### 5. **INDEX.md** — Project Architecture
- **Location**: `/projects/diet-program/INDEX.md`
- **Status**: Active (updated 2026-04-28)
- **Authority**: Current architecture and code quality status
- **Key Content**:
  - Lines 39-81: Go API + SQLite architecture v2
  - Lines 91-111: Backend features list
  - Lines 136-150: Code quality fixes (FIX-001 through FIX-013)

---

## Critical Implementation Rules

### Rule 1: Duration vs. Reps Classification ⚠️ CRITICAL
**Source**: calculator.md lines 139-146

```
IF (Reps == 1) AND (TUTSeconds > 0):
  THEN:
    - ALWAYS use: Volume = (Load × TUT) / 10
    - ALWAYS apply: multiJointFactor = 1.5
    - NEVER revert to strength formula based on exercise name
    - PURPOSE: Prevent "sled push hinge" bug
```

**Impact**: This rule takes precedence over exercise name pattern matching

### Rule 2: Intensity Recalculation at Save
**Source**: calculator.md lines 113-117

The `IntensityRelMax` field is **recalculated during save()** to capture post-logging updates:
- Takes `max(Weight/1RM, RPE/10, AvgHR/MaxHR)`
- Ensures fatigue tracking reflects latest effort data
- Applied for both duration and rep-based exercises

### Rule 3: Pattern Inference Hierarchy
**Source**: 2026-04-09 journal; confirmed in calculator.md

Priority order for pattern determination:
1. User-provided `pattern` field (explicit)
2. Exercise `category` field (if present)
3. Name substring matching (inferred):
   - "sled" → hinge
   - "deadlift" → hinge
   - "HIIT", "cardio" → conditioning
4. Fallback: "conditioning"

### Rule 4: Volume Calculation Depends on Type

**Strength-based (Reps ≠ 1)**:
```
Volume = Sets × Reps
```

**Duration-based (Reps == 1 + TUTSeconds > 0)**:
```
Volume = (Load × TUT) / 10
where TUT = time under tension in seconds
```

### Rule 5: NDS Complete Formula
**Source**: calculator.md lines 119-137

```
NDS = (Volume × PatternFactor × UnilateralFactor) × I_True²
```

**Pattern Factors**:
- Bilateral (squat, deadlift, push): 1.0
- Asymmetric (split squat, single-arm rows): 1.3
- Single-limb (pistol squat, single-leg deadlift): 1.8

**Unilateral Factors**:
- Bilateral movement: 1.0
- One-sided holds (plank, carry): 1.0
- Alternating unilateral (alternating rows): 1.1
- Total unilateral work: 1.3

---

## Recent Implementation Updates

### 2026-04-09: NDS & Duration Fixes
- ✅ Fixed double surface multiplier in volume calc
- ✅ Added surface shorthand (pavement/concrete/grass/sticky_grass)
- ✅ Added MET override for conditioning workouts
- ✅ Unified YAML format for strength and conditioning
- ✅ Fixed `extractPatternAndBias`: sled→hinge, HIIT→conditioning
- ✅ NDS calculation fixes for conditioning workouts
- ✅ Surface multiplier export: "60 lbs → 90 lbs (1.5×)"

### 2026-04-08: YAML & AI Parser Alignment
- ✅ YAML workout import from menu
- ✅ AI parser enhancement to match YAML capabilities
- ✅ Pattern and BilateralUnilateral extraction in both paths
- ✅ Consistent data structure output (AI and YAML)

---

## Surface Multiplier System

**Implemented**: 2026-04-09  
**Applied to**: Effective load display in exports; NDS calculation awareness

| Surface Value | Multiplier | Use Case |
|---|---|---|
| pavement, concrete, road | 0.7× | Hard flat surface (reduced friction) |
| wet_grass, wet grass, rain, dew | 0.9× | Damp outdoor surface |
| grass, standard_grass | 1.0× | Normal grass (neutral) |
| sticky_grass, thick_grass, overgrown | 1.2× | High-resistance surface |
| gym, home, etc. | 0.0× | Treated as neutral (no effect) |

**Export Format**: "60 lbs → 90 lbs (1.5×)" shows effective load adjustment

---

## YAML Format Reference

### Session-Level Fields
```yaml
type: "strength"              # strength|hiit|cardio|zone2|conditioning|mobility|sport|yoga
name: "Workout Name"          # Optional: descriptive name
focus: ["Hinge(B)", "Push(U)"] # Optional: movement patterns with bias
duration: 45                  # Optional: session minutes (integer)
rpe: 7                        # Optional: session-level RPE (1-10)
avgHr: 145                    # Optional: average heart rate
maxHr: 170                    # Optional: max heart rate
surface: "concrete"           # Optional: surface condition
```

### Exercise-Level Fields (Mutually Exclusive)
```yaml
exercises:
  - name: "Exercise Name"     # Required
    sets: 3                   # Required
    
    # EITHER reps (strength) OR duration (conditioning):
    reps: 8                   # Strength exercises (mutually exclusive)
    duration: "45 sec"        # Conditioning exercises (mutually exclusive)
                              # Accepted: "35 sec", "2:30 min", "10 sec"
    
    # Optional timing/intensity
    tempo: "2-0-2-0"          # Eccentric-Pause-Concentric-Pause (seconds)
    tutSeconds: 120.0         # Time under tension (auto-calc from tempo)
    rpe: 8                    # Per-exercise RPE (1-10)
    
    # Load specification
    load: "BW"                # "BW", "50 lbs", "35+35 lbs"
    loadKg: 22.7              # Explicit kg
    loadLbs: 50.0             # Explicit lbs
    
    # Pattern & intensity
    pattern: "hinge"          # Optional: squat|hinge|push|pull|conditioning
    metValue: 11.0            # Optional: explicit MET override
```

---

## Design Decision Lineage

### Historical Origin
**App-Ideas (original feature concept)**:
- Limit 10% weekly volume increase (sets/reps/time)
- Auto-progression/regression logic
- Exercise substitution based on performance
- Track rest/effort (HR, RPE) to adjust workouts

### Phase 1: Core Development
**Calculator (2026-03-27)**:
- Introduced 1RM estimation (Brzycki/Epley formulas)
- Introduced Intensity calculation: `max(Weight/1RM, RPE/10, HR/MaxHR)`
- Introduced NDS formula with pattern and unilateral factors

### Phase 2: Duration Integration
**2026-04 Enhancement**:
- Unified duration/reps pipeline
- `Reps == 1` flag for duration-based detection
- Time-based volume: `(Load × TUT) / 10`
- Surface multiplier system
- Pattern inference hierarchy

### Current State (2026-04-09)
- Bidirectional reps ↔ duration conversion
- Pattern inference with fallback chain
- Surface-aware load multipliers
- Dual input paths (AI + YAML) with consistent output
- Export-ready metrics (NDS, MWV, Session Density, Pattern %)

---

## Open Questions & Ambiguities

### Q1: Duration Field Semantics
**Ambiguity**: Session-level `duration` (minutes) vs. exercise-level `duration` (e.g., "45 sec")  
**Current Truth**: Exercise-level duration used in volume; session-level is metadata/display  
**Verification Needed**: Confirm priority order in code

### Q2: TUT Auto-Calc Trigger
**Ambiguity**: When is tempo → TUT conversion automatic?  
**Source**: calculator.md line 161 describes formula  
**Verification Needed**: Applied during save() or only post-hoc analysis?

### Q3: Surface Multiplier Scope
**Ambiguity**: Does multiplier affect:
- (A) Load only (display + NDS calc)?
- (B) MET/calorie estimation?
- (C) Both?

**Current Truth**: Affects display + NDS; unclear if MET-aware  
**Verification Needed**: Codebase check in MET calculation

---

## Recommended Verification Steps

### Priority 1: Confirm Reps == 1 Implementation
- **Check**: `services/calculator.go` for volume formula
- **Confirm**: `(Load × TUT) / 10` applied when `Reps == 1`
- **Source**: calculator.md lines 139-146

### Priority 2: Validate TUT Auto-Calc Scope
- **Check**: ExerciseSet model tempo → TUT conversion
- **Confirm**: TUTSeconds field populated automatically
- **Source**: calculator.md lines 158-162

### Priority 3: Clarify Surface Multiplier Path
- **Check**: Volume calc in NDS computation
- **Confirm**: Surface multiplier applied once (not twice)
- **Source**: 2026-04-09 journal fix

### Priority 4: Sync Docs vs. Code
- **Check**: Export format in code vs. Workout-Logging 1.md
- **Confirm**: Pattern %, intensity distribution, CNS summary match
- **Source**: Workout-Logging 1.md may lag behind implementation

---

## Documentation Trust Levels

| Document | Trust | Notes |
|----------|-------|-------|
| calculator.md | ★★★★★ | Authoritative for NDS & duration rules |
| current-exercises-six-day-pattern.md | ★★★★★ | Authoritative for YAML format |
| exercise-routes.md | ★★★★☆ | Comprehensive; actively maintained |
| Workout-Logging 1.md | ★★★★☆ | Good but may lag 1-2 weeks behind code |
| Recent journals (2026-04-09, 2026-04-08) | ★★★☆☆ | Best for "what changed" in code; captures commits |
| INDEX.md | ★★★☆☆ | High-level overview; last updated 2026-04-28 |

---

## Vault Metadata

- **Total Files Scanned**: 562
- **Files Matching Exercise Tracking**: 5 primary + 3 secondary
- **Semantic Matches** (MemPalace): 4 queries with high relevance
- **Journal Entries Reviewed**: 3 (2026-04-09, 2026-04-08, 2026-04-02)
- **Documentation Completeness**: Excellent (multiple views: design, spec, implementation)
- **Last Known Update**: 2026-04-28 (INDEX.md status flag)
- **Latest Implementation News**: 2026-04-09 (YAML format unification, surface multiplier fix)

---

## Quick Reference

### When to Use Each Document

**Designing new features?** → `calculator.md` (NDS rules) + `exercise-routes.md` (decision trees)

**Implementing YAML parsing?** → `current-exercises-six-day-pattern.md` (YAML template, lines 215-318)

**Understanding MET/calorie logic?** → `Workout-Logging 1.md` (lines 35-110)

**Checking recent implementation changes?** → 2026-04-09 journal entry

**Understanding system architecture?** → `INDEX.md` (lines 39-81)

---

## Related Documents in Codebase

This research complements:
- `/projects/diet-program/go-rewrite-changelog.md` — Full commit history
- `/projects/diet-program/FIXES.md` — Code quality improvements
- `/projects/diet-program/met-deep-research.md` — Deep MET physiology research

---

**Report Generated**: 2026-05-02  
**Vault BRAIN_ROOT**: `/home/dfgoodfellow2/Obsidian/Brain/`  
**Research Method**: MemPalace semantic search + verified vault traversal  
**Status**: Complete ✓

See `EXERCISE_TRACKING_VAULT_SUMMARY.md` for detailed content and decision tree flowchart.
