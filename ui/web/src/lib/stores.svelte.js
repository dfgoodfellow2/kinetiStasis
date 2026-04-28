export const store = $state({ user: null, currentPage: 'login', units: 'metric', sleepQualityMax: 10 })

export function setUser(u) { store.user = u }
export function clearUser() { store.user = null }
export function setCurrentPage(p) { store.currentPage = p }
export function setUnits(u) { store.units = u || 'metric' }
export function setSleepQualityMax(v) { store.sleepQualityMax = (v && v > 0) ? v : 10 }
