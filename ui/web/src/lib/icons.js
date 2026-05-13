// Icon mappings and exports for the Diet Tracker app
// Use this to centrally manage icon imports

// SVG Icon paths (relative to src/)
export const SVG_ICONS = {
  logo: 'assets/icons/logo.svg',
  'logo-icon': 'assets/icons/logo-icon.svg',
  dashboard: 'assets/icons/dashboard.svg',
  'log-meal': 'assets/icons/log-meal.svg',
  checkin: 'assets/icons/checkin.svg',
  history: 'assets/icons/history.svg',
  workout: 'assets/icons/workout.svg',
  profile: 'assets/icons/profile.svg',
};

// Helper to get icon path
export function getIconPath(name) {
  return SVG_ICONS[name] || null;
}

// For dynamic imports (if needed)
export async function loadIcon(name) {
  const path = SVG_ICONS[name];
  if (!path) return null;
  return await import(`../${path}?url`);
}
