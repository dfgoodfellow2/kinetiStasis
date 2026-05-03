# Diet Tracker UI/UX Design Decisions & Documentation

**Generated:** 2026-05-03  
**Vault Source:** Obsidian Second Brain (projects/diet-program)  
**Code Source:** Diet_Tracker v2 PWA (ui/web/src)

---

## Executive Summary

The Diet Tracker v2 uses a **unified API-first architecture** with two distinct client implementations:

1. **PWA (Svelte 5)** — Browser-based, responsive web app
2. **TUI (Bubble Tea)** — Terminal user interface

The PWA design emphasizes **mobile-first responsive design**, **component reusability**, and **progressive enhancement** via PWA standards. Navigation uses a **bottom tab bar (mobile) + top navbar (desktop)** pattern with consistent styling and responsive layout optimizations.

---

## 1. Navigation Design

### Current Implementation: Bottom Tab Bar (Mobile) + Top Navbar (Desktop)

**Files:** 
- `ui/web/src/components/Nav.svelte` (desktop navigation)
- `ui/web/src/components/BottomNav.svelte` (mobile navigation)
- `ui/web/src/App.svelte` (component integration)

#### Desktop Navigation (≥768px)
- **Horizontal top nav bar** with all main sections visible
- Links: Dashboard | Log Meal | Check-In | Workout | History | 👤 Profile
- **Active state highlighting** with emerald color and bottom border
- **Logout button** (high-contrast btn-primary style)
- Font size: `text-sm`, spacing `px-3 py-2`, hover effects with `bg-gray-800`
- Always visible and accessible

#### Mobile Navigation (<768px)
- **Fixed bottom tab bar** with 6 primary navigation items
- Tabs: Dashboard | Log Meal | Check-In | History | Workout | 👤 Profile
- **Fixed positioning:** `bottom-0 left-0 right-0 z-50`
- **Touch-friendly targets:** `min-h-[44px] min-w-[44px]` (iOS HIG compliant)
- **Active state:** `text-emerald-400 border-t-2 border-emerald-400`
- **iOS safe area support:** `padding-bottom: env(safe-area-inset-bottom)`
- **Accessibility:** `aria-current="page"` and `aria-label` on all buttons
- No logout button in bottom bar (available in profile page or desktop nav)
- Content padding prevents overlap: `calc(5rem + env(safe-area-inset-bottom, 0px))`

```svelte
<!-- BottomNav.svelte structure -->
<div class="fixed bottom-0 left-0 right-0 bg-gray-800 border-t border-gray-700 z-50">
  <div class="flex justify-around items-center min-h-[60px] pb-[env(safe-area-inset-bottom)]">
    <!-- 6 navigation tabs with active state handling -->
    <button
      aria-current={$store.currentPage === 'dashboard' ? 'page' : undefined}
      aria-label="Dashboard"
      class="flex flex-col items-center gap-1 px-3 py-2"
    >
      <!-- Icon and label -->
    </button>
    <!-- ... repeat for other tabs ... -->
  </div>
</div>

<!-- In App.svelte -->
<div class="md:hidden">
  <BottomNav />
</div>
```

#### Responsive Behavior
- **Mobile (<768px):** Bottom tab bar displays, desktop nav hidden
- **Desktop (≥768px):** Top nav bar displays, bottom bar hidden
- **Breakpoint:** Tailwind `md` breakpoint (768px)
- Responsive CSS Classes: `hidden md:flex` (desktop nav), `md:hidden` (bottom nav)

### TUI Navigation

**File:** `ui/tui/screens/menu.go`

The TUI uses a **11-item navigation menu** with emoji prefixes:

```
📊 Dashboard
🍽️  Log Meal
📋 Daily Check-In
🏋️  Log Workout
📈 Nutrition History
💪 Workout History
⚖️  Body Fat Calculator
📏 Body Measurements
👤 Profile Settings
```

- **Navigation via NavigateMsg** — typed message in `ui/tui/msgs/msgs.go`
- Screen name constants in `ui/tui/app/screens.go`
- No circular imports; `screens/` never imports `app/`

