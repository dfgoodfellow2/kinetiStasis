# Diet Tracker UI/UX Design Documentation Index

**Last Updated:** 2026-05-03  
**Search Date:** May 3, 2026  
**Status:** Complete ✓

This index provides quick access to all UI/UX design documentation, historical context, and architectural information for the Diet Tracker v2 application.

---

## 📚 Documentation Files

### Primary Documents

1. **UI_UX_DESIGN_SUMMARY.md** (500 lines, 17 KB)
   - **Purpose:** Comprehensive UI/UX design decisions and documentation
   - **Topics Covered:**
     - Navigation design (hamburger menu + top navbar)
     - Mobile & responsive design strategies
     - PWA specifications and roadmap (4 phases)
     - Component architecture (Svelte 5 patterns)
     - Design system & theming (colors, typography, spacing)
     - Form pattern standardization (FIX-010)
     - API client architecture
     - Recent commits & iterations
     - Design decisions & trade-offs
     - Future enhancements (roadmap)
     - Quality assurance & testing
   - **When to Use:** Overview of all design aspects, historical context, decision rationale
   - **Audience:** Designers, frontend developers, architects

2. **UI_ARCHITECTURE.md** (405 lines, 13 KB)
   - **Purpose:** Visual architecture diagrams and technical implementation details
   - **Sections Included:**
     - Component tree (App → Pages → Shared Components)
     - State management structure
     - API client methods & all endpoints
     - Responsive breakpoints
     - Styling layer (Tailwind configuration)
     - PWA service worker flow
     - Data flow diagrams
     - Authentication flow
     - Form data flow
     - Mobile menu state machine
     - TUI architecture (reference)
   - **When to Use:** Implementation details, debugging, visual reference
   - **Audience:** Frontend developers, QA engineers, architects

3. **This File: UI_DESIGN_INDEX.md**
   - **Purpose:** Index and navigation guide
   - **Links To:** All design documents, code files, and Obsidian vault notes

---

## 🔗 Obsidian Vault References

**Vault Location:** `~/.obsidian_vault`  
**Project:** `projects/diet-program`

### Active Design Documents

1. **Mobile Web / PWA Roadmap**
   - **Path:** `projects/diet-program/mobile-web-roadmap.md`
   - **Status:** ACTIVE ✓
   - **Last Updated:** 2026-04-28
   - **Key Content:**
     - 4-phase PWA rollout plan
     - Phase 1: WASM Computation Bridge (✅ Complete)
     - Phase 2: API Server + Sync (✅ Complete)
     - Phase 3: Offline-First & Installation (📋 Planned)
     - Phase 4: Remote Access (📋 Planned)
     - Detailed status table for each phase
   - **Use Case:** PWA roadmap, implementation status

2. **API-Client Architecture**
   - **Path:** `projects/diet-program/architecture/api-client-model.md`
   - **Status:** ACTIVE ✓
   - **Last Updated:** 2026-04-25
   - **Key Content:**
     - API server as single source of truth
     - TUI APIStorage with health-check fallback
     - Security: TLS & Token Authentication
     - Gemini & Server-Side API Key Management
     - Thread safety & concurrency patterns
     - Merge semantics for sync (PWA)
     - Browser offline cache strategy
     - Open issues/gaps
   - **Use Case:** API design patterns, sync strategy, security architecture

3. **Form Pattern Standardization**
   - **Path:** `projects/diet-program/form-pattern-standardization.md`
   - **Status:** ARCHIVE (superseded by code implementation)
   - **Last Updated:** 2026-05-02
   - **Key Content:**
     - API-first field naming convention
     - Form pattern for each page (LogMeal, Profile, Targets, etc.)
     - Standard pattern for new forms
     - Benefits of direct API field naming
     - FIX-010 implementation details
   - **Note:** Archived because current code implementation is the source of truth
   - **Use Case:** Understanding form design principles, new form creation

4. **Diet Program Index**
   - **Path:** `projects/diet-program/INDEX.md`
   - **Status:** ACTIVE ✓
   - **Last Updated:** 2026-04-28
   - **Key Content:**
     - Quick links to all documentation
     - Complete architecture overview
     - Current status (v2: Go API + SQLite + Svelte PWA + Bubble Tea TUI)
     - All feature lists (✅ Complete)
     - Build & run instructions
   - **Use Case:** Master index, project status, quick navigation

