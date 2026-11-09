<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { setUser, setCurrentPage } from '../lib/stores.svelte.js'
  import Alert from '../components/Alert.svelte'
  import Spinner from '../components/Spinner.svelte'

  let mode = $state('login')
  let username = $state('')
  let email = $state('')
  let password = $state('')
  let inviteCode = $state('')
  let error = $state('')
  let loading = $state(false)

  // ── Two-step login ──────────────────────────────────────
  // A password-correct login on a 2FA account returns { status:
  // 'mfa_required', challenge } and no cookies. The challenge is reused for
  // every retry (a wrong code does not burn it) until it expires.
  let mfaStep = $state('password') // password | code
  let mfaChallenge = $state('')
  let mfaCode = $state('')
  let mfaRecoveryMode = $state(false)
  let rememberDevice = $state(false)

  // Public posture: { mode: 'invite_only'|'public'|'public_approval', pendingApproval }.
  // A failed lookup leaves regInfo null, so the form degrades to the closed
  // invite-only shape and the server still enforces the real gate.
  let regInfo = $state(null)
  let pendingMessage = $state('')

  onMount(async () => {
    try {
      regInfo = await api.registrationInfo()
    } catch {
      // Keep the conservative default; submit errors surface the truth.
    }
  })

  function resetMfa() {
    mfaStep = 'password'
    mfaChallenge = ''
    mfaCode = ''
    mfaRecoveryMode = false
    rememberDevice = false
  }

  function switchMode(next) {
    mode = next
    error = ''
    pendingMessage = ''
    resetMfa()
  }

  function backToPassword() {
    resetMfa()
    error = ''
  }

  // The same post-login sequence for a normal login, a remembered device, or
  // a successful mfa/verify: fetch the me() payload (which carries
  // mfaEnabled) and land on the dashboard.
  async function finishLogin() {
    const u = await api.me()
    setUser(u)
    setCurrentPage('dashboard')
  }

  async function submit() {
    error = ''
    loading = true
    try {
      if (mode === 'login') {
        const res = await api.login(username, password)
        if (res?.status === 'mfa_required' && res.challenge) {
          mfaChallenge = res.challenge
          mfaCode = ''
          mfaRecoveryMode = false
          mfaStep = 'code'
          return
        }
      } else {
        const res = await api.register(
          username,
          email,
          password,
          // Unknown posture (failed lookup) also sends the code: the server
          // ignores it when it is not needed and rejects it when it is.
          regInfo?.mode === 'public' || regInfo?.mode === 'public_approval' ? undefined : inviteCode
        )
        // Pending accounts get no cookies, so there is nothing to sign in to.
        if (res?.status === 'pending') {
          pendingMessage = 'Your account is awaiting approval — the household owner approves new registrations.'
          return
        }
      }
      await finishLogin()
    } catch (err) {
      // Server copy is authoritative ("registration requires a valid invite
      // code", "account is awaiting approval", "username or email already exists").
      error = err.message || 'failed'
    } finally {
      loading = false
    }
  }

  function sanitizeMfaCode() {
    if (!mfaRecoveryMode) mfaCode = mfaCode.replace(/\D/g, '').slice(0, 6)
  }

  function mfaKeydown(e) {
    if (e.key === 'Enter') {
      e.preventDefault()
      verifyMfa()
    }
  }

  async function verifyMfa() {
    error = ''
    if (!mfaCode.trim()) {
      error = mfaRecoveryMode ? 'Enter one of your backup codes.' : 'Enter the 6-digit code from your authenticator app.'
      return
    }
    loading = true
    try {
      await api.mfaVerify(mfaChallenge, mfaCode.trim(), rememberDevice)
      await finishLogin()
    } catch (err) {
      // Keep the challenge: the server leaves it valid until expiry so a
      // mistyped digit is retryable without re-entering the password.
      error = err.message || 'invalid code'
      mfaCode = ''
    } finally {
      loading = false
    }
  }
</script>