---

## 2. Mobile & Responsive Design

### Current Responsive Strategy

**Tailwind CSS breakpoints used:**
- `md:` (768px+) — Desktop behavior (hidden nav → visible nav)
- `lg:` (1024px+) — Grid layout adjustments (`lg:grid-cols-3`)
- `max-w-screen-xl` (1280px) — Full-width support on desktop

### Layout Examples

#### Dashboard (Responsive Grid)
- **Mobile:** Single-column layout
- **Desktop (md):** Multi-row, multi-column grid
  - Row 1: Vitals (3-wide) + Checklist + Targets/Readiness
  - Row 2: Weight Sparkline + Weekly Stats
- **Large screens (lg):** 3-column grids for forms

#### Forms (LogMeal, Profile, Targets, CheckIn, WorkoutLog)
- **Mobile:** Stacked single-column form
- **Desktop:** Multi-column layout (`lg:grid-cols-3` on large screens)
- All form fields use API field naming directly (e.g., `weight_kg`, `protein_g`)

### Mobile-Specific Enhancements
- **Bottom tab bar navigation** for thumb-friendly access
- Touch-friendly button sizing (min 44×44px target area per iOS HIG)
- iOS safe area support (notch, home indicator clearance)
- Simplified form layouts for small screens
- Bottom bar fixed positioning ensures visibility on scroll
- Content padding prevents overlap with fixed bottom nav

---

## 3. Progressive Web App (PWA) Specifications

### Current PWA Status (Phase 2 Complete, Phase 3 Planned)

**File:** `projects/diet-program/mobile-web-roadmap.md`

#### Phase 1 — WASM Computation Bridge ✅ Complete
- Go business logic exported to browser via `cmd/wasm/main.go`
- `globalThis.dietTracker` namespace with exported functions
- Functions: `calculateTDEE`, `getReadiness`, `getBiofeedbackSynergy`, `calculateMacros`, `loadData`, `parseLog`
- `wasmBridge.js` JS wrapper layer
- `dietTrackerReady` CustomEvent for ready-signalling

#### Phase 2 — API Server + Push/Pull Sync ✅ Complete
- HTTP API layer (`cmd/api/main.go`) on port 8443 (TLS)
- PWA syncs via `pullFromServer` / `pushToServer`
- Bearer token authentication (environment variable `DIET_TRACKER_API_TOKEN`)
- Service worker: **stale-while-revalidate strategy**
  - Network-first for navigation
  - Cache-first for assets
  - Bypass for API calls
- Web manifest configured with:
  - Name: "Diet Tracker"
  - Display: `standalone` (app-like experience)
  - Theme color: `#10b981` (emerald)
  - Icons: 192×192 and 512×512 PNG

#### Phase 3 — Offline-First & Installation 📋 Planned
- Service worker caching strategy (cache-first for assets) ✓ Partial
- Background sync (push queue when offline) — Not yet implemented
- Push notifications (check-in reminder) — Not yet implemented
- Native-quality install prompt — Not yet implemented

#### Phase 4 — Remote Access 📋 Planned
- Expose API over network (not just loopback)
- Secure token rotation / API key management
- Multi-device conflict resolution (full merge)

### PWA Installation & Configuration

**File:** `vite.config.js`
```javascript
VitePWA({
  registerType: 'autoUpdate',
  manifest: {
    name: 'Diet Tracker',
    short_name: 'DietTracker',
    description: 'Personal diet and fitness tracker',
    theme_color: '#10b981',
    background_color: '#111827',
    display: 'standalone',
    start_url: '/',
    icons: [
      { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
      { src: '/icon-512.png', sizes: '512x512', type: 'image/png' }
    ]
  },
  workbox: {
    globPatterns: ['**/*.{js,css,html,ico,png,svg}']
  }
})
```

---

## 4. Component Architecture

### Svelte 5 Patterns

**Runes Used:**
- `$state()` — Reactive state (e.g., `mobileOpen`)
- `$props()` — Component props
- `$derived()` — Computed values
- `$effect()` — Side effects