5. **PWA Client Documentation**
   - **Path:** `Projects/diet-project2/pwa-client.md`
   - **Status:** ACTIVE ✓
   - **Last Updated:** 2026-04-26
   - **Key Content:**
     - Tech stack (Svelte 5, Tailwind, Vite)
     - File structure (src/components, src/pages, src/lib)
     - Svelte 5 patterns (runes, stores)
     - API client details
     - Embedding in Go binary
     - Theme & styling
     - Pages overview
   - **Use Case:** PWA implementation details, tech stack reference

6. **TUI Client Documentation**
   - **Path:** `Projects/diet-project2/tui-client.md`
   - **Status:** ACTIVE ✓
   - **Last Updated:** 2026-04-26
   - **Key Content:**
     - Entry point & architecture
     - File structure
     - Dependencies (Bubble Tea, Bubbles, Lipgloss)
     - Architecture rules (no circular imports)
     - Screen map (11 screens)
     - Build & run instructions
   - **Use Case:** TUI reference, comparison with PWA

---

## 💻 Code Files

### Navigation Components

**File:** `ui/web/src/components/Nav.svelte` (desktop navigation)
- **Purpose:** Desktop navigation component
- **Features:**
  - Responsive top navbar (768px+)
  - Active state highlighting with emerald accent
  - Logout button
  - Logo with home link
- **Key Classes:** `hidden md:flex`, active state styling
- **When to Reference:** Desktop navigation implementation

**File:** `ui/web/src/components/BottomNav.svelte` (mobile navigation - NEW)
- **Purpose:** Mobile bottom tab bar navigation
- **Features:**
  - Fixed bottom position (bottom-0 left-0 right-0 z-50)
  - 6 primary navigation tabs
  - iOS safe area support: `env(safe-area-inset-bottom)`
  - Touch-friendly targets: `min-h-[44px] min-w-[44px]`
  - Active state: emerald border-t-2 + text color
  - Accessibility: `aria-current="page"` and `aria-label`
- **Key Classes:** `md:hidden`, fixed positioning, active state
- **When to Reference:** Mobile navigation implementation, iOS support

**File:** `ui/web/src/App.svelte` (67+ lines)
- **Purpose:** Root component, page routing
- **Features:**
  - Auth check on mount
  - Conditional page rendering (12 pages)
  - Nav component (persistent desktop header)
  - BottomNav component (persistent mobile footer)
  - Main content area with responsive width and bottom padding
  - Content padding prevents overlap with mobile nav
- **Key Pattern:** No router library, explicit if/else chains for responsive nav
- **When to Reference:** Page routing logic, component tree, responsive layout, authentication flow

### Page Components (12 total)

Located in `ui/web/src/pages/`:
- `Login.svelte` — Authentication
- `Dashboard.svelte` — Overview with widgets
- `LogMeal.svelte` — Manual form + AI parsing
- `CheckIn.svelte` — Daily biometrics
- `WorkoutLog.svelte` — Workout logging
- `History.svelte` — 30-day nutrition history
- `WorkoutHistory.svelte` — 30-day workouts
- `Profile.svelte` — User profile edit
- `BodyFat.svelte` — Body fat calculator
- `Measurements.svelte` — Body measurements
- `Targets.svelte` — Macro targets
- `Export.svelte` — Data export (CSV/Markdown)

### Shared Components (9 total)

Located in `ui/web/src/components/`:
- `Nav.svelte` — Navigation
- `Card.svelte` — Dark card container
- `Spinner.svelte` — Loading indicator
- `Alert.svelte` — Error/success banners
- `VitalsRing.svelte` — Circular macro progress
- `WeightSparkline.svelte` — 7-day trend
- `StreakGrid.svelte` — Daily checklist
- `AITab.svelte` — AI form parser
- `YAMLTab.svelte` — YAML import/export

### State Management

**File:** `ui/web/src/lib/stores.svelte.js`
- **Purpose:** Centralized state management using Svelte 5 runes
- **Exports:**
  - `store` — Central state object
  - `setUser()`, `setCurrentPage()`, `setUnits()`, etc. — State setters
  - `clearUser()` — Logout state reset
