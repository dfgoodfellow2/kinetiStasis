<script>
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { kgToLbs, lbsToKg, inputLoad } from '../lib/utils.js'

  let { workout = $bindable(), onParsed = () => {} } = $props()

  let yamlRaw = $state('')
  let yamlParsed = $state(false)
  let yamlParsing = $state(false)
  let error = $state('')

  async function parseYAML() {
    if (!yamlRaw.trim()) return
    yamlParsing = true
    error = ''
    try {
      const res = await api.parseWorkout(yamlRaw.trim(), 'yaml')
      if (res && !res.error) {
        applyParsed(res)
        yamlParsed = true
      } else {
        error = 'YAML parse failed: ' + (res?.error ?? 'unknown error')
      }
    } catch (e) {
      error = 'YAML parse error: ' + e.message
    } finally {
      yamlParsing = false
    }
  }

  // ── Per-set overrides ───────────────────────────────────────
  // Accepted YAML shape (inline arrays inside an exercise):
  //
  //   exercises:
  //     - name: Back Squat
  //       sets: 3
  //       reps: 10                  # flat default for uncovered sets
  //       load: 100kg               # flat default load
  //       set_reps: [10, 8, 6]
  //       set_loads: [100kg, 105kg, 110kg]
  //
  // Bare numbers are display units (kg metric, lbs imperial); a `kg`/`lb(s)`
  // suffix overrides that per value. Lists merge over the server's flat
  // sets×reps×load defaults; a longer list grows the set count.
  // Inline arrays are required: the server's line parser reads a block
  // sequence ("- 10") as the start of a new exercise.
  function indentOf(line) {
    let n = 0
    for (const ch of line) {
      if (ch === ' ') n++
      else if (ch === '\t') n += 2
      else break
    }
    return n
  }

  function unquote(s) {
    const t = String(s ?? '').trim()
    if (t.length >= 2 &&
        ((t[0] === '"' && t[t.length - 1] === '"') ||
         (t[0] === "'" && t[t.length - 1] === "'"))) {
      return t.slice(1, -1)
    }
    return t
  }

  function splitKV(line) {
    const i = line.indexOf(':')
    if (i < 0) return { key: line.trim(), value: '' }
    return { key: line.slice(0, i).trim(), value: unquote(line.slice(i + 1).trim()) }
  }

  // "[10, 8, 6]" / "[100kg, 105kg]" / "10, 8, 6" → ["10", "8", "6"].
  function parseInlineList(value) {
    let s = String(value ?? '').trim()
    if (s.startsWith('[')) s = s.slice(1)
    if (s.endsWith(']')) s = s.slice(0, -1)
    return s.split(',').map(p => unquote(p.trim())).filter(p => p !== '')
  }

  function parseRepValue(raw) {
    if (raw === null || raw === undefined || String(raw).trim() === '') return null
    const m = String(raw).match(/-?\d+(?:\.\d+)?/)
    if (!m) return null
    return Math.max(0, Math.round(Number(m[0])))
  }

  // Returns canonical kg, 0 for an explicit bodyweight/zero load, or null
  // when the entry is absent/invalid (leave the server's set untouched).
  function parseSetLoad(raw, units) {
    const s = String(raw ?? '').trim()
    if (s === '') return null
    if (/^(bw|bodyweight)$/i.test(s)) return 0

    let unit = units
    let num = s
    if (/(?:kgs?|kilograms?)$/i.test(s)) {
      unit = 'metric'
      num = s.replace(/(?:kgs?|kilograms?)$/i, '').trim()
    } else if (/(?:lbs?|pounds?)$/i.test(s)) {
      unit = 'imperial'
      num = s.replace(/(?:lbs?|pounds?)$/i, '').trim()
    }

    // Sum unilateral notation ("35+35 lbs") the same way the server does.
    let found = false
    const total = num.split('+').reduce((acc, part) => {
      const n = parseFloat(part)
      if (!Number.isFinite(n)) return acc
      found = true
      return acc + n
    }, 0)
    if (!found) return null
    return inputLoad(total, unit)
  }

  function applyOverrideField(cur, text) {
    const { key, value } = splitKV(text)
    if (key === 'set_reps' || key === 'setReps') {
      cur.setReps = parseInlineList(value).map(parseRepValue)
    } else if (key === 'set_loads' || key === 'setLoads') {
      cur.setLoads = parseInlineList(value).map(v => parseSetLoad(v, store.units))
    }
  }

  // One entry per exercise, in source order, so it can be zipped with the
  // parsed exercises the server returns.
  function parseSetOverrides(text) {
    const out = []
    let inExercises = false
    let cur = null

    for (const rawLine of String(text ?? '').split(/\r?\n/)) {
      let line = rawLine
      const hash = line.indexOf(' #')
      if (hash >= 0) line = line.slice(0, hash)
      const trimmed = line.trim()
      if (!trimmed || trimmed === '---' || trimmed === '...') continue
      const indent = indentOf(rawLine)

      if (indent === 0) {
        inExercises = trimmed === 'exercises:'
        cur = null
        continue
      }
      if (!inExercises) continue

      if (trimmed.startsWith('- ')) {
        cur = { setReps: [], setLoads: [] }
        out.push(cur)
        const rest = trimmed.slice(2).trim()
        if (rest) applyOverrideField(cur, rest)
      } else if (cur) {
        applyOverrideField(cur, trimmed)
      }
    }
    return out
  }

  // Merge overrides onto the server's sets, keeping one row per set. This is
  // the same `_sets` shape AITab emits, so WorkoutLog's editor and save path
  // treat both parse tabs identically.
  function buildParsedSets(setsArr, override) {
    const base = Array.isArray(setsArr) ? setsArr : []
    const repOverrides = Array.isArray(override?.setReps) ? override.setReps : []
    const loadOverrides = Array.isArray(override?.setLoads) ? override.setLoads : []
    const count = Math.max(base.length, repOverrides.length, loadOverrides.length)
    if (count === 0) return []

    const rows = []
    for (let i = 0; i < count; i++) {
      const src = base[Math.min(i, base.length - 1)] || {}
      const baseReps = Number(src.reps) || 0
      const srcLbs = Number(src.loadLbs) || 0
      // Recover a pounds-only row (kg mirror zero) instead of losing its load.
      const baseKg = Number(src.loadKg) || (srcLbs > 0 ? lbsToKg(srcLbs) : 0)
      const rep = repOverrides[i]
      const load = loadOverrides[i]
      const reps = rep === null || rep === undefined ? baseReps : rep
      const loadKg = load === null || load === undefined ? baseKg : load
      rows.push({
        reps,
        loadKg,
        loadLbs: loadKg > 0 ? kgToLbs(loadKg) : 0,
        tutSeconds: Number(src.tutSeconds) || 0,
        restSeconds: Number(src.restSeconds) || 0,
      })
    }
    return rows
  }

  function applyParsed(res) {
    let focusStr = ''
    if (Array.isArray(res.focus) && res.focus.length > 0) {
      focusStr = res.focus.join(', ')
    } else if (typeof res.focus === 'string' && res.focus) {
      try {
        const parsed = JSON.parse(res.focus)
        focusStr = Array.isArray(parsed) ? parsed.join(', ') : res.focus
      } catch {
        focusStr = res.focus
      }
    }

    // The server ignores set_reps/set_loads (it builds sets from the flat
    // fields), so read them from the raw text and merge by exercise order.
    const overrides = parseSetOverrides(yamlRaw)

    workout = {
      ...workout,
      date:            res.date           || workout.date,
      slot:            res.slot           ? String(res.slot) : workout.slot,
      title:           res.title          || workout.title,
      type:            res.type           || workout.type,
      style:           res.style          || workout.style,
      surface:         res.surface        || workout.surface,
      focus:           focusStr           || workout.focus,
      restInterval:   res.restInterval  || workout.restInterval,
      durationMin:    res.durationMin   || workout.durationMin,
      avgHr:          res.avgHr         || workout.avgHr,
      maxHr:          res.maxHr         || workout.maxHr,
      caloriesBurned: res.caloriesBurned || workout.caloriesBurned,
      notes:           res.notes || res.rawNotes || workout.notes,
      coachNotes:    res.coachNotes   || workout.coachNotes,
      // Session metadata that has no inline input but must survive a save.
      // ParsedWorkout always emits rpe/day (0 when unknown), so only a real
      // value may override metadata restored from an existing row.
      rpe:             res.rpe      || workout.rpe,
      recovers:        res.recovers || workout.recovers,
      day:             res.day      || workout.day,
      exercises:       (res.exercises || []).map((e, i) => mapExercise(e, overrides[i])),
    }

    onParsed()
  }

  function mapExercise(e, override) {
    const setsArr = Array.isArray(e.sets) ? e.sets : []
    const rows = buildParsedSets(setsArr, override)
    const firstSet = rows[0] ?? setsArr[0] ?? {}

    // Never fabricate a load. A "70 kg" style fallback would be re-parsed as
    // pounds by WorkoutLog's parseLoad and then converted to kg on save,
    // corrupting every parsed exercise that carried set.loadKg only.
    const weightDisplay = e.loadRaw || ''

    return {
      name:        e.name        || '',
      sets:        rows.length ? String(rows.length) : '',
      reps:        firstSet.reps ?? '',
      duration:    e.durationRaw || '',
      weightLbs:  weightDisplay,
      tempo:       e.tempo        || '',
      rpe:         e.rpe          ?? '',
      pattern:     e.category     || '',
      bias:        e.bias         || '',
      distanceKm:  e.distanceKm   ?? '',
      elevationM:  e.elevationM   ?? '',
      surface:     e.surface      || '',
      pace:        e.pace         || '',
      met:         e.metValue ?? '',
      notes:       e.notes        || '',
      phase:       e.phase        || '',
      groupId:     e.groupId      || '',
      groupOrder:  e.groupOrder   ?? '',
      groupStyle:  e.groupStyle   || '',
      targetHrZone: e.targetHrZone ?? '',
      hrZoneTime:  e.hrZoneTime   || null,
      _sets:       rows,
    }
  }
</script>

<div>
  <h3 class="text-lg font-semibold mb-3">YAML Parse</h3>
  {#if error}
    <div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
      {error}
    </div>
  {/if}
  <textarea
    bind:value={yamlRaw}
    class="input font-mono text-sm mb-3"
    rows="6"
    placeholder="Enter workout YAML (e.g., Squat: 3x5 @ 100kg...)"
  ></textarea>
  <button
    onclick={parseYAML}
    disabled={yamlParsing || !yamlRaw.trim()}
    class="btn-primary"
  >
    {yamlParsing ? 'Parsing...' : 'Parse YAML'}
  </button>
  {#if yamlParsed}
    <div class="mt-2 text-sm text-green-600">✓ YAML parse successful</div>
  {/if}
</div>