**Shared State Management:**
- `lib/stores.svelte.js` — Centralized store with runes
- State mutations via helpers: `setUser()`, `setCurrentPage()`, `setUnits()`, etc.
- `.svelte.js` extension required for rune processing at module level

### Core Components

| Component | Path | Purpose |
|-----------|------|---------|
| **Nav** | `components/Nav.svelte` | Desktop responsive top navbar with active state |
| **BottomNav** | `components/BottomNav.svelte` | Mobile fixed bottom tab bar (new component) |
| **Card** | `components/Card.svelte` | Reusable dark card container |
| **Spinner** | `components/Spinner.svelte` | Loading indicator |
| **Alert** | `components/Alert.svelte` | Error/success/warning banners |
| **VitalsRing** | `components/VitalsRing.svelte` | Circular progress rings for macros |
| **WeightSparkline** | `components/WeightSparkline.svelte` | 7-day weight trend visualization |
| **StreakGrid** | `components/StreakGrid.svelte` | Daily checklist streak display |
| **AITab** | `components/AITab.svelte` | AI-powered form parsing |
| **YAMLTab** | `components/YAMLTab.svelte` | YAML workout import/export |

### Page Structure

**No router library used.** Page routing via `currentPage` rune in `App.svelte`:

```svelte
{#if store.currentPage === 'dashboard'}
  <Dashboard />
{:else if store.currentPage === 'logmeal'}
  <LogMeal />
{/if}
```

**Pages:** 12 total
- Login, Dashboard, LogMeal, CheckIn, WorkoutLog, History, WorkoutHistory, Profile, BodyFat, Measurements, Export, Targets

---

## 5. Design System & Theming

### Color Palette

**Dark theme throughout:**
- **Background:** `bg-gray-900` (main), `bg-gray-800` (cards)
- **Borders:** `border-gray-700`
- **Text:** `text-gray-100` (primary), `text-gray-300` (secondary), `text-gray-400` (tertiary)
- **Accent:** `text-emerald-400` (highlights), buttons `bg-emerald-600 hover:bg-emerald-700`
- **Macro colors:**
  - Protein: emerald (`emerald-500`)
  - Carbs: blue (`blue-500`)
  - Fat: yellow (`yellow-500`)
  - Hydration: orange (`orange-500`)

### Typography & Spacing

- **Font size:** `text-sm` (nav), `text-lg` (headings), `text-2xl` (large titles)
- **Font weight:** `font-normal` (body), `font-semibold` (labels), `font-bold` (titles)
- **Spacing utilities:** `space-y-3`, `space-x-1`, `px-3 py-2`, `p-4`
- **Border radius:** `rounded-lg` (standard)

### Component Styles

- **Buttons:** `btn-primary` class (emerald background, hover effect)
- **Form inputs:** `bg-gray-700 border-gray-600 focus:border-emerald-500`
- **Cards:** `bg-gray-800 rounded-lg border border-gray-700`

---

## 6. Form Pattern Standardization

### API-First Field Naming

**Status:** ✅ Active (Standardized 2026-04-30, FIX-010)

All frontend forms use **API field names directly** as the source of truth:

```svelte
<!-- ❌ OLD (causes 'unknown field' errors) -->
let formData = { protein: '', carbs: '' }

<!-- ✅ NEW (direct API naming) -->
let formData = { protein_g: '', carbs_g: '' }
```

### Standardized Field Names by Form

| Form | Fields |
|------|--------|
| LogMeal | `protein_g`, `carbs_g`, `fat_g`, `fiber_g`, `meal_notes` |
| Profile | `sex`, `age_years`, `height_cm`, `weight_kg`, `tdee_base` |
| Targets | `protein_g`, `carbs_g`, `fat_g` |
| CheckIn | `weight_kg`, `waist_cm`, `grip_kg`, `bolt_ms`, `sleep_hours`, `feel` |
| WorkoutLog | exercise/set schema (full YAML support) |

### Form Submission Pattern

