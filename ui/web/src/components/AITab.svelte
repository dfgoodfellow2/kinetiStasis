<script>
  import { api } from '../lib/api.js'
  import { today } from '../lib/utils.js'

  let { workout = $bindable(), onParsed = () => {} } = $props()

  let aiRaw = $state('')
  let aiParsed = $state(false)
  let aiParsing = $state(false)
  let error = $state('')

  async function parseAI() {
    if (!aiRaw.trim()) return
    aiParsing = true
    error = ''
    try {
      const res = await api.parseWorkout(aiRaw.trim(), 'ai')
      if (res && !res.error) {
        applyParsed(res)
        aiParsed = true
      } else {
        error = 'AI parse failed: ' + (res?.error ?? 'unknown error')
      }
    } catch (e) {
      error = 'AI parse error: ' + e.message
    } finally {
      aiParsing = false
    }
  }

  function applyParsed(res) {
    // Normalise focus: may be array or JSON string
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
      date:            res.date           || today(),
      slot:            res.slot           ? String(res.slot) : workout.slot,
      title:           res.title          || workout.title,
      type:            res.type           || workout.type,
      style:           res.style          || workout.style,
      surface:         res.surface        || workout.surface,
      focus:           focusStr           || workout.focus,
      rest_interval:   res.restInterval  || res.rest_interval || workout.rest_interval,
      duration_min:    res.durationMin   || res.duration_min   || workout.duration_min,
      avg_hr:          res.avgHr         || res.avg_hr         || workout.avg_hr,
      max_hr:          res.maxHr         || res.max_hr         || workout.max_hr,
      calories_burned: res.caloriesBurned || res.calories_burned || workout.calories_burned,
      notes:           res.notes || res.raw_notes || workout.notes,
      exercises:       (res.exercises || []).map(mapExercise),
    }

    onParsed()
  }

  function mapExercise(e) {
    const setsArr = Array.isArray(e.sets) ? e.sets : []
    const setCount = setsArr.length || ''
    const firstSet = setsArr[0] ?? {}

    let weightDisplay = e.loadRaw || e.load_raw || ''
    if (!weightDisplay && (firstSet.loadKg > 0 || firstSet.load_kg > 0)) {
      const kg = firstSet.loadKg ?? firstSet.load_kg
      weightDisplay = String(kg) + ' kg'
    }

    return {
      name:        e.name        || '',
      sets:        setCount,
      reps:        firstSet.reps ?? '',
      duration:    e.durationRaw || e.duration_raw || '',
      weight_lbs:  weightDisplay,
      tempo:       e.tempo        || '',
      rpe:         e.rpe          ?? '',
      pattern:     e.category     || '',
      bias:        e.bias         || '',
      distance:    (e.distanceKm ?? e.distance_km) || '',
      elevation:   (e.elevationM ?? e.elevation_m) || '',
      pace:        e.pace         || '',
      met:         (e.metValue ?? e.met_value) ?? '',
      notes:       e.notes        || '',
    }
  }
</script>

<div class="bg-gray-800 p-4 rounded-lg border border-gray-700">
  <h3 class="text-lg font-semibold mb-3">AI Parse</h3>
  {#if error}
    <div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
      {error}
    </div>
  {/if}
  <textarea
    bind:value={aiRaw}
    class="input font-mono text-sm mb-3"
    rows="6"
    placeholder="Paste meal text or workout description..."
  ></textarea>
  <button
    onclick={parseAI}
    disabled={aiParsing || !aiRaw.trim()}
    class="btn-primary"
  >
    {aiParsing ? 'Parsing...' : 'Parse with AI'}
  </button>
  {#if aiParsed}
    <div class="mt-2 text-sm text-green-600">✓ AI parse successful</div>
  {/if}
</div>