- **When to Reference:** State updates, authentication, user context

### API Client

**File:** `ui/web/src/lib/api.js`
- **Purpose:** HTTP client for `/v1` API endpoints
- **Functions:**
  - `req()` — Core fetch wrapper
  - Auth methods: `login()`, `register()`, `logout()`, `me()`, `refresh()`
  - Profile methods: `getProfile()`, `updateProfile()`
  - Nutrition methods: `logMeal()`, `getNutritionLogs()`, `parseMeal()`
  - Workout methods: `logWorkout()`, `getWorkouts()`, `parseWorkout()`
  - And more...
- **Features:**
  - Relative `/v1` base URL
  - 401 auto-refresh on token expiry
  - Bearer token injection
  - `credentials: 'include'` for cookies
- **When to Reference:** API integration, endpoint structure, authentication flow

### Configuration

**File:** `ui/web/vite.config.js`
- **Purpose:** Vite build configuration
- **Key Sections:**
  - PWA manifest configuration (theme color, display mode, icons)
  - Tailwind CSS plugin
  - Svelte plugin
  - Dev server proxy for `/v1` endpoints
- **When to Reference:** Build configuration, PWA settings, dev server setup

---

## 🎯 Design Quick Reference

### Navigation Pattern

**Mobile (< 768px)**
```
┌──────────────────────────────┐
│  Content Area (scrollable)   │
│                              │
└──────────────────────────────┘
┌──────────────────────────────┐
│📊 │🍽️ │📋 │📈 │💪 │👤 │
│Dashboard│LogMeal│CheckIn│
│History│Workout│Profile │
└──────────────────────────────┘
```

**Desktop (≥768px)**
```
┌─ 🥗 Diet Tracker  Dashboard | Log Meal | Check-In | Workout | History | 👤 Profile  [Logout] ┐
```
┌─ 🥗 Diet Tracker ─────── ☰ ─┐
├──────────────────────────────┤
│ Dashboard                    │
│ Log Meal                     │
│ Check-In                     │
│ Workout                      │
│ History                      │
│ 👤 Profile                   │
│ ─────────────────────────    │
│ Logout                       │
└──────────────────────────────┘
```

**Desktop (768px+)**
```
┌─ 🥗 Diet Tracker  Dashboard | Log Meal | Check-In | Workout | History | 👤 Profile  [Logout] ┐
```

### Responsive Breakpoints

- **Mobile:** < 768px (Bottom tab bar, stacked forms, single-column content)
- **Desktop (md):** 768px+ (Visible top nav, multi-column layouts, no bottom nav)
- **Large (lg):** 1024px+ (Grid form adjustments, full layout)
- **Full-width (max-w-screen-xl):** 1280px+ (Optimized for large screens)

### Color System

- **Primary Dark:** `bg-gray-900` (backgrounds)
- **Secondary Dark:** `bg-gray-800` (cards)
- **Text Primary:** `text-gray-100`
- **Text Secondary:** `text-gray-300`
- **Accent:** Emerald `#10b981` (buttons, highlights)
- **Macros:** Emerald (protein) | Blue (carbs) | Yellow (fat) | Orange (hydration)

### Component Hierarchy

```
App (Root)
├── Nav (Persistent)
└── Main Content (max-w-screen-xl)
    ├── Login
    ├── Dashboard
    ├── LogMeal
    ├── ... (10 more pages)
    └── Each page uses:
        ├── Shared Components (Card, Alert, Spinner, etc.)
        └── Sub-components (AITab, YAMLTab, VitalsRing, etc.)
```

---

## 📖 Design Decision Summary

### Navigation ✓
- **Choice:** Bottom tab bar (mobile) + top navbar (desktop)
- **Rationale:** Mobile-first UX, thumb-friendly placement, native app feel
- **Implementation:** BottomNav component with iOS safe area support
- **Alternative:** Hamburger menu (previous pattern, replaced for better UX)

### Routing ✓
- **Choice:** No router library, simple state-based page switching
- **Rationale:** Small app (12 pages), easier debugging, no circular dependencies
- **Alternative:** Could migrate to SvelteKit if app grows

