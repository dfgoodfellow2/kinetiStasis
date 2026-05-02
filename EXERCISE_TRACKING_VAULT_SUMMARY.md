# Exercise Tracking Documentation Summary
## From Obsidian Vault - Semantic Search Results

---

## 1. DOCUMENTED ARCHITECTURE & DESIGN DECISIONS

### A. Duration vs. Reps Classification System
**Location**: `/projects/diet-program/calculator.md` (Active, last updated 2026-03-27)

The system uses a **unified pipeline** that handles both duration-based and rep-based exercises:

#### Duration-Based Exercise Flag
- **Critical Rule**: When `Reps == 1` AND `TUTSeconds > 0` (internal representation), the system **must** apply duration-based rules
- **This applies regardless of exercise name** — the `Reps == 1` flag takes priority
- Examples: sled pulls, prowler pushes, conditioning work

#### Volume Calculation Differs by Type
- **Strength-based**: `Volume = sets × reps`
- **Duration-based**: `Volume = (Load × TUT) / 10`
  - TUT = time under tension in seconds
  - Load = weight in appropriate units

### B. NDS (Normalized Difficulty Score) Formula
**Status**: ACTIVE TRUTH (most recent updates: 2026-04-09)

```
NDS = (Volume × PatternFactor × UnilateralFactor) × I_True²
```

#### For Duration-Based Exercises (Reps == 1 + TUTSeconds > 0):
1. **Always use Time-Based Volume formula**: `Volume = (Load × TUT) / 10`
2. **Always apply `multiJointFactor` of 1.5**
   - Duration-based = treated as multi-joint effort for fatigue
   - Prevents pattern-name conflicts (e.g., "sled push hinge" reverting to strength calc)
3. **Purpose**: Accurate fatigue tracking for mixed strength/conditioning workouts

#### Pattern Factors
- Bilateral (squat, deadlift, push): 1.0
- Asymmetric (split squat, single-arm rows): 1.3
- Single-limb (single-leg deadlift, pistol squat): 1.8

#### Unilateral Factors
- Bilateral movement: 1.0
- One-sided holds (plank, carry): 1.0
- Alternating unilateral (alternating rows): 1.1
- Total unilateral work: 1.3

### C. YAML Format for Exercise Definition
**Location**: `/topics/health/fitness/movement-practice/current-exercises-six-day-pattern.md`
**Status**: ACTIVE SPECIFICATION

```yaml
exercises:
  - name: "Exercise Name"              # Required
    sets: 3                            # Required
    reps: 8                            # Strength only (mutually exclusive with duration)
    duration: "45 sec"                 # Conditioning only (mutually exclusive with reps)
    tempo: "2-0-2-0"                   # Optional: eccentric-pause-concentric-pause
    load: "BW|50 lbs|35+35 lbs"       # Optional
    rpe: 8                             # Optional: 1-10 scale
```

#### Key Mutually Exclusive Fields
- `reps`: For strength exercises (rep count per set)
- `duration`: For conditioning/isometric exercises (time per set)
  - Accepted formats: "35 sec", "2:30 min", "10 sec"

### D. Rep-to-Duration Conversion (for conditioning)
If only `duration` is provided:
1. Estimate rep count from **METs × duration × body weight**
2. Apply cardiovascular intensity (via HR or RPE)
3. Calculate volume as: `Volume = Est. Reps × Sets`
4. **Trigger**: If `Reps == 1` and `TUTSeconds > 0`, apply duration-based NDS rules

---

## 2. CURRENT SYSTEM TRUTH (Active Implementation)

### A. Workout Data Model
**Location**: `/projects/diet-program/Workout-Logging 1.md`
**Status**: ACTIVE (2026-04-02, updated continuously)

#### WorkoutEntry Fields
| Field | Type | Required | Notes |
|-------|------|----------|-------|
| Date | string | auto | YYYY-MM-DD |
| Type | enum | yes | strength, cardio, hiit, mobility, sport, conditioning |
| Duration | int | yes | minutes (session-level) |
| Intensity | string | no | light, moderate, vigorous |
| RPE | int | no | 1-10 |
| AvgHR | int | no | average heart rate |
| MaxHR | int | no | max heart rate |
| Calories | int | no | calculated or AI-estimated |
| MET | float | no | metabolic equivalent |
| Notes | string | no | raw description |

### B. Exercise-Level Metadata (Session v2)
**Location**: `/topics/health/fitness/movement-practice/current-exercises-six-day-pattern.md` (lines 239-306)

