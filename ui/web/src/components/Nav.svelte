<script>
  import { api } from '../lib/api.js'
  import { store, setUser, setCurrentPage } from '../lib/stores.svelte.js'
  let mobileOpen = $state(false)

  function goto(p) {
    setCurrentPage(p)
    mobileOpen = false
  }

  async function doLogout() {
    try { await api.logout() } catch {}
    setUser(null)
    setCurrentPage('login')
  }
</script>

<nav class="bg-gray-900 border-b border-gray-800 px-4 py-3">
  <div class="max-w-5xl mx-auto flex items-center justify-between">

    <button class="text-emerald-400 font-bold text-lg cursor-pointer bg-transparent border-none p-0" onclick={() => goto('dashboard')}>
      🥗 Diet Tracker
    </button>

    <!-- Desktop nav -->
    <div class="hidden md:flex items-center space-x-1">
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('dashboard')}>Dashboard</button>
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('logmeal')}>Log Meal</button>
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('checkin')}>Check-In</button>
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('workoutlog')}>Workout</button>
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('history')}>History</button>
      <button class="px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('profile')}>👤 Profile</button>

      <button class="ml-2 btn-primary text-sm" onclick={doLogout}>Logout</button>
    </div>

    <!-- Mobile hamburger -->
    <div class="md:hidden">
      <button class="text-gray-200 text-xl" onclick={() => mobileOpen = !mobileOpen} aria-label="menu">
        {mobileOpen ? '✕' : '☰'}
      </button>
    </div>
  </div>

  <!-- Mobile menu -->
  {#if mobileOpen}
    <div class="md:hidden mt-3 pb-3 border-t border-gray-800 pt-3 space-y-1">
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('dashboard')}>Dashboard</button>
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('logmeal')}>Log Meal</button>
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('checkin')}>Check-In</button>
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('workoutlog')}>Workout</button>
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('history')}>History</button>
      <button class="w-full text-left px-3 py-2 text-gray-300 hover:text-white hover:bg-gray-800 rounded-lg" onclick={() => goto('profile')}>👤 Profile</button>
      <div class="border-t border-gray-800 my-1"></div>
      <button class="w-full btn-primary" onclick={doLogout}>Logout</button>
    </div>
  {/if}
</nav>
