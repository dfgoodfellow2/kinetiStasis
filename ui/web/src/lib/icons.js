// Icon mappings and exports for the Diet Tracker app
// Use this to centrally manage icon imports.
//
// These are static `?url` imports on purpose: a dynamic
// `import(\`../${path}?url\`)` cannot be resolved by Vite at build time and
// silently fails in production, leaving brand icons unrendered.

import logoUrl from '../assets/icons/logo.svg?url';
import logoIconUrl from '../assets/icons/logo-icon.svg?url';
import dashboardUrl from '../assets/icons/dashboard.svg?url';
import logMealUrl from '../assets/icons/log-meal.svg?url';
import checkinUrl from '../assets/icons/checkin.svg?url';
import historyUrl from '../assets/icons/history.svg?url';
import workoutUrl from '../assets/icons/workout.svg?url';
import profileUrl from '../assets/icons/profile.svg?url';

// SVG Icon URLs (resolved and hashed by Vite)
export const SVG_ICONS = {
  logo: logoUrl,
  'logo-icon': logoIconUrl,
  dashboard: dashboardUrl,
  'log-meal': logMealUrl,
  checkin: checkinUrl,
  history: historyUrl,
  workout: workoutUrl,
  profile: profileUrl,
};

// Helper to get icon path
export function getIconPath(name) {
  return SVG_ICONS[name] || null;
}

// Kept as an async module-shaped helper so Icon.svelte can treat every icon
// source the same way (it reads `mod.default`).
export async function loadIcon(name) {
  const url = SVG_ICONS[name];
  return url ? { default: url } : null;
}
