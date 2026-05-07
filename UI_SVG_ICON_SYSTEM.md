---
title: SVG Icon System
domain: diet-tracker
type: project-note
status: active
tags: [ui, icons, svg, svelte5, vite, implementation, design-system]
created: 2026-05-07
updated: 2026-05-07
---

# SVG Icon System

## Overview

The Diet Tracker (kinetistasis) implements a **hybrid dual-mode icon system** that combines:
- **Unicode emoji** for lightweight, accessible navigation UI (BottomNav, Nav components)
- **Optimized SVG icons** for high-quality branding, PWA installation, and future scalability

This system provides semantic, consistent icon usage across all components using a centralized mapping library. Icons can be switched between emoji and SVG modes without changing component code.

---

## Architecture

### Component Hierarchy

```
Icon System
├── Icon.svelte (Central entry point)
│   ├── Type: 'emoji' (default) | 'svg'
│   ├── Name: Semantic identifier (e.g., 'dashboard', 'logo')
│   ├── Size: 'sm' | 'md' (default) | 'lg' | 'xl'
│   └── Manages routing to SvgIcon.svelte or emoji span
│
├── SvgIcon.svelte (SVG renderer)
│   ├── Props: src, alt, size, color, className
│   ├── Renders: <img /> tag with SVG URL
│   └── Accessibility: aria-hidden support
│
└── lib/icons.js (Centralized icon library)
    ├── SVG_ICONS: Icon path mappings
    ├── getIconPath(name): Path resolver
    └── loadIcon(name): Dynamic import with ?url
```

### Directory Structure

```
ui/web/
├── src/
│   ├── components/
│   │   ├── Icon.svelte           [NEW - Central dispatcher]
│   │   ├── SvgIcon.svelte        [NEW - SVG renderer]
│   │   ├── Brand.svelte          [UPDATED - Uses SVG logo]
│   │   ├── Nav.svelte            [UPDATED - Uses Icon component]
│   │   ├── BottomNav.svelte      [UPDATED - Uses Icon component]
│   │   └── ...
│   ├── lib/
│   │   ├── icons.js              [NEW - Icon library]
│   │   ├── api.js
│   │   ├── stores.svelte.js
│   │   └── ...
│   └── assets/
│       └── icons/                [NEW - SVG asset directory]
│           ├── logo.svg          [App logo with brand text]
│           ├── logo-icon.svg     [Icon-only variant]
│           ├── dashboard.svg     [Dashboard icon]
│           ├── log-meal.svg      [Add meal icon]
│           ├── checkin.svg       [Check-in confirmation icon]
│           ├── history.svg       [History/analytics icon]
│           ├── workout.svg       [Exercise/workout icon]
│           └── profile.svg       [User profile icon]
```

---

## File Inventory

### New Files

| File | Location | Purpose | Status |
|------|----------|---------|--------|
| **Icon.svelte** | `ui/web/src/components/Icon.svelte` | Central icon dispatcher (emoji/SVG routing) | ✅ Active |
| **SvgIcon.svelte** | `ui/web/src/components/SvgIcon.svelte` | SVG image renderer component | ✅ Active |
| **icons.js** | `ui/web/src/lib/icons.js` | Icon library with path mappings | ✅ Active |
| **logo.svg** | `ui/web/src/assets/icons/logo.svg` | Full branding logo (with text) | ✅ Active |
| **logo-icon.svg** | `ui/web/src/assets/icons/logo-icon.svg` | Icon-only logo variant | ✅ Active |
| **dashboard.svg** | `ui/web/src/assets/icons/dashboard.svg` | Dashboard/home icon | ✅ Active |
| **log-meal.svg** | `ui/web/src/assets/icons/log-meal.svg` | Add/create meal icon | ✅ Active |
| **checkin.svg** | `ui/web/src/assets/icons/checkin.svg` | Confirmation/check-in icon | ✅ Active |
| **history.svg** | `ui/web/src/assets/icons/history.svg` | History/analytics icon | ✅ Active |
| **workout.svg** | `ui/web/src/assets/icons/workout.svg` | Exercise/strength icon | ✅ Active |
| **profile.svg** | `ui/web/src/assets/icons/profile.svg` | User profile icon | ✅ Active |

### Modified Files

