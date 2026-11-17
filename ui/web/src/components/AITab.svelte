<script>
  import { api } from '../lib/api.js'
  import { today, lbsToKg } from '../lib/utils.js'

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
      restInterval:   res.restInterval  || workout.restInterval,
      durationMin:    res.durationMin   || workout.durationMin,
      avgHr:          res.avgHr         || workout.avgHr,
      maxHr:          res.maxHr         || workout.maxHr,
      caloriesBurned: res.caloriesBurned || workout.caloriesBurned,
      notes:           res.notes || res.rawNotes || workout.notes,
      coachNotes:      res.coachNotes   || workout.coachNotes,
      // Session metadata that has no inline input but must survive a save.
      // ParsedWorkout always emits rpe/day (0 when unknown), so only a real
      // value may override metadata restored from an existing row.
      rpe:             res.rpe      || workout.rpe,
      recovers:        res.recovers || workout.recovers,
      day:             res.day      || workout.day,
      exercises:       (res.exercises || []).map(mapExercise),
    }

    onParsed()
  }

  function mapExercise(e) {
    const setsArr = Array.isArray(e.sets) ? e.sets : []
    const setCount = setsArr.length || ''
    const firstSet = setsArr[0] ?? {}

    // Never fabricate a load. A "70 kg" style fallback would be re-parsed as
    // pounds by WorkoutLog's parseLoad and then converted to kg on save,
    // corrupting every parsed exercise that carried set.loadKg only.
    const weightDisplay = e.loadRaw || ''

    return {
      name:        e.name        || '',
      sets:        setCount,
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
      // Carry the full parsed sets array through to save; WorkoutLog renders
      // one editable row per set, so a 10/8/6 pyramid survives. A row that
      // carries pounds but a zero kg mirror still has a real load, so recover
      // kg from lbs instead of dropping it.
      _sets: setsArr.map(s => {
        const loadKg = Number(s.loadKg) || 0
        const loadLbs = Number(s.loadLbs) || 0
        return {
          reps: s.reps ?? 0,
          loadKg: loadKg || (loadLbs > 0 ? lbsToKg(loadLbs) : 0),
          loadLbs,
          tutSeconds: s.tutSeconds ?? 0,
          restSeconds: s.restSeconds ?? 0,
        }
      }),
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
    placeholder={"Meal: 200g chicken, 1 cup rice, 2 tbsp olive oil\nWorkout: Bench 135x12, 185x8, 205x6"}
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
