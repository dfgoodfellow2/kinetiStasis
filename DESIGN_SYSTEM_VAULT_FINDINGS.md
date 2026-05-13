# Design System & Frontend Architecture - Vault Search Results
**Report Date:** May 7, 2026  
**Search Scope:** Diet Tracker Project  
**Status:** Complete ✓

---

## Summary

Your Obsidian vault and project contain **comprehensive documentation** for design system, UI architecture, and asset management. The Diet Tracker v2 uses a well-structured, emoji-based icon system with modern frontend architecture (Svelte 5 + Tailwind + PWA).

---

## 📚 Key Documentation Files Found

### 1. **UI_DESIGN_INDEX.md** (451 lines, 17 KB)
**Location:** `/home/dfgoodfellow2/Projects/Personal/Diet_Tracker/UI_DESIGN_INDEX.md`  
**Purpose:** Master index for all UI/UX documentation  
**Key Sections:**
- Links to 6 active design documents
- Component hierarchy overview
- Navigation pattern reference
- Color system specification
- Design decision summary
- Recent changes and iterations

**Best For:** Getting a complete overview of the design system and quickly finding specific documentation.

---

### 2. **UI_ARCHITECTURE.md** (284 lines, 13 KB)
**Location:** `/home/dfgoodfellow2/Projects/Personal/Diet_Tracker/UI_ARCHITECTURE.md`  
**Purpose:** Visual architecture diagrams and technical implementation details  
**Key Sections:**
- Complete component tree (App → Pages → Shared Components)
- State management structure
- Data flow diagrams
- Authentication flow
- Navigation state machine
- Form data flow patterns
- PWA installation & caching strategy
- API endpoints reference

**Best For:** Understanding component structure, debugging layout issues, and implementing new features.

---

### 3. **ICON_SYSTEM_DESIGN.md** (504 lines, 19 KB)
**Location:** `/home/dfgoodfellow2/Projects/Personal/Diet_Tracker/ICON_SYSTEM_DESIGN.md`  
**Purpose:** Complete icon system documentation including navigation, PWA branding, and TUI icons  
**Key Sections:**
- Unicode emoji navigation (6 tabs in BottomNav)
- PWA icon specifications (SVG + PNG variants)
- Icon generation process (Makefile target)
- Design decisions and rationale
- WCAG accessibility compliance
- Known gaps and future enhancements
- Technical specifications for emoji rendering

**Best For:** Understanding the hybrid emoji icon system, asset management, and branding decisions.

---

## 🎯 Design System Overview

### Architecture Pattern
- **Navigation:** Responsive dual-mode (mobile bottom tab bar + desktop top navbar)
- **State Management:** Centralized Svelte 5 runes (`.svelte.js` stores)
- **Routing:** Explicit page switching (no router library)
- **Styling:** Tailwind CSS v4 with custom configuration
- **Component Structure:** Shared components + page-specific components

### Color System
```
Primary Dark: bg-gray-900 (backgrounds)
Secondary Dark: bg-gray-800 (cards)
Text Primary: text-gray-100
Text Secondary: text-gray-300
Accent: Emerald #10b981 (buttons, highlights)
Macro Colors:
  - Emerald (protein)
  - Blue (carbs)
  - Yellow (fat)
  - Orange (hydration)
```

### Responsive Breakpoints
- **Mobile:** < 768px (Bottom tab bar, stacked forms)
- **Desktop (md):** 768px+ (Visible top nav, multi-column layouts)
- **Large (lg):** 1024px+ (Grid form adjustments)
- **Full-width:** 1280px+ (Optimized for large screens)

---

## 🎨 Icon & Asset Management System

### Three-Tier Icon System

#### 1. **Navigation Icons (Unicode Emoji)**
Used in both PWA and TUI applications:

| Icon | Page | Component |
|------|------|-----------|
| 🏠 | Dashboard | BottomNav.svelte (mobile) |
| ➕ | Log Meal | BottomNav.svelte |
| ✅ | Check-In | BottomNav.svelte |
| 📊 | History | BottomNav.svelte |
| 🏋️ | Workout | BottomNav.svelte |
| 👤 | Profile | BottomNav.svelte |
| 🥗 | Logo | Nav.svelte (desktop) |

