const BASE = '/v1'

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
    return res2.status === 204 ? null : res2.json()
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  return res.status === 204 ? null : res.json()
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

  // Dashboard
  getDashboard: () => req('GET', `/dashboard?date=${new Date().toLocaleDateString('sv')}`),

  // Nutrition
  listNutrition: (from, to) => req('GET', `/nutrition/logs?from=${from}&to=${to}`),
  getNutrition: (date) => req('GET', `/nutrition/logs/${date}`),
  postNutrition: (log) => req('POST', '/nutrition/logs', log),
  putNutrition: (date, log) => req('PUT', `/nutrition/logs/${date}`, log),
  deleteNutrition: (date) => req('DELETE', `/nutrition/logs/${date}`),

  // Biometrics
  listBiometrics: (from, to) => req('GET', `/biometrics?from=${from}&to=${to}`),
  postBiometric: (b) => {
    // include optional body_fat_pct if present on the object
    const payload = Object.assign({}, b)
    if (payload.body_fat_pct === undefined && payload.bodyFatPct !== undefined) {
      // accept camelCase from some callers
      payload.body_fat_pct = payload.bodyFatPct
      delete payload.bodyFatPct
    }
    return req('POST', '/biometrics', payload)
  },
  putBiometric: (date, b) => {
    const payload = Object.assign({}, b)
    if (payload.body_fat_pct === undefined && payload.bodyFatPct !== undefined) {
      payload.body_fat_pct = payload.bodyFatPct
      delete payload.bodyFatPct
    }
    return req('PUT', `/biometrics/${date}`, payload)
  },
  deleteBiometric: (date) => req('DELETE', `/biometrics/${date}`),

  // Workouts
  listWorkouts: (from, to) => req('GET', `/workouts?from=${from}&to=${to}`),
  postWorkout: (w) => req('POST', '/workouts', w),
  deleteWorkout: (date, slot) => req('DELETE', `/workouts/${date}/${slot}`),

  // Targets
  getTargets: () => req('GET', '/targets'),
  putTargets: (t) => req('PUT', '/targets', t),

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
  parseMeal: (text) => req('POST', '/parse/meal', { text }),
  parseWorkout: (text, format = 'ai') => req('POST', '/parse/workout', { text, format }),

  // Export — returns { content: "..." }
  exportNutrition: (from, to, format) => req('GET', `/export/nutrition?from=${from}&to=${to}&format=${format}`, undefined, { Accept: 'application/json' }),
  exportWorkouts: (from, to, format) => req('GET', `/export/workouts?from=${from}&to=${to}&format=${format}`, undefined, { Accept: 'application/json' }),
  exportCombined: (from, to) => req('GET', `/export/combined?from=${from}&to=${to}`, undefined, { Accept: 'application/json' }),
}
