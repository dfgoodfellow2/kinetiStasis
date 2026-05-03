# Diet Tracker UI Architecture Diagram

## Component Tree

```
App.svelte (Root)
├── Auth State Check (onMount)
├── Nav.svelte (Persistent Desktop Header — hidden md:flex)
│   ├── Logo Button (→ Dashboard)
│   ├── Desktop Nav
│   │   ├── Nav Links with active state
│   │   ├── Profile Link
│   │   └── Logout Button
│   └── State: $state active page for highlight
│
├── BottomNav.svelte (Persistent Mobile Footer — md:hidden)
│   ├── Fixed Position (bottom-0 left-0 right-0 z-50)
│   ├── iOS Safe Area Support
│   ├── 6 Tab Navigation
│   │   ├── Dashboard (icon + label)
│   │   ├── Log Meal
│   │   ├── Check-In
│   │   ├── History
│   │   ├── Workout
│   │   └── Profile
│   └── Active State: emerald border-t-2 + text color
│
└── Main Content Area (max-w-screen-xl, padding-bottom for mobile nav)
    ├── Login Page (currentPage === 'login')
    │   └── Login.svelte
    │
    ├── Dashboard (currentPage === 'dashboard')
    │   ├── Dashboard.svelte
    │   ├── VitalsRing.svelte (Macro progress)
    │   ├── WeightSparkline.svelte (7-day trend)
    │   ├── StreakGrid.svelte (Daily checklist)
    │   ├── Card.svelte (Targets card)
    │   ├── Card.svelte (Weekly stats)
    │   └── Alert.svelte (Error/success)
    │
    ├── LogMeal (currentPage === 'logmeal')
    │   ├── LogMeal.svelte
    │   ├── Form (manual entry)
    │   │   ├── protein_g, carbs_g, fat_g, fiber_g
    │   │   └── meal_notes
    │   ├── AITab.svelte (AI parse mode)
    │   │   ├── Text input
    │   │   ├── Gemini API call
    │   │   └── Auto-fill form
    │   ├── Button: Save/Submit
    │   └── Alert.svelte (Status)
    │
    ├── CheckIn (currentPage === 'checkin')
    │   ├── CheckIn.svelte
    │   ├── Form
    │   │   ├── weight_kg
    │   │   ├── waist_cm
    │   │   ├── grip_kg
    │   │   ├── bolt_ms
    │   │   ├── sleep_hours
    │   │   └── feel (1-10)
    │   ├── Card: Display metrics
    │   └── Button: Submit
    │
    ├── WorkoutLog (currentPage === 'workoutlog')
    │   ├── WorkoutLog.svelte
    │   ├── Tab Toggle: AI | Simple | YAML
    │   ├── AITab.svelte (AI parse)
    │   ├── SimpleTab (Manual form)
    │   │   ├── Exercise picker
    │   │   ├── Set/rep tracker
    │   │   └── Inline builder
    │   ├── YAMLTab.svelte (YAML import)
    │   └── Button: Save
    │
    ├── History (currentPage === 'history')
    │   ├── History.svelte
    │   ├── Tab Toggle: Summary | Detailed
    │   └── Table: 30-day nutrition logs
    │
    ├── WorkoutHistory (currentPage === 'workouthistory')
    │   ├── WorkoutHistory.svelte
    │   └── Table: 30-day workouts (grouped by date)
    │
    ├── Profile (currentPage === 'profile')
    │   ├── Profile.svelte
    │   ├── Form
    │   │   ├── sex, age_years
    │   │   ├── height_cm, weight_kg
    │   │   ├── activity_level
    │   │   └── units (imperial/metric)
    │   └── Button: Save
    │
    ├── BodyFat (currentPage === 'bodyfat')
    │   ├── BodyFat.svelte
    │   ├── Toggle: Navy method | Skinfold
    │   ├── Form inputs
    │   └── Result: Color-coded BF%
    │
    ├── Measurements (currentPage === 'measurements')
    │   ├── Measurements.svelte
    │   ├── Form
    │   │   ├── neck_cm, chest_cm
    │   │   ├── waist_cm, hips_cm
    │   │   ├── thigh_cm, bicep_cm
    │   │   └── Button: Log
    │   └── History table
    │
    ├── Targets (currentPage === 'targets')
    │   ├── Targets.svelte
    │   ├── Form
    │   │   ├── protein_g
    │   │   ├── carbs_g
    │   │   ├── fat_g
    │   │   └── Button: Save
    │   └── Card: TDEE reference
    │
    └── Export (currentPage === 'export')
        ├── Export.svelte
        ├── Toggle: Nutrition | Workouts | Combined
        ├── Toggle: Format (Markdown | CSV)
        ├── Button: Copy
        ├── Button: Download
        └── Textarea: Preview

Shared Components
├── Nav.svelte — Desktop top navbar with active state
├── BottomNav.svelte — Mobile fixed bottom tab bar (NEW)
├── Card.svelte — Dark card container
├── Spinner.svelte — Loading animation
├── Alert.svelte — Error/success/warning banners
├── VitalsRing.svelte — Circular macro progress
├── WeightSparkline.svelte — 7-day trend sparkline
├── StreakGrid.svelte — Daily checklist grid
├── AITab.svelte — AI form parser (used in LogMeal, WorkoutLog)
└── YAMLTab.svelte — YAML import/export (used in WorkoutLog)
```