| File | Location | Changes | Status |
|------|----------|---------|--------|
| **Brand.svelte** | `ui/web/src/components/Brand.svelte` | Added SVG logo support via `type="svg"` | ✅ Active |
| **Nav.svelte** | `ui/web/src/components/Nav.svelte` | Uses Icon component for profile icon | ✅ Active |
| **BottomNav.svelte** | `ui/web/src/components/BottomNav.svelte` | Uses Icon component (emoji mode) | ✅ Active |
| **vite.config.js** | `ui/web/vite.config.js` | PWA workbox: SVG included in caching | ✅ Active |

---

## Component APIs

### Icon.svelte

Central entry point for icon rendering with semantic naming.

```svelte
<script>
  import Icon from './Icon.svelte';
</script>

<!-- Emoji mode (default) -->
<Icon name="dashboard" type="emoji" size="md" />

<!-- SVG mode -->
<Icon name="logo" type="svg" size="lg" alt="Diet Tracker logo" />

<!-- With custom class -->
<Icon name="profile" type="emoji" size="sm" class="text-gray-400" />
```

**Props:**

| Prop | Type | Default | Options | Description |
|------|------|---------|---------|-------------|
| `name` | string | `''` | See **Icon Names** | Semantic icon identifier |
| `type` | string | `'emoji'` | `'emoji'`, `'svg'` | Rendering mode |
| `size` | string | `'md'` | `'sm'`, `'md'`, `'lg'`, `'xl'` | Icon size |
| `alt` | string | `''` | Any text | Alt text for SVG (accessibility) |
| `class` | string | `''` | Tailwind/CSS classes | Additional CSS classes |

**Icon Names** (available mappings):

| Name | Emoji | SVG | Use Case |
|------|-------|-----|----------|
| `logo` | 🥗 | Full branding logo | App header, branding |
| `home` / `dashboard` | 🏠 | Dashboard grid icon | Home/dashboard navigation |
| `log-meal` | ➕ | Plus/add icon | Add new meal entry |
| `checkin` | ✅ | Checkmark icon | Daily check-in, confirmation |
| `history` | 📊 | Bar chart icon | History, analytics view |
| `workout` | 🏋️ | Strength/exercise icon | Workout tracking |
| `profile` | 👤 | User avatar icon | User profile, account |

**Size Mapping:**

| Size | Emoji Class | SVG Size (px) |
|------|-------------|---------------|
| `sm` | `text-sm` | 16 |
| `md` | `text-base` | 24 |
| `lg` | `text-xl` | 28 |
| `xl` | `text-2xl` | 32 |

### SvgIcon.svelte

Low-level SVG image renderer (typically used internally by Icon.svelte).

```svelte
<script>
  import SvgIcon from './SvgIcon.svelte';
</script>

<SvgIcon 
  src="/assets/icons/dashboard.svg" 
  alt="Dashboard" 
  size="24" 
  color="currentColor"
/>
```

**Props:**

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `src` | string | Required | SVG file URL (Vite ?url import) |
| `alt` | string | `''` | Alt text for accessibility |
| `size` | string | `'24'` | Width/height in pixels |
| `color` | string | `'currentColor'` | SVG stroke/fill color |
| `class` | string | `''` | CSS classes |

---

## Usage Examples

### Basic Navigation Icons (Emoji Mode)

```svelte
<!-- BottomNav.svelte -->
<script>
  import Icon from './Icon.svelte';
</script>

<div class="tab">
  <Icon name="dashboard" type="emoji" size="lg" />
  <span>Dashboard</span>
</div>
```

### Brand Logo (SVG Mode)

```svelte
<!-- Brand.svelte -->
<script>
  import Icon from './Icon.svelte';
</script>

<!-- Full branding -->
<div class="brand">
  <Icon name="logo" type="svg" size="md" alt="Diet Tracker logo" />
  <span>kinetistasis</span>
</div>

<!-- Icon only -->
<Icon name="logo" type="svg" size="lg" alt="App logo" />
```

### Dual-Mode Component (Configurable)

```svelte
<!-- Using Icon with configurable type -->
<script>
  let useEmoji = true; // Toggle mode
</script>

<Icon 
  name="profile" 
  type={useEmoji ? 'emoji' : 'svg'} 
  size="md"
/>
```

---

## Technical Details

### Svelte 5 Runes Implementation

The Icon component uses Svelte 5 runes for reactivity:

```javascript
// $props() - Svelte 5 reactive props
let {
  name = '',
  type = 'emoji',
  size = 'md'
} = $props();

// $derived() - Computed properties
const sizeClass = $derived(sizeMap[size] ?? sizeMap.md);

// $effect() - Side effects (SVG loading)
$effect(() => {
  if (type === 'svg' && name) {
    updateSvg();
  } else {
    svgSrc = null;
  }
});
```