#### Core Set Specification Fields
| Field | Type | Description |
|-------|------|-------------|
| sets | int | Number of sets |
| reps | int | Reps per set (strength only) |
| duration | string | Time per set (conditioning only) |
| tempo | string | Eccentric-Pause-Concentric-Pause in seconds |
| tutSeconds | float | Time under tension per set in seconds |

#### Load & Intensity Fields
| Field | Type | Description |
|-------|------|-------------|
| load | string | "BW", "50 lbs", "35+35 lbs" |
| loadKg | float | Explicit load in kg |
| loadLbs | float | Explicit load in lbs |
| rpe | int | Per-exercise RPE (1-10) |
| intensityRelMax | float | Intensity as % of 1RM (0.0-1.0) |
| metValue | float | Explicit MET override |

### C. Pattern & Bias Classification
**Recent Update**: 2026-04-09 (journal commit)

Pattern inference from exercise name:
- `sled` → hinge pattern
- `deadlift` → hinge pattern  
- `HIIT`, `cardio` → conditioning pattern

Bias classification:
- `bilateral` - both sides equally
- `unilateral` - single-sided work
- Defaults inferred from movement type if not specified

### D. Surface Multiplier System
**Status**: IMPLEMENTED (2026-04-09)

Applied to effective load calculations for surface-dependent exercises (sled work):

| Surface | Multiplier | Use For |
|---------|-----------|---------|
| pavement/concrete/road | 0.7× | Hard flat surface |
| wet_grass/wet grass/rain | 0.9× | Damp outdoor |
| grass/standard_grass | 1.0× | Normal grass (neutral) |
| sticky_grass/thick_grass | 1.2× | High-resistance |
| gym/home/other | 0.0× | Treated as neutral |

**Export Format**: Shows as "60 lbs → 90 lbs (1.5×)" to visualize effective load

---

## 3. HISTORICAL DESIGN DECISIONS

### A. From App-Ideas (Feature Brainstorm)
**Location**: `/personal/systems/ideas.md` (matched as "app-ideas.md")

#### Original Feature Requirements
- Limit of **no more than 10% increase per week** based on volume (sets/reps/time)
- **Auto-logging** with easy entry
- **Auto-progression/regression** logic
- **Large exercise inventory** with progressions/regressions
- **Track rests and effort** (HR, RPE) to adjust workouts

#### Smart Logging Concept
- Quick 30-second daily assessment
- Auto-calculate total weekly volume
- Track 10% rule compliance
- Flag excessive fatigue patterns

#### Auto-Progression Logic
- Movement quality analysis (camera-based)
- Heart rate zone adherence
- Recovery trend monitoring
- Exercise substitution based on performance

### B. Exercise Routes & Decision Trees
**Location**: `/topics/health/fitness/movement-practice/exercise-routes.md`
**Status**: COMPREHENSIVE SPECIFICATION (794 lines)

#### Five Major Decision Points
1. **Squat Pattern** — bilateral loading, pistol squat, jump/power, ATG/mobility
2. **Hinge Pattern** — ballistic, deadlift, Nordic/hamstring, mobility
3. **Pull Pattern** — one-arm pull-up, muscle-up, volume, gymnastics skills
4. **Cardio Training** — running build-up, HIIT/sprint, Zone 2, long duration
5. **Medical/Limitation-Based** — ankle mobility, knee pain, hip tightness, lower back, shoulder issues

---

## 4. SUMMARY DISPLAY & LOGGING LOGIC

### A. Summary Calculation (Exercise-Level)
**Status**: IMPLEMENTED (Last confirmed: 2026-04-09)

For each exercise in a workout:
1. **Volume**: Calculated based on type (reps-based vs. duration-based)
2. **NDS**: Applied using pattern factor, unilateral factor, and intensity squared
3. **MWV** (Mechanical Work Volume): Load × reps (for strength)
4. **Session Density**: Total NDS / session duration

### B. Export Format (CNS Summary & Pattern Distribution)
**Status**: ACTIVE (2026-04-09 commit)

**Exported metrics**:
- Pattern % distribution (squat %, hinge %, push %, pull %, conditioning %)
- Intensity distribution (by RPE zones)
- RPE average + trend
- Training frequency (workouts/week)
- Surface multiplier (for sled work)
- Effective load calculation
- Unilateral/bilateral totals per pattern

