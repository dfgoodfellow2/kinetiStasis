# Diet Tracker v2: Design Documentation Index

**Generated:** 2026-04-30  
**Source:** Obsidian Vault Search & Analysis  
**Status:** ✅ Complete & Ready for Development

---

## 📚 Documentation Files (This Project)

### 1. QUICK_REFERENCE.md ⚡ START HERE
**Purpose:** Quick-lookup card for developers  
**Read Time:** 5-10 minutes  
**Size:** 5.2 KB

**Contents:**
- The 5 immutable rules (copy-paste format)
- Check-in flow diagram
- Code locations quick table
- API endpoints reference
- Safety checklist
- Troubleshooting Q&A

**Use Cases:**
- Quick reminder of core rules
- Finding where things are in the code
- Answering "can I do X?" questions

---

### 2. WEEKLY_CHECKIN_DESIGN.md 📖 COMPREHENSIVE GUIDE
**Purpose:** Complete reference guide with all design details  
**Read Time:** 30-45 minutes  
**Size:** 17 KB

**Contents:**
- Architecture overview
- 5 critical rules with code examples
- Tiered TDEE computation (detailed)
- Damping filter logic (line-by-line)
- Safety & stability checklist
- Key file locations with line numbers
- Example scenario walkthrough
- Full troubleshooting guide
- Future enhancement brainstorm

**Use Cases:**
- Understanding the full weekly check-in flow
- Learning why decisions were made (design rationale)
- Finding specific implementation details
- Planning enhancements

---

### 3. DESIGN_INDEX.md (This File)
**Purpose:** Navigation guide to all design documentation  
**Read Time:** 5 minutes  

---

## 🔗 Related Vault Documents

### Source of Truth (Current Implementation)

**adaptive-tdee-calorie-targets.md**
- **Location:** Vault: `projects/diet-program/adaptive-tdee-calorie-targets.md`
- **Status:** CURRENT SPEC (audited 2026-04-15)
- **Size:** 309 lines
- **Contains:**
  - All 5 rules verified against code
  - Damping logic implementation
  - Priority hierarchy
  - Safety checklist (all ✅)
  - Known limitations (LOW priority)

**Why Read:** This is the single source of truth. Use when you need to verify that the implementation matches the design.

---

### Implementation Details (Algorithms)

**calculator.md**
- **Location:** Vault: `projects/diet-program/calculator.md`
- **Contains:**
  - TDEE calculation algorithms
  - NDS (Normalized Difficulty Score) for workouts
  - Macro allocation order of operations
  - 1RM estimation formulas
  - Phase 1 & Phase 2 algorithm specs

**Why Read:** When you need to understand HOW the calculations work.

---

### API Reference

**api-server.md**
- **Location:** Vault: `Projects/diet-project2/api-server.md`
- **Contains:**
  - All 35 endpoints listed
  - Request/response formats
  - Handler structure
  - Rate limiting rules
  - Database schema relationships

**Why Read:** When integrating with the API or implementing new endpoints.

---

### Research & Methodology

**research-notes.md**
- **Location:** Vault: `projects/diet-program/research-notes.md`
- **Contains:**
  - MacroFactor philosophy & algorithms
  - SBS Diet Guide summaries
  - Weight-delta TDEE methodology
  - Protein/fat/carb recommendations
  - RED-S warning thresholds

**Why Read:** For background on why decisions were made (design philosophy).

---

### Architecture Overview

**INDEX.md**
- **Location:** Vault: `projects/diet-program/INDEX.md`
- **Contains:**
  - v2 architecture diagram
  - Feature checklist (all ✅)
  - Build & run instructions
  - Quick links to sub-documents

**Why Read:** To understand the overall project structure and all features.

---

### Historical Context (Archived)

**tracker-plan.md**
- **Status:** ARCHIVED (TDEE algorithm section outdated)
- **Why:** Historical reference; TDEE algorithm superseded by tiered method
- **Safe to read:** Yes, but ignore algorithm section (see adaptive-tdee-calorie-targets instead)

---

## 🎯 The Five Immutable Rules (TL;DR)

1. **Observed TDEE > Profile Estimates** — Your data beats static estimates
2. **Max ±500 kcal/week swing** — `const MaxWeeklyCalorieSwing = 500`
3. **Macros from final damped calories** — Never from ideal/pre-damping
4. **Maintenance goal special case** — Skip goal multiplier, use observed directly
5. **Emergency override ≤ timing only** — >15% divergence unlocks 5-day wait (damping still applies)

---

## 🛠️ Code Locations

### Backend (Go)

| Component | File | Lines | What It Does |
|-----------|------|-------|-------------|
| Observed TDEE | `services/calculator.go` | 50–150 | Tiered TDEE computation (LinReg/RLS/EMA) |
| Damping Logic | `services/calculator.go` | 506–541 | Clamps delta to ±500 kcal |
| Emergency Override | `services/calculator.go` | 580–593 | Decides if override should trigger |
| Macro Distribution | `services/nutrition/macros.go` | Full file | ComputeTargets() implementation |
| API Handlers | `internal/handlers/calculations.go` | All routes | `/v1/calc/*`, `/v1/dashboard` |
| Target Snapshot | `internal/handlers/targets.go` | PUT handler | Saves targets + history |

### Frontend (TUI)