## Data Flow Diagram

```
User Interaction
     ↓
Page Component (e.g., LogMeal.svelte)
     ↓
Form with API field names
(protein_g, carbs_g, etc.)
     ↓
API Client (lib/api.js)
     ↓
/v1 Endpoint (POST/GET/PUT)
     ↓
Backend (cmd/api/main.go)
     ↓
SQLite Database
     ↓
Response JSON
     ↓
API Client parses JSON
     ↓
Component updates $state
     ↓
Svelte reactivity updates DOM
     ↓
User sees updated data
```

## Authentication Flow

```
User starts app
    ↓
App.svelte onMount
    ↓
api.me() call
    ├─ Success → setUser(u), setCurrentPage('dashboard')
    └─ Failure → setCurrentPage('login')
    ↓
If Login:
    ├─ api.login(email, password)
    ├─ Check status
    ├─ Cookies set (access_token, refresh_token)
    ├─ setUser(u)
    └─ setCurrentPage('dashboard')
    ↓
API calls include credentials: 'include'
    ├─ Valid token → proceed
    ├─ 401 error → api.refresh()
    │   ├─ Refresh token valid → new access_token
    │   ├─ Retry original request
    │   └─ Continue
    └─ 401 after refresh → api.logout(), setCurrentPage('login')
```

## Navigation State Management

```
App.svelte
├── Route page changes via setCurrentPage(page)
│
Nav.svelte (Desktop — hidden md:flex)
├── Receives $store.currentPage
├── Highlights active link with emerald border-b
├── goto(page) → setCurrentPage(page)
└── doLogout() → api.logout(), setUser(null), setCurrentPage('login')

BottomNav.svelte (Mobile — md:hidden)
├── Fixed position: bottom-0 left-0 right-0 z-50
├── iOS safe area: padding-bottom: env(safe-area-inset-bottom)
├── Receives $store.currentPage
├── Highlights active tab with emerald border-t-2 + text color
├── goto(page) → setCurrentPage(page)
└── 6 tabs (Dashboard, LogMeal, CheckIn, History, Workout, Profile)
    └── Each button has aria-current="page" when active + aria-label
```

## Form Data Flow

```
Form Component (e.g., LogMeal.svelte)
│
├─ Initialize formData with API field names
│  formData = {
│    protein_g: '',
│    carbs_g: '',
│    fat_g: '',
│    fiber_g: '',
│    meal_notes: ''
│  }
│
├─ Bind inputs: bind:value={formData.protein_g}
│
├─ On submit:
│  ├─ Validate form
│  ├─ Call api.logMeal(formData)
│  ├─ Send JSON.stringify(formData) to /v1/nutrition/log
│  ├─ Server validates field names
│  ├─ Server stores in database
│  └─ Return success/error
│
├─ Handle response:
│  ├─ Success → show Alert, clear form, update history
│  └─ Error → show Alert with error message
│
└─ Update component state
   └─ Component re-renders via Svelte reactivity
```

## PWA Installation & Caching

```
User visits app
    ↓
vite-plugin-pwa registers service worker
    ↓
Service Worker lifecycle
├─ Install event
│  └─ Cache static assets (manifest.json config)
├─ Activate event
│  └─ Clean up old caches
└─ Fetch event
    ├─ Navigation → network-first
    ├─ Assets (.js/.css) → cache-first
    └─ API (/v1/*) → bypass cache
    ↓
User installs app
    ├─ Browser install prompt (Chrome, Edge, Firefox)
    ├─ App icon added to home screen
    ├─ Standalone mode (fullscreen, no address bar)
    ├─ Uses manifest theme color (#10b981)
    └─ Splash screen with manifest name
    ↓
App launches
    ├─ Service worker serves cached assets
    ├─ Fast initial load
    └─ Network requests fetch fresh data
    ↓
Offline support (Phase 2 partial, Phase 3 full)
    ├─ Currently: Static assets work offline
    ├─ Planned: Sync queue for offline changes
    └─ Planned: Background sync on reconnect
```