### SVG Import Strategy (Vite ?url)

SVG icons are imported as URLs using Vite's `?url` query parameter:

```javascript
// lib/icons.js
export async function loadIcon(name) {
  const path = SVG_ICONS[name];
  if (!path) return null;
  return await import(`../${path}?url`);
}
```

**How it works:**
1. Vite processes `?url` imports as URL strings (not ES modules)
2. SVG files are included in the build output
3. Dynamic imports allow lazy-loading icons on demand
4. URL references enable img src attributes and PWA caching

### PWA Caching Configuration

The `vite.config.js` includes SVG files in Workbox caching:

```javascript
workbox: {
  globPatterns: ['**/*.{js,css,html,ico,png,svg}']
}
```

This ensures:
- All SVG icons are cached by the PWA service worker
- Offline access to app icons
- Fast icon loading on subsequent visits
- Consistent experience in standalone/app mode

### SVG Specifications

All SVG files follow consistent standards:

```xml
<!-- Example: dashboard.svg -->
<svg 
  xmlns="http://www.w3.org/2000/svg" 
  viewBox="0 0 24 24" 
  fill="none" 
  stroke="currentColor" 
  stroke-width="2"
>
  <!-- Uses stroke for outline style -->
  <!-- viewBox enables scalability -->
  <!-- currentColor allows dynamic coloring -->
</svg>
```

**Standards:**
- `viewBox="0 0 24 24"` - Standard 24px grid (Material Design)
- `stroke="currentColor"` - Inherits color from parent (Tailwind compatible)
- `fill="none"` - Outline style (consistent with emoji style)
- `stroke-width="2"` - Readable at all sizes

---

## Migration Path

### Phase 1: Completed ✅
- ✅ Create Icon.svelte, SvgIcon.svelte components
- ✅ Create lib/icons.js library
- ✅ Add 8 SVG icon files to assets/icons/
- ✅ Update Brand.svelte to use SVG logo
- ✅ Update Nav.svelte and BottomNav.svelte to use Icon component

### Phase 2: Future (Optional)
- [ ] Migrate all navigation icons from emoji to SVG mode
- [ ] Add additional SVG icons as needed (settings, notifications, etc.)
- [ ] Create icon management dashboard for design team
- [ ] Implement icon animation states (hover, active, loading)
- [ ] Add icon prefetching strategy

### Phase 3: Long-term (Optional)
- [ ] Integrate with design system library (Figma tokens)
- [ ] Auto-generate SVG icons from design tool
- [ ] Create icon versioning system
- [ ] Implement dark mode icon variants

---

## Performance Considerations

### Bundle Size Impact

| Asset | Size | Impact |
|-------|------|--------|
| Icon.svelte | ~1.5 KB | Minimal (router component) |
| SvgIcon.svelte | ~0.5 KB | Minimal (renderer component) |
| icons.js | ~0.3 KB | Minimal (mappings) |
| All 8 SVGs | ~15 KB | Small (optimized, outline style) |
| **Total new assets** | **~17 KB** | **Negligible** |

### Optimization Strategies

1. **Lazy Loading**: SVG icons loaded only when requested (via `loadIcon()`)
2. **Compression**: Vite automatically minifies SVGs in production
3. **PWA Caching**: Service worker caches SVG assets for offline use
4. **Emoji Fallback**: Emoji mode incurs zero additional bytes

### Load Time Profile

- **Emoji mode**: 0ms additional load time (system font)
- **SVG mode (first load)**: ~50-100ms (dynamic import + fetch)
- **SVG mode (cached)**: <5ms (service worker cache hit)

---

## Accessibility

### Icon Accessibility Features

| Feature | Implementation | Example |
|---------|-----------------|---------|
| **Alt Text** | `alt` prop on SvgIcon | `alt="Dashboard"` |
| **Aria Hidden** | For decorative emojis | `aria-hidden="true"` |
| **Label Association** | Component context | Nav labels paired with icons |
| **Color Contrast** | Tailwind classes | `text-emerald-400` on `bg-gray-900` |
| **Size Flexibility** | `size` prop (4 options) | Supports zoom accessibility |

### Best Practices