<div class="max-w-md mx-auto mt-12">
  <div class="bg-gray-800 p-6 rounded-lg border border-gray-700">
    {#if mfaStep !== 'code'}
      <div class="mb-4 flex space-x-2">
        <button class={mode==='login'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => switchMode('login')}>Login</button>
        <button class={mode==='register'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => switchMode('register')}>Register</button>
      </div>
    {/if}

    {#if pendingMessage}
      <div class="space-y-3">
        <Alert type="success" message={pendingMessage} />
        <p class="text-xs text-gray-400">
          You can close this page. Once approved, sign in with the same username and password.
        </p>
        <button class="btn-primary" onclick={() => switchMode('login')} type="button">
          Back to login
        </button>
      </div>
    {:else if mfaStep === 'code'}
      <div class="space-y-3">
        <div>
          <h2 class="text-emerald-400 font-bold text-lg">Two-step verification</h2>
          <p class="text-xs text-gray-400 mt-1">
            {mfaRecoveryMode
              ? 'Enter one of the single-use backup codes you saved when you enabled 2FA.'
              : 'Enter the 6-digit code from your authenticator app.'}
          </p>
        </div>
        {#if error}
          <Alert type="error" message={error} />
        {/if}
        {#if mfaRecoveryMode}
          <input
            class="input font-mono uppercase"
            placeholder="Backup code"
            autocomplete="one-time-code"
            autocapitalize="characters"
            spellcheck="false"
            bind:value={mfaCode}
            onkeydown={mfaKeydown}
          />
        {:else}
          <input
            class="input font-mono tracking-widest text-center text-xl"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            placeholder="000000"
            bind:value={mfaCode}
            oninput={sanitizeMfaCode}
            onkeydown={mfaKeydown}
          />
        {/if}
        <label class="flex items-center space-x-2 text-sm">
          <input type="checkbox" bind:checked={rememberDevice} />
          <span>Remember this device for 30 days</span>
        </label>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <button class="btn-primary" onclick={verifyMfa} disabled={loading} type="button">
            {loading ? 'Verifying…' : 'Verify'}
          </button>
          <button
            class="text-xs text-emerald-400 underline"
            onclick={() => { mfaRecoveryMode = !mfaRecoveryMode; mfaCode = ''; error = '' }}
            type="button"
          >
            {mfaRecoveryMode ? 'Use an authenticator code instead' : 'Use a backup code instead'}
          </button>
        </div>
        <button class="text-xs text-gray-400 underline" onclick={backToPassword} disabled={loading} type="button">
          ← Back to password
        </button>
      </div>
    {:else}
      {#if error}
        <Alert type="error" message={error} />
      {/if}

      <div class="space-y-3">
        <input class="input" placeholder="Username" bind:value={username} />
        {#if mode === 'register'}
          <input class="input" placeholder="Email" bind:value={email} />
        {/if}
        <input class="input" type="password" placeholder="Password" bind:value={password} />
        {#if mode === 'register' && regInfo?.mode !== 'public' && regInfo?.mode !== 'public_approval'}
          <div>
            <label class="text-xs text-gray-400 block mb-1" for="reg-invite">Invite code <span class="text-emerald-400">*</span></label>
            <input
              class="input font-mono uppercase"
              id="reg-invite"
              placeholder="ABC123XYZ0"
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              bind:value={inviteCode}
            />
            <p class="text-xs text-gray-500 mt-1">
              Ask the household owner for a single-use code. (The very first account on a fresh instance can register without one.)
            </p>
          </div>
        {:else if mode === 'register' && regInfo?.mode === 'public_approval'}
          <p class="text-xs text-gray-400">
            New accounts need approval from the household owner before they can sign in.
          </p>
        {/if}
        <div class="flex items-center justify-between">
          <button class="btn-primary" onclick={submit} disabled={loading}>
            {#if loading}
              <span>Working…</span>
            {:else}
              <span>{mode === 'login' ? 'Login' : 'Register'}</span>
            {/if}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
