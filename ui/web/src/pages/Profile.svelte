<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { dispHeightCm, heightFtInToCm, kmToMi, miToKm, distUnit } from '../lib/utils.js'
  import { setUnits, setSleepQualityMax, setCurrentPage } from '../lib/stores.svelte.js'
  import Alert from '../components/Alert.svelte'

  // ── Profile ─────────────────────────────────────────────
  let profile = $state({
    name: '', age: '', birthdate: '', sex: 'male', heightCm: '', activity: 'sedentary',
    exerciseFreq: '', runningKm: '', isLifter: false, goal: 'maintenance',
    prioritizeCarbs: false, bfPct: '', hrRest: '', hrMax: '',
    gripWeight: 0.5, tdeeLookbackDays: 90, sleepQualityMax: 10, units: 'imperial',
    macroDistribution: 'balanced',
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
          birthdate: p.birthdate || '',
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
          macroDistribution: p.macroDistribution || 'balanced',
        }
            // Convert runningKm to display units for the input
                if (profile.units === 'imperial' && p.runningKm) {
                profile.runningKm = String(kmToMi(p.runningKm))
            }
      }
    } catch (e) {
      profileError = 'Failed to load profile: ' + (e.message || e)
    }
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
            birthdate: profile.birthdate || '',
            macroDistribution: profile.macroDistribution || 'balanced',
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

  // ── Targets (read-only; edited on the Targets page) ─────
  let targets = $state({ calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '' })
  let targetsError = $state('')

  async function loadTargets() {
    try {
      const t = await api.getTargets()
      if (t) targets = { ...targets, ...t }
    } catch (e) {
      targetsError = 'Failed to load targets: ' + (e.message || e)
    }
  }

  function targetText(value, unit = '') {
    if (value === '' || value === null || value === undefined) return '—'
    return unit ? `${value} ${unit}` : String(value)
  }

  // ── Export ──────────────────────────────────────────────
  function openExport() {
    setCurrentPage('export')
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
      <label class="text-xs text-gray-400" for="pf-birthdate">Birthdate</label>
      <input class="input" id="pf-birthdate" type="date" bind:value={profile.birthdate} />
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
      <label class="text-xs text-gray-400" for="pf-macro-dist">Macro Distribution</label>
      <select class="input" id="pf-macro-dist" bind:value={profile.macroDistribution}>
        <option value="balanced">Balanced</option>
        <option value="keto">Keto (very low carb, high fat)</option>
        <option value="low_carb">Low Carb (moderate carb, higher fat)</option>
        <option value="high_carb">High Carb (lower fat)</option>
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

  <!-- Current targets (read-only) -->
  <div class="pt-2 border-t border-gray-700">
    <div class="flex items-center justify-between mb-2">
      <h3 class="text-emerald-400 font-semibold text-sm">Current Targets</h3>
      <button class="text-xs text-emerald-400 underline" onclick={() => setCurrentPage('targets')}>
        Edit on Targets page →
      </button>
    </div>
    {#if targetsError}<Alert type="error" message={targetsError} />{/if}
    <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 text-sm">
      <div>
        <div class="text-xs text-gray-500">Calories</div>
        <div class="text-white font-mono">{targetText(targets.calories, 'kcal')}</div>
      </div>
      <div>
        <div class="text-xs text-gray-500">Protein</div>
        <div class="text-white font-mono">{targetText(targets.proteinG, 'g')}</div>
      </div>
      <div>
        <div class="text-xs text-gray-500">Carbs</div>
        <div class="text-white font-mono">{targetText(targets.carbsG, 'g')}</div>
      </div>
      <div>
        <div class="text-xs text-gray-500">Fat</div>
        <div class="text-white font-mono">{targetText(targets.fatG, 'g')}</div>
      </div>
      <div>
        <div class="text-xs text-gray-500">Fiber</div>
        <div class="text-white font-mono">{targetText(targets.fiberG, 'g')}</div>
      </div>
      <div>
        <div class="text-xs text-gray-500">Water</div>
        <div class="text-white font-mono">{targetText(targets.waterMl, 'ml')}</div>
      </div>
    </div>
  </div>

  <!-- Action row -->
  <div class="flex flex-wrap items-center gap-3 pt-2 border-t border-gray-700">
    <button class="btn-primary" onclick={submitProfile} disabled={profileLoading}>
      {profileLoading ? 'Saving…' : 'Save Profile'}
    </button>
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={() => setCurrentPage('targets')}>
      🎯 Edit Targets
    </button>
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={openExport}>
      📤 Export
    </button>
  </div>
</div>

