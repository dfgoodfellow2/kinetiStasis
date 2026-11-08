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

  function switchMode(next) {
    mode = next
    error = ''
    pendingMessage = ''
  }

  async function submit() {
    error = ''
    loading = true
    try {
      if (mode === 'login') {
        await api.login(username, password)
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
      const u = await api.me()
      setUser(u)
      setCurrentPage('dashboard')
    } catch (err) {
      // Server copy is authoritative ("registration requires a valid invite
      // code", "account is awaiting approval", "username or email already exists").
      error = err.message || 'failed'
    } finally {
      loading = false
    }
  }
</script>

<div class="max-w-md mx-auto mt-12">
  <div class="bg-gray-800 p-6 rounded-lg border border-gray-700">
    <div class="mb-4 flex space-x-2">
      <button class={mode==='login'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => switchMode('login')}>Login</button>
      <button class={mode==='register'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => switchMode('register')}>Register</button>
    </div>

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