### C. AI Parsing vs. Manual YAML
**Location**: `/projects/diet-program/Workout-Logging 1.md`
**Status**: BOTH ACTIVE (2026-04-08)

#### AI Parser Path
- Receives: workout notes, body weight, age, sex, HR data
- Returns: exercise type, duration, intensity, RPE, MET, calories, summary
- Uses **Gemini 2.5 Flash Lite** (server-proxied)

#### YAML Parser Path
- Parses strength and conditioning formats
- Extracts pattern and bilateral/unilateral from focus field
- Surface coefficient support for sled work
- Full input form with Tab navigation

**Key Truth**: Both paths produce **consistent data structure** (as of 2026-04-08)

---

## 5. CRITICAL IMPLEMENTATION RULES

### A. Duration vs. Reps: The Reps == 1 Flag
**CRITICAL** (from calculator.md, lines 139-146):

**Rule**: For any exercise where `TUTSeconds > 0` and `Reps == 1`:
- **Always use Time-Based Volume formula**: `Volume = (Load × TUT) / 10`
- **Always apply `multiJointFactor` of 1.5**
- **This applies REGARDLESS of the exercise name** — priority: Reps == 1 flag over pattern name
- **Purpose**: Prevent pattern-name conflicts (e.g., "sled push hinge" reverting to strength calc)

### B. Intensity Calculation (True Intensity I_True)
**Status**: RECALCULATED DURING SAVE (as of 2026-03-27)

```
I_True = max(
  Weight / 1RM,           # Load ratio (0.5-1.0 typical)
  RPE / 10,               # Perceived effort (0.6-1.0)
  AvgHR / MaxHR           # Cardiovascular stress
)
```

**Important**: `IntensityRelMax` is **recalculated during save()** to capture any post-logging RPE/HR updates. This ensures fatigue tracking reflects latest effort data.

### C. Exercise Name Pattern Inference
**Status**: IMPLEMENTED (2026-04-09)

Pattern-name mappings with fallback inference:
- User-provided `pattern` field takes priority
- Else: infer from `exercises.category`
- Else: infer from `exercises.name` substring matching
- Fallback: "conditioning" if name contains sled/cardio/HIIT

---

## 6. KEY RESEARCH FINDINGS (Applied to Design)

From calculator.md (lines 80-85):
- **Running burns ~0.71 kcal/lb/mile** (ACSM, regardless of pace)
- **1RM Estimation**: Brzycki (r < 10) vs. Epley (r ≥ 10) formulas
- **Volume Normalization**: Unified pipeline ensures consistent fatigue tracking
- **MET-based conditioning**: Time × intensity, not traditional reps

---

## 7. VAULT FILE LOCATIONS (VERIFIED PATHS)

### Primary Documentation
1. **Calculator & NDS Rules**
   - Path: `/home/dfgoodfellow2/Obsidian/Brain/projects/diet-program/calculator.md`
   - Status: active | Type: project-note | Updated: 2026-03-27

2. **Workout YAML Format & Templates**
   - Path: `/home/dfgoodfellow2/Obsidian/Brain/topics/health/fitness/movement-practice/current-exercises-six-day-pattern.md`
   - Status: active | Updated: continuously

3. **Exercise Routes & Decision Trees**
   - Path: `/home/dfgoodfellow2/Obsidian/Brain/topics/health/fitness/movement-practice/exercise-routes.md`
   - Status: active | 794 lines | Comprehensive limitation-based routing

4. **Workout-Logging Feature Spec**
   - Path: `/home/dfgoodfellow2/Obsidian/Brain/projects/diet-program/Workout-Logging 1.md`
   - Status: active | Type: research | Updated: 2026-04-02

5. **Project Index & Architecture**
   - Path: `/home/dfgoodfellow2/Obsidian/Brain/projects/diet-program/INDEX.md`
   - Status: active | Type: research | Updated: 2026-04-28

### Recent Journal Updates
- **2026-04-09**: NDS calculation fixes, surface multiplier, unified YAML format
- **2026-04-08**: YAML and AI parser enhancement, pattern extraction, consistent data structures

---

## 8. OPEN QUESTIONS / AMBIGUITIES FLAGGED

### A. Duration Field Semantics
- **Question**: When both `duration` (session-level, minutes) and `exercises[].duration` (exercise-level, e.g., "45 sec") are present, which takes precedence?
- **Current Truth**: Exercise-level duration is used in volume calc; session-level is metadata/display