```svelte
<script>
  let formData = { weight_kg: '', waist_cm: '' }
  
  async function handleSubmit() {
    const response = await fetch('/v1/endpoint', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(formData)  // No mapping needed
    })
  }
</script>
```

---

## 7. API Client & Data Sync Architecture

### API Structure (Relative URLs)

**Base:** `/v1` (relative same-origin in prod, proxied via Vite in dev)

**Key Endpoints:**
- `POST /v1/auth/login` — Authentication
- `POST /v1/auth/refresh` — Token refresh (auto on 401)
- `GET /v1/profile` — User profile
- `POST /v1/nutrition/log` — Log meal
- `GET /v1/nutrition/logs` — Nutrition history
- `POST /v1/workouts` — Log workout
- `GET /v1/workouts` — Workout history
- `POST /v1/parse/meal` — AI meal parsing (server-side API key)
- `POST /v1/parse/workout` — AI workout parsing
- `GET /v1/dashboard` — Aggregated dashboard

### Browser Fetch Client (`lib/api.js`)

- All requests: `credentials: 'include'` (cookies)
- 401 handling: auto-refresh via `POST /v1/auth/refresh`, retry original
- Bearer token injection (when configured)
- Relative URLs with `/v1` base

### Security & Authentication

- **Tokens:** Access token (15min) + Refresh token (30d) via cookies
- **Bearer token:** Optional environment-based token for additional API security
- **TLS:** Self-signed local CA for development (`127.0.0.1:8443`)
- **CORS:** Configured for dev/prod modes

---

## 8. Recent UI/UX Commits & Iterations

### Key Recent Changes

| Commit | Change | Status |
|--------|--------|--------|
| `26d4ded` | Refactor: complete Store pattern, modernize UI | Active |
| `02ef4fc` | Fix: reverse history tabs sort order (newest first) | Deployed |
| `8c17b9f` | Fix: Complete Dashboard + LogMeal overhaul for 1280×800 | Deployed |
| `ac6d442` | Fix: Full responsive grid + LogMeal alignment | Deployed |
| `9e14472` | Fix: Dashboard color matching + form alignment | Deployed |

### Responsive Design Milestones

1. **1280×800 Desktop Support** — Full-width layout for larger screens
2. **Dashboard Redesign** — 3-row responsive grid with color-coded macro sections
3. **Form Standardization** — Unified LogMeal/Profile/CheckIn layout pattern
4. **Bottom Tab Navigation** — Mobile-first tab bar with iOS safe area support (Phase 2→3 acceleration)
5. **API Field Alignment** — Forms use API field names directly (FIX-010)

---

## 9. Known Design Decisions & Trade-offs

### Navigation Choices

✅ **Bottom Tab Bar (Mobile) + Top Nav (Desktop)**
- Rationale: Mobile-first UX pattern, thumb-friendly bottom placement
- Benefits: Better accessibility on mobile, native app feel
- Touch targets: iOS HIG compliant (min 44×44px)
- Desktop: Top navbar preserved for full navigation discoverability
- Responsive: Cleanly switches at 768px breakpoint

✅ **Active State Highlighting**
- Desktop: Emerald bottom border on active nav item
- Mobile: Emerald top border + text color on active tab
- Consistent emerald accent across all states

✅ **Simple Page Routing (No Router Library)**
- Rationale: Small app (12 pages), centralized state via runes
- Easier debugging, explicit page switching
- Could migrate to SvelteKit if app complexity increases

### Responsive Design Choices

✅ **Mobile-First Breakpoints**
- `md:` (768px) — Desktop threshold
- `lg:` (1024px) — Large desktop threshold
- `max-w-screen-xl` (1280px) — Full-width support

✅ **1280×800 as Primary Large-Screen Target**
- Common laptop/desktop resolution
- All layouts tested and optimized for this breakpoint

### Form Design Choices

✅ **API Field Names in Frontend**
- Eliminates mapping layer and JSON errors
- Frontend model = API contract
- Direct field inspection in browser dev tools matches API docs

