<script>
  import { api } from '../lib/api.js'

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

    workout = {
      ...workout,
      date:            res.date           || workout.date,
      slot:            res.slot           ? String(res.slot) : workout.slot,
      title:           res.title          || workout.title,
      type:            res.type           || workout.type,
      style:           res.style          || workout.style,
      surface:         res.surface        || workout.surface,
      focus:           focusStr           || workout.focus,
      rest_interval:   res.rest_interval  || workout.rest_interval,
      duration_min:    res.duration_min   || workout.duration_min,
      avg_hr:          res.avg_hr         || workout.avg_hr,
      max_hr:          res.max_hr         || workout.max_hr,
      calories_burned: res.calories_burned || workout.calories_burned,
      notes:           res.notes || res.raw_notes || workout.notes,
      exercises:       (res.exercises || []).map(mapExercise),
    }

    onParsed()
  }

  function mapExercise(e) {
    const setsArr = Array.isArray(e.sets) ? e.sets : []
    const setCount = setsArr.length || ''
    const firstSet = setsArr[0] ?? {}

    let weightDisplay = e.load_raw || ''
    if (!weightDisplay && firstSet.load_kg > 0) {
      weightDisplay = String(firstSet.load_kg) + ' kg'
    }

    return {
      name:        e.name        || '',
      sets:        setCount,
      reps:        firstSet.reps ?? '',
      duration:    e.duration_raw || '',
      weight_lbs:  weightDisplay,
      tempo:       e.tempo        || '',
      rpe:         e.rpe          ?? '',
      pattern:     e.category     || '',
      bias:        e.bias         || '',
      distance:    e.distance_km  || '',
      elevation:   e.elevation_m  || '',
      pace:        e.pace         || '',
      met:         e.met_value    ?? '',
      notes:       e.notes        || '',
    }
  }
</script>

<div class="border rounded-lg p-4">
  <h3 class="text-lg font-semibold mb-3">YAML Parse</h3>
  {#if error}
    <div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
      {error}
    </div>
  {/if}
  <textarea
    bind:value={yamlRaw}
    class="w-full p-2 border rounded mb-3 font-mono text-sm"
    rows="6"
    placeholder="Enter workout YAML (e.g., Squat: 3x5 @ 100kg...)"
  ></textarea>
  <button
    onclick={parseYAML}
    disabled={yamlParsing || !yamlRaw.trim()}
    class="px-4 py-2 bg-primary-600 text-white rounded hover:bg-primary-700 disabled:opacity-50"
  >
    {yamlParsing ? 'Parsing...' : 'Parse YAML'}
  </button>
  {#if yamlParsed}
    <div class="mt-2 text-sm text-green-600">✓ YAML parse successful</div>
  {/if}
</div>