```svelte
<!-- ✅ Good: Icon with descriptive text label -->
<button>
  <Icon name="profile" type="emoji" />
  <span>Profile</span>
</button>

<!-- ✅ Good: SVG with alt text -->
<Icon name="logo" type="svg" alt="Diet Tracker logo" />

<!-- ❌ Avoid: Decorative emoji without context -->
<span aria-hidden="true">🏠</span>
```

---

## Testing Strategy

### Component Unit Tests

```javascript
// Icon.svelte tests
test('renders emoji by default', () => {
  const { container } = render(Icon, { props: { name: 'dashboard' } });
  expect(container.textContent).toContain('🏠');
});

test('renders SVG when type="svg"', async () => {
  const { container } = render(Icon, { 
    props: { name: 'dashboard', type: 'svg' } 
  });
  // Wait for async SVG load
  await vi.waitFor(() => {
    expect(container.querySelector('img')).toBeTruthy();
  });
});

test('respects size prop', () => {
  const { container } = render(Icon, { 
    props: { name: 'dashboard', size: 'lg' } 
  });
  expect(container.querySelector('span')).toHaveClass('text-xl');
});
```

### Visual Regression Tests

- Compare emoji vs SVG rendering at different sizes
- Verify icon colors in dark mode
- Check PWA icon asset loading

---

## Related Documentation

- [[UI_ARCHITECTURE.md]] - Overall component structure and layout system
- [[ICON_SYSTEM_DESIGN.md]] - Historical design decisions and emoji selections
- [[UI_DESIGN_INDEX.md]] - UI component inventory
- [[DESIGN_SYSTEM_VAULT_FINDINGS.md]] - Design system research and standards

---

## Troubleshooting

### SVG Not Loading

**Issue:** SVG icon displays as broken image or placeholder

**Solutions:**
1. Verify file path in `lib/icons.js` matches actual file location
2. Check browser network tab for 404 errors
3. Ensure Vite server is running (dev mode)
4. Clear build cache: `npm run build` then test

### Icon Type Switching Not Working

**Issue:** Changing `type` prop doesn't update display

**Solutions:**
1. Verify `$effect()` hook is properly tracking dependencies
2. Check browser console for dynamic import errors
3. Ensure SVG file exists and is readable
4. Try full page refresh (may be service worker caching)

### Size Not Applying

**Issue:** Icon doesn't scale with `size` prop

**Solutions (emoji):** Check parent container width constraints
**Solutions (SVG):** Verify `svgSizeMap` in Icon.svelte is correct

### PWA Icon Not Showing

**Issue:** App icons don't appear in PWA install prompt

**Solutions:**
1. Verify `vite.config.js` manifest has correct icon paths
2. Ensure PNG icons exist: `/public/icon-192.png`, `/public/icon-512.png`
3. Test PWA in Chrome DevTools → Application → Manifest
4. Check service worker registration in console

---

## Future Enhancements

### Planned Features

1. **Icon Animation States**
   - Hover effects (scale, color change)
   - Loading spinners
   - State transitions (active → inactive)

2. **Dynamic Icon Library**
   - Icon management dashboard
   - Upload custom icons
   - Version control for icon changes

3. **Design Token Integration**
   - Figma token sync
   - Automated icon generation
   - Multi-variant support (light/dark)

4. **Performance Optimization**
   - SVG sprite sheets (advanced)
   - Icon prefetching strategy
   - Incremental PWA updates

---

## Maintenance

### Adding New Icons

1. Create new SVG file in `ui/web/src/assets/icons/`
2. Add entry to `SVG_ICONS` in `lib/icons.js`
3. Add emoji mapping to `iconMap` in `Icon.svelte` (if applicable)
4. Update **Icon Names** table in this document
5. Test in both emoji and SVG modes

### Updating Existing Icons

1. Edit SVG file directly
2. Clear Vite build cache: `npm run build`
3. Force browser cache clear (Ctrl+Shift+R)
4. Commit changes with descriptive message

### Deprecating Icons

1. Mark as `@deprecated` in comments
2. Update migration guide
3. Create new icon as replacement
4. Remove old icon after migration window

---

## Commit History

- **2026-05-07**: SVG icon system implementation completed
- **2026-05-04**: Brand.svelte updated to use SVG logo
- **2026-05-02**: Initial Icon component architecture

---

## Document Metadata

- **Last Updated:** 2026-05-07
- **Author:** Librarian (Documentation Agent)
- **Status:** Active
- **Version:** 1.0
- **Domain:** diet-tracker
- **Type:** project-note