### B. TUT Calculation from Tempo
**Rule Implemented** (calculator.md line 161):
```
Duration = Reps × Tempo Sum
Example: 8 reps × "2-0-2-0" (sum=4) = 32 seconds TUT
```
- **Validation**: Is this formula applied automatically during logging, or only in post-hoc analysis?

### C. Surface Multiplier Application
**Ambiguity**: Does surface multiplier apply to:
1. Load only (affecting NDS volume calculation)?
2. MET/calorie estimation (making work "feel" harder)?
3. Both?

**Current Status** (2026-04-09): Applied to effective load display in export; NDS calculation shows "60 → 90 lbs (1.5×)"

---

## 9. RECOMMENDATIONS FOR CONSISTENCY

### Immediate Verification Needed
1. **Codebase check**: Confirm Reps == 1 flag is implemented in volume calculation
2. **TUT auto-calc**: Verify if tempo string is automatically converted to TUTSeconds during save
3. **Surface multiplier scope**: Confirm if applied to NDS volume or display-only

### Documentation Sync
- **calendar.md** is the "source of truth" for NDS rules
- **current-exercises-six-day-pattern.md** is the "source of truth" for YAML format
- **Workout-Logging 1.md** describes the API/UI layer (may lag behind internal logic)
- **Recent journals** (2026-04-09, 2026-04-08) capture the latest implementation state

---

**Generated**: 2026-05-02  
**Vault BRAIN_ROOT**: `/home/dfgoodfellow2/Obsidian/Brain/`  
**Last Semantic Sync**: MemPalace semantic index
╔════════════════════════════════════════════════════════════════════════════════╗
║     EXERCISE TRACKING: DURATION VS. REPS CLASSIFICATION FLOWCHART              ║
║                  (From Vault Truth - 2026-04-09 ACTIVE)                        ║
╚════════════════════════════════════════════════════════════════════════════════╝

                         ┌─────────────────────────────┐
                         │   EXERCISE LOGGED TO SYSTEM │
                         │  (YAML, AI, or Manual Entry)│
                         └──────────────┬──────────────┘
                                        │
                                        ▼
                         ┌─────────────────────────────┐
                         │   EXTRACT EXERCISE FIELDS   │
                         │  - name                     │
                         │  - sets                     │
                         │  - reps (optional)          │
                         │  - duration (optional)      │
                         │  - tempo (optional)         │
                         │  - load (optional)          │
                         │  - rpe (optional)           │
                         └──────────────┬──────────────┘
                                        │
                                        ▼
              ┌─────────────────────────────────────────────────┐
              │   CRITICAL DECISION POINT:                      │
              │   CHECK INTERNAL REPRESENTATION                 │
              │   (Reps == 1) AND (TUTSeconds > 0)?            │
              └──────────────┬────────────────────┬─────────────┘
                             │                    │
                    YES ─────┘                    └───── NO
                    │                                   │
                    ▼                                   ▼
     ┌──────────────────────────────┐   ┌──────────────────────────────┐
     │  DURATION-BASED EXERCISE      │   │  STRENGTH/REP-BASED EXERCISE │
     │  ════════════════════════     │   │  ═════════════════════════  │
     │                               │   │                              │
     │ Characteristics:              │   │ Characteristics:             │
     │ - Sled pulls                  │   │ - Squats, deadlifts         │
     │ - Prowler pushes              │   │ - Rows, presses             │
     │ - Conditioning work           │   │ - Isometric holds (planks)  │
     │ - HIIT circuits               │   │ - Traditional strength moves │
     │                               │   │                              │
     │ Volume Formula:               │   │ Volume Formula:              │
     │ ───────────────────────────── │   │ ─────────────────────────── │
     │ Volume = (Load × TUT) / 10    │   │ Volume = Sets × Reps        │
     │                               │   │                              │
     │ where TUT = seconds           │   │ NDS = Volume ×              │
     │                               │   │       PatternFactor ×       │
     │ NDS = Volume ×                │   │       UnilateralFactor ×    │
     │       1.5 (multiJoint) ×      │   │       I_True²               │
     │       1.0 (pattern) ×         │   │                              │
     │       I_True²                 │   │ Pattern Factors:             │
     │                               │   │ - Bilateral: 1.0             │
     │ NOTE: Always uses 1.5 factor  │   │ - Asymmetric: 1.3            │
     │       regardless of pattern   │   │ - Single-limb: 1.8           │
     │       name to prevent         │   │                              │
     │       "sled push hinge" bug   │   │ TUT auto-calc (optional):    │
     │                               │   │ TUT = Reps × Tempo Sum      │
     └───────────────┬───────────────┘   │ e.g., 8 × 4 = 32 sec       │
                     │                    │                              │
                     │                    └──────────────┬───────────────┘
                     │                                   │
                     └───────────────────┬───────────────┘
                                         │
                                         ▼
                         ┌───────────────────────────────┐
                         │ EXTRACT PATTERN & BIAS        │
                         │ Priority Order:               │
                         │ 1. User-provided pattern      │
                         │ 2. Infer from category        │
                         │ 3. Infer from name substring  │
                         │ 4. Fallback: conditioning     │
                         │                               │
                         │ Pattern Mappings:             │
                         │ - "sled" → hinge              │
                         │ - "deadlift" → hinge          │
                         │ - "HIIT/cardio" → conditioning│
                         └───────────────┬───────────────┘
                                         │
                                         ▼
                         ┌───────────────────────────────┐
                         │ CALCULATE INTENSITY (I_True)  │
                         │                               │
                         │ I_True = max(                 │
                         │   Weight / 1RM,               │
                         │   RPE / 10,                   │
                         │   AvgHR / MaxHR               │
                         │ )                             │
                         │                               │
                         │ NOTE: Recalculated at save()  │
                         │ to capture RPE/HR updates     │
                         └───────────────┬───────────────┘
                                         │
                                         ▼
                         ┌───────────────────────────────┐
                         │ APPLY SURFACE MULTIPLIER      │
                         │ (for sled/conditioning work)  │
                         │                               │
                         │ Pavement/Concrete: 0.7×       │
                         │ Wet Grass: 0.9×               │
                         │ Grass (normal): 1.0×          │
                         │ Sticky Grass: 1.2×            │
                         │ Gym/Home: 0.0× (neutral)      │
                         │                               │
                         │ Display as:                   │
                         │ "60 lbs → 90 lbs (1.5×)"      │
                         └───────────────┬───────────────┘
                                         │
                                         ▼
                         ┌───────────────────────────────┐
                         │ COMPUTE FINAL METRICS         │
                         │                               │
                         │ - NDS (Normalized Difficulty) │
                         │ - MWV (Mechanical Work Volume)│
                         │ - Session Density             │
                         │ - Calories (if applicable)    │
                         │                               │
                         │ Export Includes:              │
                         │ - Pattern % distribution      │
                         │ - Intensity distribution      │
                         │ - RPE avg + trend             │
                         │ - Training frequency          │
                         │ - Unilateral/bilateral totals │
                         └───────────────┬───────────────┘
                                         │
                                         ▼
                         ┌───────────────────────────────┐
                         │      STORE & EXPORT           │
                         │   (SQLite + Markdown/CSV)     │
                         └───────────────────────────────┘

