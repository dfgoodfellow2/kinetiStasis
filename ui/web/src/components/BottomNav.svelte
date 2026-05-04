<script>
  import { store, setCurrentPage } from '../lib/stores.svelte.js'
</script>

<nav class="fixed bottom-0 left-0 right-0 z-50 bg-gray-900 border-t border-gray-800" style="padding-bottom: env(safe-area-inset-bottom);">
  <div class="flex justify-around items-center h-16">
    <!-- Dashboard -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'dashboard' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('dashboard')}
      aria-label="Dashboard"
      aria-current={store.currentPage === 'dashboard' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">🏠</div>
      <div class="text-xs">Dashboard</div>
    </button>

    <!-- Log Meal -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'logmeal' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('logmeal')}
      aria-label="Log Meal"
      aria-current={store.currentPage === 'logmeal' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">➕</div>
      <div class="text-xs">Log</div>
    </button>

    <!-- Check-In -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'checkin' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('checkin')}
      aria-label="Check-In"
      aria-current={store.currentPage === 'checkin' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">✅</div>
      <div class="text-xs">Check-In</div>
    </button>

    <!-- History -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'history' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('history')}
      aria-label="History"
      aria-current={store.currentPage === 'history' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">📊</div>
      <div class="text-xs">History</div>
    </button>

    <!-- Workout -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'workoutlog' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('workoutlog')}
      aria-label="Workout"
      aria-current={store.currentPage === 'workoutlog' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">🏋️</div>
      <div class="text-xs">Workout</div>
    </button>

    <!-- Profile -->
    <button
      class={`flex-1 flex flex-col items-center justify-center min-h-[44px] min-w-[44px] ${store.currentPage === 'profile' ? 'text-emerald-400 border-t-2 border-emerald-400' : 'text-gray-400'}`}
      onclick={() => setCurrentPage('profile')}
      aria-label="Profile"
      aria-current={store.currentPage === 'profile' ? 'page' : undefined}
      type="button"
    >
      <div class="text-xl">👤</div>
      <div class="text-xs">Profile</div>
    </button>
  </div>
</nav>
