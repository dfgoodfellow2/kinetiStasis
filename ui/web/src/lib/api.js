import { today } from './utils.js'

const BASE = '/v1'

/**
 * API response shapes.
 *
 * The client is plain JS, so these JSDoc typedefs are the single source of
 * truth for consumers (Dashboard/CheckIn/Targets components). Keep them in
 * sync with internal/models/domain.go.
 *
 * @typedef {Object} DashboardData
 * @property {Object} today              TodaySummary { consumed, targets, ... }
 * @property {Object} tdee               TDEEResult
 * @property {Object} macros             MacroResult (includes goalLabel, e.g. "cut_20")
 * @property {Object} readiness          ReadinessResult
 * @property {Object} weeklyStats        WeeklyStats (legacy dailyLogged/currentStreak)
 * @property {Array<{date: string, weightKg: number}>} weightTrend Raw weigh-ins, oldest→newest
 * @property {number} weightTrendKg        EMA trend weight in kg (0 = insufficient data)
 * @property {number} weightRateKgPerWeek  Weekly rate of change in kg (negative = losing)
 * @property {number} trendDays            Days of data backing the trend
 * @property {number} loggingStreak        Consecutive logged days ending today
 * @property {number} longestStreak        Longest logging streak in the window
 * @property {boolean[]} loggedDays        Calendar window, oldest→newest, last = today (60 entries)
 *
 * @typedef {Object} CheckinPreview
 * @property {number} observedTdee
 * @property {string} tdeeConfidence       New field; older servers call this `confidence`
 * @property {number} lookbackDays
 * @property {string} confidence           Legacy alias of tdeeConfidence
 * @property {number} recommendedCalories
 *
 * @typedef {Object} CheckinHistoryEntry
 * @property {string} checkInDate
 * @property {number} observedTdee
 * @property {string} tdeeConfidence
 * @property {number} lookbackDays
 * @property {number} resultingCalories
 *
 * @typedef {Object} TargetSnapshot
 * @property {string} effectiveDate
 * @property {number} calories
 * @property {number} proteinG
 * @property {number} carbsG
 * @property {number} fatG
 * @property {number} fiberG
 * @property {number} waterMl
 * @property {boolean} eatBackExercise
 */

