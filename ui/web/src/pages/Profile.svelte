<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { dispHeightCm, heightFtInToCm, kmToMi, miToKm, distUnit, copyToClipboard } from '../lib/utils.js'
  import { setUnits, setSleepQualityMax, setCurrentPage, setUser, store } from '../lib/stores.svelte.js'
  import Alert from '../components/Alert.svelte'
  import InfoTip from '../components/InfoTip.svelte'

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

  // ── Change password ─────────────────────────────────────
  let pw = $state({ current: '', next: '', confirm: '' })
  let pwError = $state('')
  let pwSuccess = $state('')
  let pwLoading = $state(false)

  async function submitPassword() {
    pwError = ''
    pwSuccess = ''
    if (pw.next.length < 8) {
      pwError = 'New password must be at least 8 characters.'
      return
    }
    if (pw.next !== pw.confirm) {
      pwError = 'New password and confirmation do not match.'
      return
    }
    pwLoading = true
    try {
      await api.changePassword(pw.current, pw.next)
      pw = { current: '', next: '', confirm: '' }
      pwSuccess = 'Password changed — other devices have been logged out.'
    } catch (e) {
      pwError = e.message || 'Could not change password.'
    } finally {
      pwLoading = false
    }
  }

  // ── Two-factor authentication ───────────────────────────
  // Enrolment is a four-beat state machine: idle → setup (pending secret +
  // QR) → done (recovery codes shown once) → idle with mfaEnabled true.
  // The enabled card adds a separate regenerate → regenerated → idle loop.
  let mfaStep = $state('idle') // idle | setup | done | regenerated
  let mfaSecret = $state('')
  let mfaCode = $state('')
  let mfaError = $state('')
  let mfaNotice = $state('')
  let mfaBusy = $state(false)
  let mfaRecoveryCodes = $state([])
  let mfaDisablePassword = $state('')
  let mfaDisableCode = $state('')
  let mfaRegenCode = $state('')
  let recoveryCopied = $state(false)

  // /auth/me carries mfaEnabled; App/Login put that response in the shared
  // store, so the card follows the account rather than local guesswork.
  let mfaEnabled = $derived(store.user?.mfaEnabled === true)

  async function beginMfaSetup() {
    mfaError = ''
    mfaNotice = ''
    mfaBusy = true
    try {
      const res = await api.mfaSetup()
      mfaSecret = res?.secret || ''
      mfaCode = ''
      mfaStep = 'setup'
    } catch (e) {
      mfaError = e.message || 'Could not start 2FA setup.'
    } finally {
      mfaBusy = false
    }
  }

  async function confirmMfa() {
    mfaError = ''
    mfaNotice = ''
    if (!mfaCode.trim()) {
      mfaError = 'Enter the 6-digit code from your authenticator app.'
      return
    }
    mfaBusy = true
    try {
      const res = await api.mfaConfirm(mfaSecret, mfaCode.trim())
      mfaRecoveryCodes = res?.recoveryCodes || []
      mfaSecret = ''
      mfaCode = ''
      setUser({ ...(store.user || {}), mfaEnabled: true })
      mfaStep = 'done'
    } catch (e) {
      mfaError = e.message || 'Could not enable 2FA.'
    } finally {
      mfaBusy = false
    }
  }

  async function disableMfa() {
    mfaError = ''
    mfaNotice = ''
    if (!mfaDisablePassword || !mfaDisableCode.trim()) {
      mfaError = 'Enter your current password and a 6-digit or recovery code.'
      return
    }
    mfaBusy = true
    try {
      await api.mfaDisable(mfaDisablePassword, mfaDisableCode.trim())
      mfaDisablePassword = ''
      mfaDisableCode = ''
      setUser({ ...(store.user || {}), mfaEnabled: false })
      mfaStep = 'idle'
      mfaNotice = 'Two-factor authentication is off.'
    } catch (e) {
      // Wrong password is a 401 whose server copy ("invalid credentials") is
      // authoritative; surface it verbatim.
      mfaError = e.message || 'Could not disable 2FA.'
    } finally {
      mfaBusy = false
    }
  }

  async function regenerateMfa() {
    mfaError = ''
    mfaNotice = ''
    if (!mfaRegenCode.trim()) {
      mfaError = 'Enter a 6-digit or recovery code to replace your backup codes.'
      return
    }
    mfaBusy = true
    try {
      const res = await api.mfaRegenerate(mfaRegenCode.trim())
      mfaRecoveryCodes = res?.recoveryCodes || []
      mfaRegenCode = ''
      mfaStep = 'regenerated'
    } catch (e) {
      mfaError = e.message || 'Could not regenerate recovery codes.'
    } finally {
      mfaBusy = false
    }
  }

  function finishRecovery() {
    const wasRegenerate = mfaStep === 'regenerated'
    mfaStep = 'idle'
    mfaRecoveryCodes = []
    recoveryCopied = false
    mfaNotice = wasRegenerate
      ? 'Recovery codes replaced — the old codes no longer work.'
      : 'Two-factor authentication is now on.'
  }

  function cancelMfaSetup() {
    mfaStep = 'idle'
    mfaSecret = ''
    mfaCode = ''
    mfaError = ''
  }

  function recoveryText() {
    return [
      'DietTracker recovery codes',
      '',
      ...mfaRecoveryCodes,
      '',
      'Each code works once. Store them somewhere safe.',
      '',
    ].join('\n')
  }

  async function copyRecoveryCodes() {
    const ok = await copyToClipboard(recoveryText())
    recoveryCopied = ok
    if (ok) {
      setTimeout(() => { recoveryCopied = false }, 2000)
    } else {
      mfaError = 'Copy failed — select the codes and copy them manually.'
    }
  }

  function downloadRecoveryCodes() {
    const blob = new Blob([recoveryText()], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'diet-tracker-recovery-codes.txt'
    document.body.appendChild(a)
    a.click()
    a.remove()
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
      <div class="flex items-center gap-1">
        <label class="text-xs text-gray-400" for="pf-macro-dist">Macro Distribution</label>
        <InfoTip label="macro distribution" text="How leftover calories split between carbs and fat: keto (~30g carbs), low_carb (~100g), balanced (~40% fat), high_carb (smaller fat share)." />
      </div>
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
      <input class="input" id="pf-bf" type="number" min="0" max="70" step="0.1" placeholder="e.g. 18" bind:value={profile.bfPct} />
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
    <div class="flex items-center gap-1">
      <label class="flex items-center space-x-2 text-sm">
        <input type="checkbox" bind:checked={profile.prioritizeCarbs} />
        <span>Prioritize carbs</span>
      </label>
      <InfoTip label="prioritize carbs" text="Keeps carbohydrate higher on training days — protein still comes first." />
    </div>
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

  <!-- Change password -->
  <div class="pt-2 border-t border-gray-700 space-y-3">
    <h3 class="text-emerald-400 font-semibold text-sm">Change password</h3>
    {#if pwError}<Alert type="error" message={pwError} />{/if}
    {#if pwSuccess}<Alert type="success" message={pwSuccess} />{/if}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
      <div>
        <label class="text-xs text-gray-400" for="pw-current">Current password</label>
        <input class="input" id="pw-current" type="password" autocomplete="current-password" bind:value={pw.current} />
      </div>
      <div>
        <label class="text-xs text-gray-400" for="pw-new">New password</label>
        <input class="input" id="pw-new" type="password" autocomplete="new-password" bind:value={pw.next} />
      </div>
      <div>
        <label class="text-xs text-gray-400" for="pw-confirm">Confirm new password</label>
        <input class="input" id="pw-confirm" type="password" autocomplete="new-password" bind:value={pw.confirm} />
      </div>
    </div>
    <button class="btn-primary" onclick={submitPassword} disabled={pwLoading} type="button">
      {pwLoading ? 'Saving…' : 'Change password'}
    </button>
  </div>

  <!-- Guide link -->
  <div class="flex items-center justify-between pt-2 border-t border-gray-700">
    <span class="text-xs text-gray-500">New here, or need a refresher?</span>
    <button class="text-xs text-emerald-400 underline" onclick={() => setCurrentPage('guide')} type="button">
      📖 Guide
    </button>
  </div>

  <!-- Admin link (admins only) -->
  {#if store.user?.isAdmin}
    <div class="flex items-center justify-between pt-2 border-t border-gray-700">
      <span class="text-xs text-gray-500">Manage registration, invites, and approvals.</span>
      <button class="text-xs text-emerald-400 underline" onclick={() => setCurrentPage('admin')} type="button">
        🛠 Admin
      </button>
    </div>
  {/if}

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

<!-- ── Two-factor authentication ─────────────────────────── -->
<div class="max-w-screen-xl mx-auto bg-gray-800 p-4 rounded border border-gray-700 space-y-3 mt-4">
  <h3 class="text-emerald-400 font-semibold text-sm">Two-factor authentication</h3>
  {#if mfaError}<Alert type="error" message={mfaError} />{/if}
  {#if mfaNotice}<Alert type="success" message={mfaNotice} />{/if}

  {#if mfaStep === 'done' || mfaStep === 'regenerated'}
    <!-- Recovery codes are displayed exactly once. -->
    <Alert
      type="success"
      message={mfaStep === 'regenerated'
        ? 'New recovery codes generated. Save them now — they will not be shown again.'
        : '2FA is on. Save these recovery codes now — they will not be shown again.'}
    />
    <p class="text-sm text-amber-300">
      Each code works once — store them safely. If you lose your phone and these codes, ask the owner for a one-time reset.
    </p>
    <div class="grid grid-cols-2 md:grid-cols-5 gap-2">
      {#each mfaRecoveryCodes as code}
        <code class="bg-gray-900 border border-gray-700 rounded px-2 py-1 text-center font-mono text-sm text-gray-100 select-all">{code}</code>
      {/each}
    </div>
    <div class="flex flex-wrap items-center gap-3">
      <button class="btn-primary" onclick={copyRecoveryCodes} type="button">
        {recoveryCopied ? 'Copied!' : 'Copy all codes'}
      </button>
      <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={downloadRecoveryCodes} type="button">
        Download .txt
      </button>
      <button class="text-xs text-emerald-400 underline" onclick={finishRecovery} type="button">
        Done
      </button>
    </div>
  {:else if !mfaEnabled}
    <p class="text-sm text-gray-400">
      Add a second step at sign-in with any authenticator app. 2FA is off for this account.
    </p>
    {#if mfaStep === 'setup'}
      <div class="flex flex-col md:flex-row gap-4">
        <div class="space-y-2">
          <img
            src={api.mfaQRUrl(mfaSecret, store.user?.email)}
            alt="Authenticator setup QR code"
            width="200"
            height="200"
            class="bg-white p-2 rounded"
          />
          <p class="text-xs text-gray-500">Scan with your authenticator app.</p>
        </div>
        <div class="space-y-3 flex-1">
          <div>
            <div class="text-xs text-gray-400 mb-1">Can’t scan? Enter this secret manually:</div>
            <div class="flex items-center gap-2">
              <code class="bg-gray-900 border border-gray-700 rounded px-2 py-1 font-mono text-sm text-gray-100 select-all break-all">{mfaSecret}</code>
              <button
                class="text-xs text-emerald-400 underline"
                onclick={() => copyToClipboard(mfaSecret)}
                type="button"
              >
                Copy
              </button>
            </div>
          </div>
          <div>
            <label class="text-xs text-gray-400 block mb-1" for="mfa-confirm-code">6-digit code from the app</label>
            <input
              class="input font-mono"
              id="mfa-confirm-code"
              inputmode="numeric"
              autocomplete="one-time-code"
              placeholder="123456"
              bind:value={mfaCode}
            />
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <button class="btn-primary" onclick={confirmMfa} disabled={mfaBusy} type="button">
              {mfaBusy ? 'Confirming…' : 'Confirm'}
            </button>
            <button class="text-xs text-gray-400 underline" onclick={cancelMfaSetup} disabled={mfaBusy} type="button">
              Cancel
            </button>
          </div>
        </div>
      </div>
    {:else}
      <button class="btn-primary" onclick={beginMfaSetup} disabled={mfaBusy} type="button">
        {mfaBusy ? 'Starting…' : 'Enable 2FA'}
      </button>
    {/if}
  {:else}
    <p class="text-sm text-emerald-300">2FA is on for this account.</p>

    <div class="pt-2 border-t border-gray-700 space-y-2">
      <h4 class="text-sm text-gray-200 font-semibold">Regenerate recovery codes</h4>
      <p class="text-xs text-gray-500">
        Replaces every existing backup code with ten new single-use codes. A live authenticator or backup code is required.
      </p>
      <div class="flex flex-col sm:flex-row gap-2 sm:items-end">
        <div class="flex-1">
          <label class="text-xs text-gray-400 block mb-1" for="mfa-regen-code">Current code</label>
          <input
            class="input font-mono"
            id="mfa-regen-code"
            autocomplete="one-time-code"
            placeholder="6-digit or recovery code"
            bind:value={mfaRegenCode}
          />
        </div>
        <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={regenerateMfa} disabled={mfaBusy} type="button">
          {mfaBusy ? 'Working…' : 'Regenerate'}
        </button>
      </div>
    </div>

    <div class="pt-2 border-t border-gray-700 space-y-2">
      <h4 class="text-sm text-gray-200 font-semibold">Disable 2FA</h4>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-gray-400 block mb-1" for="mfa-disable-password">Current password</label>
          <input
            class="input"
            id="mfa-disable-password"
            type="password"
            autocomplete="current-password"
            bind:value={mfaDisablePassword}
          />
        </div>
        <div>
          <label class="text-xs text-gray-400 block mb-1" for="mfa-disable-code">6-digit or recovery code</label>
          <input
            class="input font-mono"
            id="mfa-disable-code"
            autocomplete="one-time-code"
            placeholder="123456"
            bind:value={mfaDisableCode}
          />
        </div>
      </div>
      <button class="bg-red-700 px-3 py-2 rounded-lg text-sm hover:bg-red-600" onclick={disableMfa} disabled={mfaBusy} type="button">
        {mfaBusy ? 'Disabling…' : 'Disable 2FA'}
      </button>
    </div>
  {/if}
</div>