╔════════════════════════════════════════════════════════════════════════════════╗
║                      KEY TRUTH TABLE: WHEN TO USE WHAT                         ║
╠════════════════════════════════════════════════════════════════════════════════╣
║                                                                                ║
║  User Input Format          Internal Flag              Volume Calculation     ║
║  ─────────────────────      ─────────────────          ─────────────────────  ║
║                                                                                ║
║  reps: 8                    Reps = 8                   Volume = Sets × 8      ║
║  (no duration)              TUTSeconds = (optional)    (strength path)        ║
║                                                                                ║
║  duration: "2:30 min"       Reps = 1 (FLAG)            Volume = (Load × TUT)  ║
║  (no reps)                  TUTSeconds = 150           / 10 (duration path)   ║
║                                                        Apply 1.5 factor       ║
║                                                                                ║
║  reps: 8 + tempo: "2-0-2-0" Reps = 8                  Volume = Sets × 8      ║
║  (auto-calc TUT)            TUTSeconds = 32            (strength path)        ║
║                             (auto from tempo)         TUT for display         ║
║                                                                                ║
║  *** CRITICAL RULE ***                                                        ║
║  If Reps == 1 AND TUTSeconds > 0:                                             ║
║  → ALWAYS use duration formula                                                ║
║  → ALWAYS apply 1.5 multiJoint factor                                         ║
║  → REGARDLESS of exercise name (e.g., "sled push hinge")                      ║
║                                                                                ║
╚════════════════════════════════════════════════════════════════════════════════╝