async function req(method, path, body, extraHeaders = {}) {
  const opts = {
    method,
    credentials: 'include',
    headers: { ...extraHeaders }
  }
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(BASE + path, opts)
  if (res.status === 401) {
    // Try refresh
    const ref = await fetch(BASE + '/auth/refresh', { method: 'POST', credentials: 'include' })
    if (!ref.ok) throw new Error('unauthorized')
    // Retry original
    const res2 = await fetch(BASE + path, opts)
    if (!res2.ok) {
      const err = await res2.json().catch(() => ({}))
      throw new Error(err.error || `HTTP ${res2.status}`)
    }
    return res2.status === 204 ? null : toCamel(await res2.json().catch(() => ({})))
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  if (res.status === 204) return null
  const json = await res.json().catch(() => ({}))
  return toCamel(json)
}

function toCamel(v) {
  if (v === null || v === undefined) return v
  if (Array.isArray(v)) return v.map(toCamel)
  if (typeof v === 'object') {
    const out = {}
    for (const k of Object.keys(v)) {
      const val = v[k]
      const camel = k.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
      out[camel] = toCamel(val)
    }
    return out
  }
  return v
}

export const api = {
  // Auth
  login: (username, password) => req('POST', '/auth/login', { login: username, password }),
  register: (username, email, password) => req('POST', '/auth/register', { username, email, password }),
  logout: () => req('POST', '/auth/logout'),
  me: () => req('GET', '/auth/me'),

  // Profile
  getProfile: () => req('GET', '/profile'),
  updateProfile: (p) => req('PUT', '/profile', p),

  // Dashboard — returns DashboardData (see typedef above)
  getDashboard: () => req('GET', `/dashboard?date=${today()}`),

  // Nutrition
  listNutrition: (from, to) => req('GET', `/nutrition/logs?from=${from}&to=${to}`),
  getNutrition: (date) => req('GET', `/nutrition/logs/${date}`),
  postNutrition: (log) => req('POST', '/nutrition/logs', log),
  putNutrition: (date, log) => req('PUT', `/nutrition/logs/${date}`, log),
  deleteNutrition: (date) => req('DELETE', `/nutrition/logs/${date}`),

  // Biometrics
  listBiometrics: (from, to) => req('GET', `/biometrics?from=${from}&to=${to}`),
  postBiometric: (b) => {
    // Backend now uses camelCase consistently — send payload as-is
    return req('POST', '/biometrics', b)
  },
  putBiometric: (date, b) => {
    // Backend now uses camelCase consistently — send payload as-is
    return req('PUT', `/biometrics/${date}`, b)
  },
  deleteBiometric: (date) => req('DELETE', `/biometrics/${date}`),

  // Workouts
  listWorkouts: (from, to) => req('GET', `/workouts?from=${from}&to=${to}`),
  postWorkout: (w) => req('POST', '/workouts', w),
  deleteWorkout: (date, slot) => req('DELETE', `/workouts/${date}/${slot}`),

  // Targets
  getTargets: () => req('GET', '/targets'),
  putTargets: (t) => req('PUT', '/targets', t),
  // Newest-first TargetSnapshot[]; each snapshot holds the *previous* values
  // at the moment of the change.
  listTargetHistory: () => req('GET', '/targets/history'),

  // Saved meals & templates
  listSaved: () => req('GET', '/meals/saved'),
  postSaved: (m) => req('POST', '/meals/saved', m),
  deleteSaved: (id) => req('DELETE', `/meals/saved/${id}`),
  listTemplates: () => req('GET', '/meals/templates'),

  // Measurements
  listMeasurements: (from, to) => req('GET', `/measurements?from=${from}&to=${to}`),
  postMeasurement: (m) => req('POST', '/measurements', m),
  putMeasurement: (date, m) => req('PUT', `/measurements/${date}`, m),
  deleteMeasurement: (date) => req('DELETE', `/measurements/${date}`),

  // Calculations
  getTDEE: (days) => req('GET', `/calc/tdee?days=${days}`),
  getMacros: () => req('GET', '/calc/macros'),
  getReadiness: () => req('GET', '/calc/readiness'),
  getBodyFat: (method) => req('GET', `/calc/bodyfat?method=${method}`),

  // Parse (AI)
  // Back-compat: parseMeal('2 eggs') still maps to { text: '2 eggs' }.
  // New form: parseMeal({ text, imageBase64, mimeType }) where imageBase64 is
  // raw base64 (no "data:" prefix) and mimeType is image/jpeg|png|webp.
  parseMeal: (input) => {
    const opts = typeof input === 'string' ? { text: input } : (input || {})
    const body = {}
    if (opts.text !== undefined && opts.text !== null) body.text = opts.text
    if (opts.imageBase64) {
      body.imageBase64 = opts.imageBase64
      body.mimeType = opts.mimeType || 'image/jpeg'
    }
    return req('POST', '/parse/meal', body)
  },
  parseWorkout: (text, format = 'ai') => req('POST', '/parse/workout', { text, format }),

  // Export — returns { content: "..." }
  exportNutrition: (from, to, format) => req('GET', `/export/nutrition?from=${from}&to=${to}&format=${format}`, undefined, { Accept: 'application/json' }),
  exportWorkouts: (from, to, format) => req('GET', `/export/workouts?from=${from}&to=${to}&format=${format}`, undefined, { Accept: 'application/json' }),
  exportCombined: (from, to) => req('GET', `/export/combined?from=${from}&to=${to}`, undefined, { Accept: 'application/json' }),

  // Weekly Check-in
  // Returns CheckinPreview (see typedef above). tdeeConfidence may arrive as
  // the legacy `confidence` field, so read both with `?.` fallbacks.
  getCheckinPreview: () => req('GET', '/checkin'),
  postCheckin: (data) => req('POST', '/checkin', data),
  // Returns CheckinHistoryEntry[] (order not guaranteed; sort by checkInDate).
  listCheckinHistory: () => req('GET', '/checkin/history'),
  // Macro distribution
  distributeMacros: (calories) => req('POST', '/calc/macros/distribute', { calories }),
}
