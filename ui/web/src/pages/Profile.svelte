<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { copyToClipboard, dispHeightCm, heightFtInToCm, kmToMi, miToKm, distUnit, today, daysAgo } from '../lib/utils.js'
  import { setUnits, setSleepQualityMax } from '../lib/stores.svelte.js'
  import Alert from '../components/Alert.svelte'

  // ── Profile ─────────────────────────────────────────────
  let profile = $state({
    name: '', age: '', sex: 'male', heightCm: '', activity: 'sedentary',
    exerciseFreq: '', runningKm: '', isLifter: false, goal: 'maintenance',
    prioritizeCarbs: false, bfPct: '', hrRest: '', hrMax: '',
    gripWeight: 0.5, tdeeLookbackDays: 90, sleepQualityMax: 10, units: 'imperial',
  })
  let profileLoading = $state(false)
  let profileError = $state('')
  let profileSuccess = $state('')

  // Height display: for imperial, show separate ft/in inputs
  let heightFt = $state('')
  let heightIn = $state('')
  // When profile loads, split heightCm into ft+in for imperial display
  $effect(() => {
    if (profile.units === 'imperial' && profile.heightCm) {
      const totalIn = Number(profile.heightCm) / 2.54
      heightFt = String(Math.floor(totalIn / 12))
      heightIn = String(Math.round(totalIn % 12))
    }
  })

  onMount(async () => {
    try {
      const p = await api.getProfile()
      if (p) {
        profile = { 
          ...profile, 
          name: p.name || '',
          age: p.age || '',
          sex: p.sex || 'male',
          heightCm: p.heightCm ?? '',
          activity: p.activity || 'sedentary',
          exerciseFreq: p.exerciseFreq ?? '',
          runningKm: p.runningKm ?? '',
          isLifter: p.isLifter ?? false,
          goal: p.goal || 'maintenance',
          prioritizeCarbs: p.prioritizeCarbs ?? false,
          bfPct: p.bfPct ?? '',
          hrRest: p.hrRest ?? '',
          hrMax: p.hrMax ?? '',
          gripWeight: p.gripWeight ?? 0.5,
          tdeeLookbackDays: p.tdeeLookbackDays ?? 90,
          sleepQualityMax: p.sleepQualityMax ?? 10,
          units: p.units || 'imperial',
        }
            // Convert runningKm to display units for the input
                if (profile.units === 'imperial' && p.runningKm) {
                profile.runningKm = String(kmToMi(p.runningKm))
            }
      }
    } catch {}
    await loadTargets()
  })

  async function submitProfile() {
    profileError = ''
    profileSuccess = ''
    profileLoading = true
    try {
        await api.updateProfile({
            name: profile.name,
            age: Number(profile.age) || 0,
            sex: profile.sex,
            heightCm: profile.units === 'imperial'
                ? heightFtInToCm(heightFt, heightIn)
                : Number(profile.heightCm) || 0,
            activity: profile.activity,
            exerciseFreq: Number(profile.exerciseFreq) || 0,
            runningKm: profile.units === 'imperial'
                ? miToKm(Number(profile.runningKm) || 0)
                : Number(profile.runningKm) || 0,
            isLifter: profile.isLifter || false,
            goal: profile.goal,
            prioritizeCarbs: profile.prioritizeCarbs || false,
            bfPct: Number(profile.bfPct) || 0,
            hrRest: Number(profile.hrRest) || 0,
            hrMax: Number(profile.hrMax) || 0,
            gripWeight: Number(profile.gripWeight) || 0.5,
            tdeeLookbackDays: Number(profile.tdeeLookbackDays) || 90,
            sleepQualityMax: Number(profile.sleepQualityMax) || 10,
            units: profile.units,
        })
        setUnits(profile.units)
        setSleepQualityMax(profile.sleepQualityMax)
        profileSuccess = 'Profile updated'
    } catch (e) {
      profileError = e.message
    } finally {
      profileLoading = false
    }
  }

  // ── Targets modal ───────────────────────────────────────
  let showTargets = $state(false)
  let targets = $state({ calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '' })
  let tdee = $state(null)
  let targetsLoading = $state(false)
  let targetsError = $state('')
  let targetsSuccess = $state('')

  async function loadTargets() {
    try {
      const t = await api.getTargets()
      if (t) targets = { ...targets, ...t }
    } catch {}
    try {
      const res = await api.getTDEE(30)
      tdee = res
    } catch {}
  }

  async function submitTargets() {
    targetsError = ''
    targetsSuccess = ''
    targetsLoading = true
    try {
      await api.putTargets({
        calories:  Number(targets.calories)  || 0,
        proteinG: Number(targets.proteinG) || 0,
        carbsG:   Number(targets.carbsG)   || 0,
        fatG:     Number(targets.fatG)     || 0,
        fiberG:   Number(targets.fiberG)   || 0,
        waterMl:  Number(targets.waterMl)  || 0,
      })
      targetsSuccess = 'Targets updated'
    } catch (e) {
      targetsError = e.message
    } finally {
      targetsLoading = false
    }
  }

  // ── Export modal ────────────────────────────────────────
  let showExport = $state(false)
  let expFrom = $state(daysAgo(7))
  let expTo = $state(today())
  let expFormat = $state('md')
  let expContent = $state('')
  let expError = $state('')
  let expSuccess = $state('')
  let expCopied = $state(false)

  async function exportNutrition() {
    expError = ''
    try {
      const res = await api.exportNutrition(expFrom, expTo, expFormat)
      expContent = res.content || ''
    } catch (e) { expError = e.message }
  }

  async function exportWorkouts() {
    expError = ''
    try {
      const res = await api.exportWorkouts(expFrom, expTo, expFormat)
      expContent = res.content || ''
    } catch (e) { expError = e.message }
  }

  async function exportCombined() {
    expError = ''
    try {
      const res = await api.exportCombined(expFrom, expTo)
      expContent = res.content || ''
    } catch (e) { expError = e.message }
  }

  async function doCopy() {
    const ok = await copyToClipboard(expContent)
    if (ok) {
      expCopied = true
      setTimeout(() => expCopied = false, 2000)
    } else {
      expError = 'Copy failed'
      setTimeout(() => expError = '', 2000)
    }
  }

  function doDownload() {
    if (!expContent) return
    const blob = new Blob([expContent], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'export.md'
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  }
</script>

<!-- ── Profile form ──────────────────────────────────────── -->
{#if profileError}<Alert type="error" message={profileError} />{/if}
{#if profileSuccess}<Alert type="success" message={profileSuccess} />{/if}

<div class="max-w-screen-xl mx-auto bg-gray-800 p-4 rounded border border-gray-700 space-y-3">
  <h2 class="text-emerald-400 font-bold text-lg">Profile</h2>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
    <div>
      <label class="text-xs text-gray-400" for="pf-name">Name</label>
      <input class="input" id="pf-name" placeholder="Name" bind:value={profile.name} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-age">Age</label>
      <input class="input" id="pf-age" type="number" placeholder="Age" bind:value={profile.age} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-sex">Sex</label>
      <select class="input" id="pf-sex" bind:value={profile.sex}>
        <option value="male">Male</option>
        <option value="female">Female</option>
      </select>
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-height">Height {profile.units === 'imperial' ? '(ft / in)' : '(cm)'}</label>
      {#if profile.units === 'imperial'}
        <div class="flex gap-2">
          <input class="input" id="pf-height" type="number" placeholder="ft" bind:value={heightFt} />
          <input class="input" id="pf-height-in" type="number" placeholder="in" bind:value={heightIn} />
        </div>
        {:else}
        <input class="input" id="pf-height" type="number" placeholder="Height (cm)" bind:value={profile.heightCm} />
      {/if}
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-units">Units</label>
      <select class="input" id="pf-units" bind:value={profile.units}>
        <option value="imperial">Imperial</option>
        <option value="metric">Metric</option>
      </select>
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-activity">Activity Level</label>
      <select class="input" id="pf-activity" bind:value={profile.activity}>
        <option value="sedentary">Sedentary</option>
        <option value="lightly_active">Lightly Active</option>
        <option value="moderately_active">Moderately Active</option>
        <option value="very_active">Very Active</option>
      </select>
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-goal">Goal</label>
      <select class="input" id="pf-goal" bind:value={profile.goal}>
        <option value="cut_10">Cut 10%</option>
        <option value="cut_20">Cut 20%</option>
        <option value="cut_30">Cut 30%</option>
        <option value="maintenance">Maintenance</option>
        <option value="bulk_10">Bulk 10%</option>
        <option value="bulk_20">Bulk 20%</option>
      </select>
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-exercise-freq">Exercise Days/Week</label>
      <input class="input" id="pf-exercise-freq" type="number" placeholder="0–7" bind:value={profile.exerciseFreq} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-running">Running {distUnit(profile.units)}/Week</label>
      <input class="input" id="pf-running" type="number" placeholder="0" bind:value={profile.runningKm} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-bf">Body Fat %</label>
      <input class="input" id="pf-bf" type="number" placeholder="e.g. 18" bind:value={profile.bfPct} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-hr-rest">Resting HR</label>
      <input class="input" id="pf-hr-rest" type="number" placeholder="bpm" bind:value={profile.hrRest} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-hr-max">Max HR</label>
      <input class="input" id="pf-hr-max" type="number" placeholder="bpm" bind:value={profile.hrMax} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-tdee-days">TDEE Lookback Days</label>
      <input class="input" id="pf-tdee-days" type="number" placeholder="90" bind:value={profile.tdeeLookbackDays} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="pf-sleep-max">Sleep Quality Scale Max</label>
      <input class="input" id="pf-sleep-max" type="number" placeholder="10" bind:value={profile.sleepQualityMax} />
    </div>
  </div>

  <div class="pt-2">
    <label class="text-xs text-gray-400 block mb-1" for="pf-grip-weight">
      Readiness Weighting — BOLT vs Grip
      <span class="text-white font-mono ml-2">
      {#if Number(profile.gripWeight) === 0}BOLT only
        {:else if Number(profile.gripWeight) === 1}Grip only
        {:else}Grip {Math.round(Number(profile.gripWeight) * 100)}% · BOLT {Math.round((1 - Number(profile.gripWeight)) * 100)}%
        {/if}
      </span>
    </label>
    <div class="flex items-center space-x-3">
      <span class="text-xs text-gray-500">BOLT</span>
      <input id="pf-grip-weight" type="range" min="0" max="1" step="0.05" bind:value={profile.gripWeight} class="w-full accent-emerald-500" />
      <span class="text-xs text-gray-500">Grip</span>
    </div>
  </div>

  <div class="flex items-center space-x-4 pt-1">
    <label class="flex items-center space-x-2 text-sm">
      <input type="checkbox" bind:checked={profile.isLifter} />
      <span>I am a lifter</span>
    </label>
    <label class="flex items-center space-x-2 text-sm">
      <input type="checkbox" bind:checked={profile.prioritizeCarbs} />
      <span>Prioritize carbs</span>
    </label>
  </div>

  <!-- Action row -->
  <div class="flex flex-wrap items-center gap-3 pt-2 border-t border-gray-700">
    <button class="btn-primary" onclick={submitProfile} disabled={profileLoading}>
      {profileLoading ? 'Saving…' : 'Save Profile'}
    </button>
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={() => showTargets = true}>
      🎯 Targets
    </button>
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={() => showExport = true}>
      📤 Export
    </button>
  </div>
</div>

<!-- ── Targets modal ──────────────────────────────────────── -->
{#if showTargets}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
    role="presentation"
    onclick={() => showTargets = false}
    onkeydown={(e) => { if (e.key === 'Escape') showTargets = false }}
  >
    <div
      class="bg-gray-800 border border-gray-700 rounded-lg shadow-2xl w-full max-w-md mx-4 p-5"
      role="dialog"
      aria-modal="true"
      tabindex="0"
      onclick={(e) => e.stopPropagation()}
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-emerald-400 font-bold text-lg">🎯 Targets</h3>
        <button class="text-gray-400 hover:text-white text-xl leading-none" onclick={() => showTargets = false}>✕</button>
      </div>

      {#if targetsError}<Alert type="error" message={targetsError} />{/if}
      {#if targetsSuccess}<Alert type="success" message={targetsSuccess} />{/if}

      {#if tdee}
        <div class="bg-gray-700 p-3 rounded text-sm space-y-1 mb-4">
          <div class="text-emerald-400 font-semibold">Suggested from your data</div>
          <div>Observed TDEE: <span class="text-white font-mono">{Math.round(tdee.observedTdee ?? 0)} kcal</span></div>
          <div>Estimated TDEE: <span class="text-white font-mono">{Math.round(tdee.estimatedTdee ?? 0)} kcal</span></div>
          <div class="text-gray-400 text-xs">{tdee.confidence ?? ''} confidence · {tdee.daysOfData ?? 0} days of data</div>
          <button
            class="mt-2 text-xs text-emerald-400 underline"
            onclick={() => { targets.calories = String(Math.round(tdee.observedTdee ?? tdee.estimatedTdee ?? 0)) }}
          >Use observed TDEE as calorie target</button>
        </div>
      {/if}

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-xs text-gray-400" for="pt-calories">Calories (kcal)</label>
            <input class="input" id="pt-calories" type="number" placeholder="e.g. 2200" bind:value={targets.calories} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pt-protein">Protein (g)</label>
            <input class="input" id="pt-protein" type="number" placeholder="e.g. 180" bind:value={targets.proteinG} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pt-carbs">Carbs (g)</label>
            <input class="input" id="pt-carbs" type="number" placeholder="e.g. 220" bind:value={targets.carbsG} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pt-fat">Fat (g)</label>
            <input class="input" id="pt-fat" type="number" placeholder="e.g. 70" bind:value={targets.fatG} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pt-fiber">Fiber (g)</label>
            <input class="input" id="pt-fiber" type="number" placeholder="e.g. 30" bind:value={targets.fiberG} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pt-water">Water (ml)</label>
            <input class="input" id="pt-water" type="number" placeholder="e.g. 2500" bind:value={targets.waterMl} />
          </div>
        </div>

      <div class="flex gap-3 mt-4">
        <button class="btn-primary" onclick={submitTargets} disabled={targetsLoading}>
          {targetsLoading ? 'Saving…' : 'Save Targets'}
        </button>
        <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm" onclick={() => showTargets = false}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

<!-- ── Export modal ───────────────────────────────────────── -->
{#if showExport}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
    role="presentation"
    onclick={() => showExport = false}
    onkeydown={(e) => { if (e.key === 'Escape') showExport = false }}
  >
    <div
      class="bg-gray-800 border border-gray-700 rounded-lg shadow-2xl w-full max-w-lg mx-4 p-5 max-h-[90vh] overflow-y-auto"
      role="dialog"
      aria-modal="true"
      tabindex="0"
      onclick={(e) => e.stopPropagation()}
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-emerald-400 font-bold text-lg">📤 Export</h3>
        <button class="text-gray-400 hover:text-white text-xl leading-none" onclick={() => showExport = false}>✕</button>
      </div>

      {#if expError}<Alert type="error" message={expError} />{/if}
      {#if expSuccess}<Alert type="success" message={expSuccess} />{/if}

      <div class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-xs text-gray-400" for="pe-from">From</label>
            <input class="input" id="pe-from" type="date" bind:value={expFrom} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="pe-to">To</label>
            <input class="input" id="pe-to" type="date" bind:value={expTo} />
          </div>
        </div>
        <div>
          <label class="text-xs text-gray-400" for="pe-format">Format</label>
          <select class="input" id="pe-format" bind:value={expFormat}>
            <option value="md">Markdown</option>
            <option value="csv">CSV</option>
          </select>
        </div>

        <div class="flex flex-wrap gap-2">
          <button class="btn-primary text-sm" onclick={exportNutrition}>Nutrition</button>
          <button class="btn-primary text-sm" onclick={exportWorkouts}>Workouts</button>
          <button class="btn-primary text-sm" onclick={exportCombined}>Combined</button>
          <button class="btn-secondary text-sm" class:btn-success={expCopied} onclick={doCopy} disabled={!expContent}>{expCopied ? '✓ Copied!' : 'Copy'}</button>
          <button class="btn-secondary text-sm" onclick={doDownload} disabled={!expContent}>Download</button>
          <button class="btn-secondary text-sm" onclick={() => expContent = ''} disabled={!expContent}>Clear</button>
        </div>

        {#if expContent}
          <pre class="whitespace-pre-wrap bg-gray-900 p-3 rounded border border-gray-700 text-xs max-h-64 overflow-y-auto">{expContent}</pre>
        {/if}
      </div>

      <div class="mt-4">
        <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm" onclick={() => showExport = false}>Close</button>
      </div>
    </div>
  </div>
{/if}