| Component | File | What It Does |
|-----------|------|-------------|
| Check-In Screen | `cmd/tui/ui/checkin.go` | Display, toggle eat-back, accept/skip |
| Dashboard | `cmd/tui/ui/dashboard.go` | Show TDEE, macros, weekly stats |

### Frontend (PWA)

| Component | File | What It Does |
|-----------|------|-------------|
| Check-In Page | `ui/web/src/routes/checkin/+page.svelte` | Browser version of check-in screen |
| Dashboard | `ui/web/src/routes/dashboard/+page.svelte` | Dashboard for web |

---

## 🚀 Next Steps for Development

### Phase 1: Understand the Design (1-2 hours)

1. **Read QUICK_REFERENCE.md** (10 min)
   - Get the 5 rules
   - Learn code locations
   
2. **Read WEEKLY_CHECKIN_DESIGN.md** (30 min)
   - Understand full flow
   - Learn design rationale
   
3. **Skim adaptive-tdee-calorie-targets.md** (vault) (15 min)
   - Verify code matches design
   - Check safety checklist

### Phase 2: Review Implementation (1-2 hours)

1. **Check services/calculator.go**
   - ComputeWeeklyTargetsWithExercise() [damping logic]
   - ShouldTriggerEmergencyOverride() [override decision]
   
2. **Check cmd/tui/ui/checkin.go**
   - How UI calls the backend
   - How user interacts with check-in
   
3. **Check internal/handlers/calculations.go**
   - How API exposes the feature

### Phase 3: Test & Verify (1-2 hours)

1. **Unit tests** for damping logic
2. **Integration tests** for full flow
3. **Edge cases:**
   - Emergency override (>15% divergence)
   - Maintenance goal (no goal multiplier)
   - Floor guard (never below 1200 kcal)
   - Macro mismatch (post-damping recalc)

---

## 📊 Project Status

| Component | Status | Last Verified |
|-----------|--------|---------------|
| Observed TDEE (Tiered) | ✅ Complete | 2026-04-15 |
| Damping Filter (±500 kcal) | ✅ Complete | 2026-04-15 |
| Macro Recalculation | ✅ Complete | 2026-04-15 |
| Maintenance Goal Special Case | ✅ Complete | 2026-04-15 |
| Emergency Override (15% threshold) | ✅ Complete | 2026-04-15 |
| TUI Check-In Screen | ✅ Complete | 2026-04-28 |
| PWA Check-In Page | ✅ Complete | 2026-04-28 |
| API Endpoints | ✅ Complete | 2026-04-28 |
| Build Status | ✅ Clean (0 errors) | 2026-04-28 |

---

## 🔍 How to Use This Documentation

### I want to understand the weekly check-in at a high level
→ Read QUICK_REFERENCE.md (5 min)

### I need to implement a new feature
→ Read WEEKLY_CHECKIN_DESIGN.md + check code locations (45 min)

### I need to debug a bug in the damping logic
→ Read WEEKLY_CHECKIN_DESIGN.md "Rule 2: Max Weekly Calorie Swing" section (10 min)

### I need to understand why a specific decision was made
→ Read the rationale in WEEKLY_CHECKIN_DESIGN.md (10 min per topic)

### I need to verify the code matches the design
→ Compare adaptive-tdee-calorie-targets.md (vault) against services/calculator.go (30 min)

### I need to plan an enhancement
→ Read WEEKLY_CHECKIN_DESIGN.md "Future Enhancements" section (5 min)

---

## 🔗 Vault Access

To view vault documents, use:

```bash
# Navigate to vault root
cd /home/dfgoodfellow2/Obsidian/Brain/

# View a document
cat "projects/diet-program/adaptive-tdee-calorie-targets.md"

# Search for content
grep -r "500" projects/diet-program/
```

Or open in Obsidian:
1. Open Obsidian app
2. Vault: `/home/dfgoodfellow2/Obsidian/Brain/`
3. Navigate to `projects/diet-program/`

---

## 📞 Questions?

| Question | Answer | Resource |
|----------|--------|----------|
| What are the 5 rules? | See TL;DR section above | QUICK_REFERENCE.md |
| How does damping work? | ±500 kcal clamp on delta | WEEKLY_CHECKIN_DESIGN.md, Rule 2 |
| When does emergency override trigger? | >15% TDEE divergence + not low confidence | WEEKLY_CHECKIN_DESIGN.md, Rule 5 |
| Where's the code? | See code locations table | This file, "Code Locations" |
| Why was this designed this way? | See design rationale | WEEKLY_CHECKIN_DESIGN.md |
| What's the current status? | All ✅ complete, clean build | Project Status table above |

---

## 📝 Document Metadata

| File | Size | Lines | Created | Last Updated |
|------|------|-------|---------|--------------|
| QUICK_REFERENCE.md | 5.2 KB | 250 | 2026-04-30 | 2026-04-30 |
| WEEKLY_CHECKIN_DESIGN.md | 17 KB | 650 | 2026-04-30 | 2026-04-30 |
| DESIGN_INDEX.md (this) | 6 KB | 300 | 2026-04-30 | 2026-04-30 |

---

**Maintained by:** The Librarian (Obsidian Knowledge Retrieval)  
**Last Generated:** 2026-04-30  
**Status:** ✅ Ready for Development