**File:** `ui/web/src/components/BottomNav.svelte` (added May 4, 2026)  
**Styling:** `text-xl` with 44×44px minimum touch targets  
**Implementation:** Direct emoji characters, no icon fonts

#### 2. **PWA Icons (SVG + PNG)**
**Source SVG:** `ui/web/public/icon.svg`
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <rect width="512" height="512" rx="80" fill="#10b981"/>
  <text x="256" y="340" font-size="300" font-family="serif">🥗</text>
</svg>
```

**Generated Variants:**
- `icon-192.png` — Home screen icon (Android, PWA)
- `icon-512.png` — Splash screen icon (high-density displays)

**Generation Process:**
```bash
make gen-icons  # ImageMagick-based generation
```

Makefile target in project root creates emerald background with centered salad emoji.

#### 3. **TUI Icons (Terminal)**
**File:** `ui/tui/screens/menu.go`  
Uses same emoji prefixes as PWA for consistency:
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

### Asset Locations

**Source Assets:**
- `ui/web/public/icon.svg` — Source vector
- `ui/web/src/components/BottomNav.svelte` — Mobile nav component
- `ui/web/src/components/Nav.svelte` — Desktop nav component

**Generated/Deployed Assets:**
- `ui/web/public/icon-192.png` — Generated by Makefile
- `ui/web/public/icon-512.png` — Generated by Makefile
- `dist/manifest.json` — Generated at build time (via vite-plugin-pwa)

**PWA Configuration:**
`ui/web/vite.config.js` contains:
```javascript
VitePWA({
  manifest: {
    name: 'kinetistasis',
    short_name: 'kinetistasis',
    theme_color: '#10b981',
    background_color: '#111827',
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

## 🏗️ Frontend Architecture Decisions

### 1. Navigation Pattern ✓
**Choice:** Bottom tab bar (mobile) + top navbar (desktop)  
**Rationale:** Mobile-first UX, thumb-friendly placement, native app feel  
**Changed:** Replaced hamburger menu pattern in May 2026 (Commit `227dc7a`)

### 2. Routing ✓
**Choice:** No router library (Svelte Kit not used)  
**Rationale:** Small app (12 pages), easier debugging, no circular dependencies  
**Implementation:** State-based page switching in `App.svelte`

### 3. State Management ✓
**Choice:** Svelte 5 runes in centralized store (`ui/web/src/lib/stores.svelte.js`)  
**Rationale:** Reactive by default, no TypeScript needed, simpler than Pinia  
**Key Exports:** `setUser()`, `setCurrentPage()`, `setUnits()`, `clearUser()`

### 4. Styling System ✓
**Choice:** Tailwind CSS v4 via `@tailwindcss/vite`  
**Rationale:** Utility-first approach, no custom CSS files  
**Package:** `@tailwindcss/vite@^4.0.0`

### 5. Form Design ✓
**Choice:** API field names directly in forms (no mapping layer)  
**Rationale:** Eliminates JSON errors, frontend = API contract  
**Pattern:** Form pattern standardization (FIX-010)

### 6. Icon System ✓
**Choice:** Unicode emoji (no icon font library)  
**Rationale:** Reduced bundle size, no font dependencies, universal support  
**Trade-offs:** Limited customization (color via CSS only), platform rendering variance

### 7. Icon Branding ✓
**Choice:** Salad emoji (🥗) for app logo  
**Rationale:** Directly represents app purpose, memorable, works at all scales  
**Theme Color:** Emerald (#10b981) for PWA manifest

---

## 📊 Component Hierarchy

```
App.svelte (Root)
├── Auth State Check (onMount)
├── Nav.svelte (Desktop Header — hidden md:flex)
│   ├── Logo: 🥗 Diet Tracker
│   ├── Navigation Links
│   └── Logout Button
│
├── BottomNav.svelte (Mobile Footer — md:hidden)
│   ├── 6 Emoji Tab Buttons
│   ├── iOS Safe Area Support
│   └── Active state: emerald-400
│
└── Main Content Area (max-w-screen-xl)
    ├── Login Page
    ├── Dashboard + Widgets
    ├── LogMeal (Manual + AI)
    ├── CheckIn (Vitals)
    ├── WorkoutLog (AI/Simple/YAML tabs)
    ├── History (Nutrition 30-day)
    ├── WorkoutHistory (30-day)
    ├── Profile
    ├── BodyFat Calculator
    ├── Measurements
    ├── Targets
    └── Export (CSV/Markdown)

Shared Components:
├── Nav.svelte — Desktop navigation
├── BottomNav.svelte — Mobile navigation (NEW)
├── Card.svelte — Dark container
├── Spinner.svelte — Loading
├── Alert.svelte — Notifications
├── VitalsRing.svelte — Macro ring
├── WeightSparkline.svelte — 7-day trend
├── StreakGrid.svelte — Daily grid
├── AITab.svelte — AI parser
└── YAMLTab.svelte — YAML import/export
```

---

## 🔗 Related Obsidian Vault References

The following files in your Obsidian vault cross-reference this project:

**Path:** `~/.obsidian_vault/projects/diet-program/`

1. **mobile-web-roadmap.md** (Last Updated: 2026-04-28)
   - 4-phase PWA rollout plan
   - Phase 1-2: COMPLETE ✅
   - Phase 3-4: PLANNED 📋

2. **architecture/api-client-model.md** (Last Updated: 2026-04-25)
   - API server as single source of truth
   - Security: TLS & Token Authentication
   - Merge semantics for sync

3. **form-pattern-standardization.md** (Status: ARCHIVE)
   - Superseded by code implementation
   - Reference for FIX-010 form pattern

4. **INDEX.md** (Last Updated: 2026-04-28)
   - Master project index
   - Architecture overview

---

## 🚀 Build & Deployment

### Asset Generation
```bash
make gen-icons  # Generates icon-192.png and icon-512.png
```

**Dependencies:**
- ImageMagick (optional for emoji rendering)
- Fallback: Plain emerald background if emoji unavailable

### Build Process
```bash
cd ui/web
npm install
npm run build  # Outputs to dist/
```

**Plugins Used:**
- `@sveltejs/vite-plugin-svelte` — Svelte 5 support
- `@tailwindcss/vite` — Tailwind CSS v4
- `vite-plugin-pwa` — PWA manifest generation

### Service Worker Caching
```
Navigation: network-first
Assets (.js/.css): cache-first
API (/v1/*): bypass cache
Icons: cache-first (included in globPatterns)
```

---

## ✅ Design System Compliance

### WCAG 2.1 Accessibility
- ✅ Touch targets: 44×44 pixels minimum (iOS HIG)
- ✅ Color contrast: AAA for emerald, AA for gray
- ✅ Semantic HTML: `aria-label`, `aria-current="page"`
- ✅ Icon + text labels: No icon-only buttons

### Browser Support
- ✅ Chrome 90+
- ✅ Firefox 88+
- ✅ Safari 14+
- ✅ iOS Safari (PWA-capable)
- ✅ Android Chrome (PWA-capable)

---

## 🎯 Key Design Principles

1. **Mobile-First:** Responsive design starting from mobile
2. **Emoji-Native:** No external dependencies for icons
3. **Dark Theme:** Consistent emerald on dark gray
4. **Accessibility:** WCAG 2.1 compliant touch targets and contrast
5. **API-First:** Forms directly mirror API field names
6. **Single Responsibility:** Each component has one clear purpose
7. **No Router Library:** Simpler debugging, smaller bundle
8. **PWA-Ready:** Service worker caching, manifest, installable

---

## 📋 Documentation Status

### Fully Documented ✅
- Navigation patterns and responsive behavior
- Component tree and hierarchy
- Color system and theming
- Icon system (emoji + PWA)
- Form design patterns
- Authentication flow
- Data flow diagrams
- PWA caching strategy

### Partially Documented 🟡
- Icon selection rationale (implied from design)
- Brand identity reasoning (implicit)
- Alternative approaches considered (no historical record)

### Missing Documentation ⚠️
- Dark/light theme toggle planning
- Custom SVG icon migration path (mentioned as future enhancement)
- Cross-platform emoji rendering testing results

---

## 🔄 Recent Changes (May 2026)

| Date | Change | File | Status |
|------|--------|------|--------|
| May 4 | Bottom Tab Navigation | `BottomNav.svelte` | ✅ Complete |
| May 4 | iOS Safe Area Support | `BottomNav.svelte` | ✅ Complete |
| May 3 | UI Documentation Update | `UI_DESIGN_INDEX.md` | ✅ Complete |
| Apr 28 | Initial v2 Design | `UI_ARCHITECTURE.md` | ✅ Complete |

---

## 🚀 Future Enhancements (From Documentation)

### Phase 3 (Offline-First & Installation)
- Background sync queue
- Push notifications (check-in reminders)
- Enhanced install prompt
- Full offline operation with conflict resolution

### Additional UI/UX Features
- Dark/light theme toggle
- Customizable dashboard grid
- Advanced workout builder
- Social/sharing features
- Enhanced data export (PDF)

### Icon System Evolution
- Consider Font Awesome if emoji limitations arise
- Light theme support for icons
- Custom SVG icons for fine-tuned design
- Cross-platform emoji rendering verification
- Micro-interactions for tab switching

---

## 📁 File Inventory

### Design Documentation Files
- `UI_DESIGN_INDEX.md` — Master index
- `UI_ARCHITECTURE.md` — Component tree and flows
- `ICON_SYSTEM_DESIGN.md` — Complete icon system

### Source Components
- `ui/web/src/components/BottomNav.svelte` — Mobile nav
- `ui/web/src/components/Nav.svelte` — Desktop nav
- `ui/web/src/components/Card.svelte` — Dark container
- `ui/web/src/components/Spinner.svelte` — Loading animation
- `ui/web/src/components/Alert.svelte` — Notifications
- `ui/web/src/components/VitalsRing.svelte` — Macro progress
- `ui/web/src/components/WeightSparkline.svelte` — 7-day trend
- `ui/web/src/components/StreakGrid.svelte` — Daily checklist
- `ui/web/src/components/AITab.svelte` — AI parser
- `ui/web/src/components/YAMLTab.svelte` — YAML import

### Asset Files
- `ui/web/public/icon.svg` — PWA icon source
- `ui/web/public/icon-192.png` — Generated PWA icon
- `ui/web/public/icon-512.png` — Generated PWA icon

### Configuration Files
- `ui/web/vite.config.js` — PWA manifest & build config
- `ui/web/package.json` — Dependencies
- `Makefile` — Icon generation target

---

## 💡 Recommendations for Asset Management

Based on the vault analysis:

1. **No Changes Needed** — The icon system is well-designed and documented
2. **Consider Adding:**
   - Icon design rationale document (expand section 4 of ICON_SYSTEM_DESIGN.md)
   - Cross-platform emoji rendering testing results
   - Brand identity guidelines (color psychology, emoji selection process)

3. **Future Migrations:**
   - Document decision point for Font Awesome migration if emoji limitations are encountered
   - Plan for light theme icon variants before implementation
   - Create SVG fallbacks for emoji if rendering becomes problematic

4. **Maintenance:**
   - Run `make gen-icons` whenever updating the SVG source
   - Keep PWA manifest theme colors in sync with Tailwind config
   - Document any emoji changes in git commit messages

---

## 📞 Quick Reference

**Key Files to Remember:**
- Design System: `UI_DESIGN_INDEX.md`
- Architecture: `UI_ARCHITECTURE.md`
- Icons/Branding: `ICON_SYSTEM_DESIGN.md`

**Key Components:**
- Mobile Nav: `ui/web/src/components/BottomNav.svelte`
- Desktop Nav: `ui/web/src/components/Nav.svelte`
- PWA Config: `ui/web/vite.config.js`

**Build Commands:**
```bash
make gen-icons              # Generate PWA icons
npm run build              # Build PWA distribution
npm run dev                # Development server
```

**Asset Sources:**
- `ui/web/public/icon.svg` — Update this for new icons
- `ui/web/src/components/BottomNav.svelte` — Update emoji selection here
- `ui/web/vite.config.js` — Update manifest theme colors here

---

**Report Generated By:** Librarian (Knowledge Retrieval Agent)  
**Search Method:** Semantic hints + Code analysis + Documentation review  
**Verification:** All files located and verified on filesystem  
**Completeness:** Comprehensive coverage of design system and asset management ✓
