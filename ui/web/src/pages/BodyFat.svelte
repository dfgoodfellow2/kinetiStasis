<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { dispWeight, weightUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'

  let data = $state(null)
  let loading = $state(true)

  async function load() {
    loading = true
    try {
      data = await api.getBodyFat('navy')
    } catch {
      data = null
    } finally {
      loading = false
    }
  }

  onMount(load)
</script>

{#if loading}
  <Spinner />
{:else}
  <Card title="Body Fat % — Navy Method">
    {#if data && data.bfPct > 0}
      <div class="space-y-3">
        <div class="text-4xl font-bold text-emerald-400">{(data.bfPct ?? 0).toFixed(1)}<span class="text-lg text-gray-400 ml-1">%</span></div>
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="bg-gray-700 rounded-lg p-3">
            <div class="text-xs text-gray-400 mb-1">Lean Mass</div>
            <div class="text-lg font-semibold text-gray-100">{dispWeight(data.leanMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
          </div>
          <div class="bg-gray-700 rounded-lg p-3">
            <div class="text-xs text-gray-400 mb-1">Fat Mass</div>
            <div class="text-lg font-semibold text-gray-100">{dispWeight(data.fatMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
          </div>
        </div>
        <p class="text-xs text-gray-500">Calculated using the U.S. Navy circumference method from your most recent weight and body measurements.</p>
      </div>
    {:else}
      <div class="space-y-3">
        <p class="text-sm text-gray-300">Not enough data to calculate. The Navy method requires:</p>
        <ul class="text-sm space-y-1 text-gray-400">
          <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Weight logged in a Daily check-in</li>
          <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Neck, Waist &amp; Hips logged in the Measurements tab</li>
          <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Height and Sex set in your Profile</li>
        </ul>
        <p class="text-xs text-gray-500">Once all three are logged, your body fat % will appear here automatically.</p>
      </div>
    {/if}
  </Card>
{/if}
