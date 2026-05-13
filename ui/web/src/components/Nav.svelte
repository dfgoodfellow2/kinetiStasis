<script>
  import { api } from '../lib/api.js'
  import Brand from './Brand.svelte'
  import Icon from './Icon.svelte'
  import { store, setUser, setCurrentPage } from '../lib/stores.svelte.js'

  async function doLogout() {
    try { await api.logout() } catch {}
    setUser(null)
    setCurrentPage('login')
  }
</script>

<nav class="bg-gray-900 border-b border-gray-800 px-4 py-3">
  <div class="max-w-5xl mx-auto flex items-center justify-between">

    <button class="text-emerald-400 font-bold text-lg cursor-pointer bg-transparent border-none p-0" onclick={() => setCurrentPage('dashboard')} type="button">
      <Brand />
    </button>

    <!-- Desktop nav -->
    <div class="hidden md:flex items-center space-x-1">
      <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'dashboard' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('dashboard')} type="button">Dashboard</button>
      <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'logmeal' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('logmeal')} type="button">Log Meal</button>
      <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'checkin' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('checkin')} type="button">Check-In</button>
      <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'workoutlog' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('workoutlog')} type="button">Workout</button>
      <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'history' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('history')} type="button">History</button>
       <button class={`px-3 py-2 text-sm rounded-lg ${store.currentPage === 'profile' ? 'text-emerald-400 bg-gray-800' : 'text-gray-300 hover:text-white hover:bg-gray-800'}`} onclick={() => setCurrentPage('profile')} type="button"><Icon name="profile" /> Profile</button>

      <button class="ml-2 btn-primary text-sm" onclick={doLogout} type="button">Logout</button>
    </div>

  </div>
</nav>
