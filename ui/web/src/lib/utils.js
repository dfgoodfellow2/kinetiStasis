export function today() {
  return new Date().toLocaleDateString('sv')
}

export function daysAgo(n) {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().slice(0, 10)
}

export function progressBar(current, max, width = 20) {
  const pct = Math.min(current / (max || 1), 1)
  const filled = Math.round(pct * width)
  return '█'.repeat(filled) + '░'.repeat(width - filled)
}

export function fmt1(n) {
  return typeof n === 'number' ? n.toFixed(1) : '0.0'
}

export function fmt0(n) {
  return typeof n === 'number' ? Math.round(n).toString() : '0'
}

export async function copyToClipboard(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

// ─── Unit conversion helpers ──────────────────────────────────────────────────
// Backend stores: weight=kg, measurements=cm, distance=km, elevation=m, load=kg
// Display converts based on profile.units ('metric' or 'imperial')

// Raw conversions
export const kgToLbs  = kg  => +(kg  * 2.20462).toFixed(1)
export const lbsToKg  = lbs => +(lbs / 2.20462).toFixed(2)
export const cmToIn   = cm  => +(cm  / 2.54).toFixed(1)
export const inToCm   = i   => +(i   * 2.54).toFixed(1)
export const kmToMi   = km  => +(km  / 1.60934).toFixed(2)
export const miToKm   = mi  => +(mi  * 1.60934).toFixed(2)
export const mToFt    = m   => +(m   / 0.3048).toFixed(0)
export const ftToM    = ft  => +(ft  * 0.3048).toFixed(1)

// Display helpers (backend value → display value, number)
// units param = store.units ('metric' | 'imperial')
export const dispWeight  = (kg,  units) => {
    if (kg === undefined || kg === null || !isFinite(kg)) return '—'
    return units === 'imperial' ? kgToLbs(kg)  : +kg.toFixed(1)
}
export const dispLength  = (cm,  units) => {
    if (cm === undefined || cm === null || !isFinite(cm)) return '—'
    return units === 'imperial' ? cmToIn(cm)   : +cm.toFixed(1)
}
export const dispDist    = (km,  units) => {
    if (km === undefined || km === null || !isFinite(km)) return '—'
    return units === 'imperial' ? kmToMi(km)   : +km.toFixed(2)
}
export const dispElev    = (m,   units) => {
    if (m === undefined || m === null || !isFinite(m)) return '—'
    return units === 'imperial' ? mToFt(m)     : +m.toFixed(0)
}
export const dispLoad    = (kg,  units) => {
    if (kg === undefined || kg === null || !isFinite(kg)) return '—'
    return units === 'imperial' ? kgToLbs(kg)  : +kg.toFixed(1)
}

// Unit label helpers
export const weightUnit  = units => units === 'imperial' ? 'lbs' : 'kg'
export const lengthUnit  = units => units === 'imperial' ? 'in'  : 'cm'
export const distUnit    = units => units === 'imperial' ? 'mi'  : 'km'
export const elevUnit    = units => units === 'imperial' ? 'ft'  : 'm'
export const loadUnit    = units => units === 'imperial' ? 'lbs' : 'kg'

// Input → backend: convert user-entered display value back to metric for storage
export const inputWeight = (val, units) => units === 'imperial' ? lbsToKg(+val)  : +val  // store as kg
export const inputLength = (val, units) => units === 'imperial' ? inToCm(+val)   : +val  // store as cm
export const inputDist   = (val, units) => units === 'imperial' ? miToKm(+val)   : +val  // store as km
export const inputElev   = (val, units) => units === 'imperial' ? ftToM(+val)    : +val  // store as m
export const inputLoad   = (val, units) => units === 'imperial' ? lbsToKg(+val)  : +val  // store as kg

// Height helpers: backend stores height_cm always in cm regardless of units setting
// For imperial display, show as ft + in string e.g. "5'10\""
export function dispHeightCm(cm, units) {
  if (units !== 'imperial') return +cm.toFixed(1)
  const totalIn = cm / 2.54
  const ft = Math.floor(totalIn / 12)
  const inch = Math.round(totalIn % 12)
  return `${ft}'${inch}"`
}
// For imperial input: user enters feet + inches separately; combine to cm
export function heightFtInToCm(ft, inch) {
  return +((+ft * 12 + +inch) * 2.54).toFixed(1)
}
