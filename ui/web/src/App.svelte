<script>
  import { onMount } from 'svelte'
  import { api } from './lib/api.js'
  import Login from './pages/Login.svelte'
  import Dashboard from './pages/Dashboard.svelte'
  import LogMeal from './pages/LogMeal.svelte'
  import CheckIn from './pages/CheckIn.svelte'
  import WorkoutLog from './pages/WorkoutLog.svelte'
  import History from './pages/History.svelte'
  import WorkoutHistory from './pages/WorkoutHistory.svelte'
  import Profile from './pages/Profile.svelte'
  import BodyFat from './pages/BodyFat.svelte'
  import Measurements from './pages/Measurements.svelte'
  import Export from './pages/Export.svelte'
  import Targets from './pages/Targets.svelte'
  import Guide from './pages/Guide.svelte'
  import Groups from './pages/Groups.svelte'
  import Nav from './components/Nav.svelte'
  import BottomNav from './components/BottomNav.svelte'

  import { store, setUser, setCurrentPage, setUnits, setSleepQualityMax } from './lib/stores.svelte.js'

  let online = $state(typeof navigator === 'undefined' ? true : navigator.onLine)
  let bannerHeight = $state(0)

  onMount(() => {
    const syncOnline = () => { online = navigator.onLine }
    syncOnline()
    window.addEventListener('online', syncOnline)
    window.addEventListener('offline', syncOnline)
    return () => {
      window.removeEventListener('online', syncOnline)
      window.removeEventListener('offline', syncOnline)
    }
  })

  onMount(async () => {
    try {
      const u = await api.me()
      setUser(u)
      try {
        const profile = await api.getProfile()
        setUnits(profile.units)
        setSleepQualityMax(profile.sleepQualityMax)
      } catch {
        // profile not set yet — keep defaults
      }
      setCurrentPage('dashboard')
    } catch {
      setCurrentPage('login')
    }
  })
</script>

{#if !online}
  <div
    bind:clientHeight={bannerHeight}
    class="fixed top-0 left-0 right-0 z-50 bg-amber-500 text-gray-900 text-xs sm:text-sm font-medium text-center px-4 py-2"
    role="status"
    aria-live="polite"
  >
    You're offline — your entries are saved as drafts until you reconnect.
  </div>
  <div style="height: {bannerHeight}px" aria-hidden="true"></div>
{/if}

{#if store.currentPage === 'login' || store.currentPage === 'register'}
  <Login />
{:else}
  <Nav />
  <div class="md:hidden"><BottomNav /></div>
  <main class="max-w-screen-xl mx-auto px-4 py-6 md:pb-6" style="padding-bottom: calc(5rem + env(safe-area-inset-bottom, 0px));">
    {#if store.currentPage === 'dashboard'}
      <Dashboard />
    {:else if store.currentPage === 'groups'}
      <Groups />
    {:else if store.currentPage === 'logmeal'}
      <LogMeal />
    {:else if store.currentPage === 'checkin'}
      <CheckIn />
    {:else if store.currentPage === 'workoutlog'}
      <WorkoutLog />
    {:else if store.currentPage === 'history'}
      <History />
    {:else if store.currentPage === 'workouthistory'}
      <WorkoutHistory />
    {:else if store.currentPage === 'profile'}
      <Profile />
    {:else if store.currentPage === 'bodyfat'}
      <BodyFat />
    {:else if store.currentPage === 'measurements'}
      <Measurements />
    {:else if store.currentPage === 'export'}
      <Export />
    {:else if store.currentPage === 'targets'}
      <Targets />
    {:else if store.currentPage === 'guide'}
      <Guide />
    {/if}
  </main>
{/if}
