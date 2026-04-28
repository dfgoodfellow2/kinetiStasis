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
  let error = $state('')
  let loading = $state(false)

  async function submit() {
    error = ''
    loading = true
    try {
      if (mode === 'login') {
        await api.login(username, password)
      } else {
        await api.register(username, email, password)
      }
      const u = await api.me()
      setUser(u)
      setCurrentPage('dashboard')
    } catch (err) {
      error = err.message || 'failed'
    } finally {
      loading = false
    }
  }
</script>

<div class="max-w-md mx-auto mt-12">
  <div class="bg-gray-800 p-6 rounded-lg border border-gray-700">
    <div class="mb-4 flex space-x-2">
      <button class={mode==='login'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => mode = 'login'}>Login</button>
      <button class={mode==='register'? 'px-3 py-1 rounded bg-emerald-600' : 'px-3 py-1 rounded bg-gray-700'} onclick={() => mode = 'register'}>Register</button>
    </div>

    {#if error}
      <Alert type="error" message={error} />
    {/if}

    <div class="space-y-3">
      <input class="input" placeholder="Username" bind:value={username} />
      {#if mode === 'register'}
        <input class="input" placeholder="Email" bind:value={email} />
      {/if}
      <input class="input" type="password" placeholder="Password" bind:value={password} />
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
  </div>
</div>
