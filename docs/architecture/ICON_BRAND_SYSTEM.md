---
title: Icon & Brand System Architecture
date: 2026-05-04
domain: frontend
status: active
tags: [architecture, components, icons, brand, svelte, kinetistasis]
---

# Icon & Brand System Architecture

## Overview
Implementation of centralized Icon and Brand components for the kinetistasis app (formerly Diet Tracker).

## Components

### Icon.svelte
**Location:** `ui/web/src/components/Icon.svelte`
**Purpose:** Central icon system with semantic naming

**API:**
- `name`: 'logo' | 'home' | 'log-meal' | 'checkin' | 'history' | 'workout' | 'profile'
- `size`: 'sm' | 'md' | 'lg' (default: 'md')

**Icon Mapping:**
| Name | Emoji | Usage |
|------|-------|-------|
| logo | 🥗 | App logo/branding |
| home | 🏠 | Dashboard |
| log-meal | ➕ | Log Meal |
| checkin | ✅ | Check-In |
| history | 📊 | History |
| workout | 🏋️ | Workout |
| profile | 👤 | Profile |

### Brand.svelte
**Location:** `ui/web/src/components/Brand.svelte`
**Purpose:** Centralized brand identity for kinetistasis

**API:**
- `variant`: 'full' | 'icon' | 'text' (default: 'full')
- `size`: 'sm' | 'md' | 'lg' (default: 'md')

**Variants:**
- `full`: 🥗 kinetistasis (icon + text)
- `icon`: 🥗 (icon only)
- `text`: kinetistasis (text only)

## Rebrand Summary

**From:** Diet Tracker
**To:** kinetistasis

**Files Updated:**
1. `ui/web/src/components/Nav.svelte` - Uses `<Brand />` and `<Icon name="profile" />`
2. `ui/web/src/components/BottomNav.svelte` - All 6 nav icons use `<Icon />`
3. `ui/web/index.html` - Title: "kinetistasis"
4. `ui/web/vite.config.js` - PWA manifest name: "kinetistasis"

## Architecture Decisions

### Why Two Components?
- **Separation of Concerns:** Brand identity vs icon system
- **Single Responsibility:** Each component has one reason to change
- **Reusability:** Icon.svelte can be used independently of brand

### Why Semantic Names?
- **Maintainability:** Change emoji in one place (Icon.svelte)
- **Clarity:** `<Icon name="home" />` is clearer than `🏠`
- **Future-Proof:** Easy to swap emoji for SVG icons later

### Why "kinetistasis"?
- User's chosen brand name
- Replaces generic "Diet Tracker" with unique identity

## Usage Examples

```svelte
<!-- Full brand in header -->
<Brand />

<!-- Icon only -->
<Brand variant="icon" size="lg" />

<!-- Navigation icons -->
<Icon name="home" />
<Icon name="log-meal" size="lg" />

<!-- In components -->
<button>
  <Icon name="checkin" />
  <span>Check-In</span>
</button>
```

## Migration Notes

**Date:** 2026-05-04
**Commit:** (pending - not yet committed)

**Changes Made:**
1. Created Icon.svelte with emoji-to-name mapping
2. Created Brand.svelte with variant system
3. Updated Nav.svelte to use both components
4. Updated BottomNav.svelte to use Icon.svelte
5. Updated index.html title
6. Updated PWA manifest in vite.config.js

**No Breaking Changes:** All existing functionality preserved.

## Future Enhancements

1. **SVG Icons:** Replace emoji with custom SVG icons
2. **Icon Categories:** Add outline/filled variants
3. **Animation:** Add loading/spin states for icons
4. **Brand Evolution:** Update logo from 🥗 to custom graphic

## Related Files
- `ui/web/src/components/Icon.svelte`
- `ui/web/src/components/Brand.svelte`
- `ui/web/src/components/Nav.svelte`
- `ui/web/src/components/BottomNav.svelte`
- `ui/web/vite.config.js`
- `ui/web/index.html`