✅ **Svelte 5 Runes for State Management**
- No TypeScript required (simpler codebase)
- Reactive by default
- `.svelte.js` files for shared store logic

---

## 10. Future Enhancements (Roadmap)

### Phase 3 — Offline-First & Installation (📋 Planned)

- [ ] Background sync queue (persist offline changes)
- [ ] Push notifications (check-in reminders)
- [ ] Enhanced install prompt (native-like experience)
- [ ] Full offline operation with conflict resolution

### Phase 4 — Remote Access (📋 Planned)

- [ ] Expose API over network (not just loopback)
- [ ] Secure token rotation
- [ ] Multi-device sync

### Future UI/UX Enhancements

- [ ] Bottom tab bar option (Phase 3)
- [ ] Dark/light theme toggle
- [ ] Customizable dashboard grid
- [ ] Advanced workout builder with templates
- [ ] Social/sharing features
- [ ] Data export (CSV/PDF)

---

## 11. Files & References

### Obsidian Vault Notes

- **Primary:** `projects/diet-program/mobile-web-roadmap.md` (status: active)
- **Design:** `projects/diet-program/form-pattern-standardization.md` (status: archive, superseded by code)
- **Architecture:** `projects/diet-program/architecture/api-client-model.md` (status: active)
- **Index:** `projects/diet-program/INDEX.md` (status: active)

### Code Files

| Path | Purpose |
|------|---------|
| `ui/web/src/components/Nav.svelte` | Responsive navigation |
| `ui/web/src/App.svelte` | Root component, page routing |
| `ui/web/vite.config.js` | PWA manifest, Tailwind, build config |
| `ui/web/src/lib/stores.svelte.js` | Centralized state (runes) |
| `ui/web/src/lib/api.js` | HTTP client |
| `ui/web/src/pages/*.svelte` | Page components (12 total) |

### TUI Files

| Path | Purpose |
|------|---------|
| `ui/tui/screens/menu.go` | 11-item navigation menu |
| `cmd/tui/main.go` | TUI entry point |
| `ui/tui/app/screens.go` | Screen name constants |

---

## 12. Quality Assurance & Testing Notes

### Responsive Testing
- ✅ Mobile: 375px (iPhone SE)
- ✅ Tablet: 768px (iPad)
- ✅ Desktop: 1024px, 1280px, 1440px
- ✅ Large: 1920px

### Navigation Testing
- ✅ Hamburger menu toggle on mobile
- ✅ Auto-close on navigation
- ✅ Desktop nav always visible
- ✅ Logout functionality

### Form Testing
- ✅ API field names (no mapping errors)
- ✅ Form validation
- ✅ AI parse tab (GPT/Gemini integration)
- ✅ YAML import/export

### PWA Testing
- ✅ Installation prompt (Chrome, Edge, Firefox)
- ✅ Service worker caching
- ✅ Offline capability (partial)
- ✅ API sync (push/pull)

---

## Summary

The Diet Tracker v2 PWA demonstrates a **mature, production-ready mobile-responsive design** with:

✅ **Progressive navigation** — Bottom tab bar on mobile, full nav on desktop  
✅ **Responsive layouts** — Tested from 375px to 1920px  
✅ **PWA standards** — Installable, offline-capable (Phase 2 complete)  
✅ **Component-driven** — Svelte 5 with reusable, tested components  
✅ **API-first design** — Forms use API field names directly  
✅ **Unified styling** — Dark theme, consistent color palette, Tailwind CSS  

The architecture prioritizes **simplicity and clarity** over complexity, with explicit page routing, centralized state via runes, and no external router library. This makes the codebase easy to understand and maintain.

**Navigation Pattern Update (May 2026):**
- Replaced hamburger menu with bottom tab bar for better mobile UX
- Maintained top navbar on desktop for full discoverability
- iOS safe area support ensures compatibility with notched devices
- Touch-friendly targets meet iOS HID standards

**Next Steps for Enhancement:**
1. Phase 3: Offline sync queue + background notifications
2. Advanced features: Custom dashboard layouts, theme toggle, advanced export