### State Management ✓
- **Choice:** Svelte 5 runes in centralized `.svelte.js` store
- **Rationale:** Reactive by default, no TypeScript required, simpler
- **Alternative:** Could use Pinia or other external store

### Form Design ✓
- **Choice:** API field names directly in forms (no mapping layer)
- **Rationale:** Eliminates JSON errors, frontend = API contract, FIX-010
- **Alternative:** Could use form library (not needed for this complexity)

### PWA ✓
- **Current:** Phase 2 complete (API sync working)
- **Planned:** Phase 3 (offline-first), Phase 4 (remote access)
- **Service Worker Strategy:** Cache-first for assets, network-first for nav, bypass API

---

## 🔄 Recent Changes

### Recent Commits (Git History)

1. **26d4ded** — Refactor: complete Store pattern, modernize UI
2. **02ef4fc** — Fix: reverse history tabs sort order (newest first)
3. **8c17b9f** — Fix: Complete Dashboard + LogMeal overhaul for 1280×800
4. **ac6d442** — Fix: Full responsive grid + LogMeal alignment
5. **9e14472** — Fix: Dashboard color matching + form alignment

### Major Iterations

1. **Form Standardization (FIX-010)** — All forms now use API field names directly
2. **Dashboard Redesign** — 3-row responsive grid with color-coded sections
3. **Full 1280×800 Support** — All layouts optimized for desktop
4. **Bottom Tab Navigation** — Replaced hamburger menu with bottom tab bar (May 2026)
5. **Store Pattern Modernization** — Complete Svelte 5 rune migration

---

## 📊 Status

### Current Version
- **v2** (Go API + SQLite + Svelte PWA + Bubble Tea TUI)
- **Last Updated:** 2026-04-28
- **Branch:** `cleanup/audit-fixes`
- **All Core Features:** ✓ Complete

### PWA Status
- **Phase 1 (WASM Bridge):** ✅ Complete
- **Phase 2 (API + Sync):** ✅ Complete
- **Phase 3 (Offline + Install):** 📋 Planned
- **Phase 4 (Remote Access):** 📋 Planned

### Known Issues / Gaps
- No background sync queue (Phase 3)
- No push notifications (Phase 3)
- MergeAppData doesn't cover all data types (planned improvement)

---

## 🚀 Future Enhancements

### Phase 3 (Offline-First & Installation)
- [ ] Background sync queue
- [ ] Push notifications (check-in reminders)
- [ ] Enhanced install prompt
- [ ] Full offline operation with conflict resolution

### Phase 4 (Remote Access)
- [ ] Network-exposed API
- [ ] Secure token rotation
- [ ] Multi-device sync

### Additional UI/UX Features
- [ ] Dark/light theme toggle
- [ ] Customizable dashboard grid
- [ ] Advanced workout builder
- [ ] Social/sharing features
- [ ] Enhanced data export (PDF)

---

## 📞 Quick Links

### Documentation Files (This Directory)
- `UI_UX_DESIGN_SUMMARY.md` — Full design guide
- `UI_ARCHITECTURE.md` — Architecture diagrams
- `UI_DESIGN_INDEX.md` — This file

### Code Directories
- `ui/web/src/` — PWA source code
- `ui/web/src/components/` — Shared components
- `ui/web/src/pages/` — Page components
- `ui/web/src/lib/` — State & API client

### Obsidian Vault
- `projects/diet-program/mobile-web-roadmap.md` — PWA roadmap
- `projects/diet-program/architecture/api-client-model.md` — API architecture
- `projects/diet-program/INDEX.md` — Project index

### Git History
```bash
git log --oneline --all --grep="ui\|design\|nav\|layout\|responsive" -i
```

---

## 📝 Notes

**Created By:** Librarian (Knowledge Retrieval Agent)  
**Search Method:** mem-hint-mcp semantic search + Obsidian CLI + Git history analysis  
**Data Sources:** Obsidian vault (6 active documents) + Code analysis (21 files)  
**Completeness:** All UI/UX design decisions documented with code references

**For Questions:** Refer to specific documents above or examine code files directly.

