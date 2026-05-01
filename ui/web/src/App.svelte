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
  import Nav from './components/Nav.svelte'

  import { store, setUser, setCurrentPage, setUnits, setSleepQualityMax } from './lib/stores.svelte.js'
  onMount(async () => {
    try {
      const u = await api.me()
      setUser(u)
      try {
        const profile = await api.getProfile()
        setUnits(profile.units)
        setSleepQualityMax(profile.sleep_quality_max)
      } catch {
        // profile not set yet — keep defaults
      }
      setCurrentPage('dashboard')
    } catch {
      setCurrentPage('login')
    }
  })
</script>

{#if store.currentPage === 'login' || store.currentPage === 'register'}
  <Login />
{:else}
  <Nav />
  <main class="max-w-screen-xl mx-auto px-4 py-6">
    {#if store.currentPage === 'dashboard'}
      <Dashboard />
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
    {/if}
  </main>
{/if}
