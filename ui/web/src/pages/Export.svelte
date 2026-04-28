<script>
  import { copyToClipboard, today, daysAgo } from '../lib/utils.js'
  import { api } from '../lib/api.js'
  import Alert from '../components/Alert.svelte'

  let from = $state(daysAgo(7))
  let to = $state(today())
  let format = $state('md')
  let content = $state('')
  let error = $state('')
  let success = $state('')
  let copied = $state(false)

  async function exportNutrition() {
    error = ''
    try {
      const res = await api.exportNutrition(from, to, format)
      content = res.content || ''
    } catch (e) {
      error = e.message
    }
  }

  async function exportWorkouts() {
    error = ''
    try {
      const res = await api.exportWorkouts(from, to, format)
      content = res.content || ''
    } catch (e) {
      error = e.message
    }
  }

  async function exportCombined() {
    error = ''
    try {
      const res = await api.exportCombined(from, to)
      content = res.content || ''
    } catch (e) {
      error = e.message
    }
  }

  async function doCopy() {
    const ok = await copyToClipboard(content)
    if (ok) {
      copied = true
      setTimeout(() => copied = false, 2000)
    } else {
      error = 'Copy failed'
      setTimeout(() => error = '', 2000)
    }
  }

  function download(name) {
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if success}<Alert type="success" message={success} />{/if}

<div class="max-w-2xl mx-auto space-y-4">
  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">Nutrition Export</h3>
    <input class="input" type="date" bind:value={from} />
    <input class="input" type="date" bind:value={to} />
    <select class="input" bind:value={format}><option value="md">Markdown</option><option value="csv">CSV</option></select>
      <div class="flex space-x-2 mt-2">
      <button class="btn-primary" onclick={exportNutrition}>Export</button>
      <button class="btn-secondary" onclick={() => { content = ''; }}>Clear</button>
    </div>
  </div>

  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">Workouts Export</h3>
    <input class="input" type="date" bind:value={from} />
    <input class="input" type="date" bind:value={to} />
    <select class="input" bind:value={format}><option value="md">Markdown</option><option value="csv">CSV</option></select>
      <div class="flex space-x-2 mt-2">
      <button class="btn-primary" onclick={exportWorkouts}>Export</button>
    </div>
  </div>

  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">Combined Export</h3>
    <input class="input" type="date" bind:value={from} />
    <input class="input" type="date" bind:value={to} />
      <div class="flex space-x-2 mt-2">
      <button class="btn-primary" onclick={exportCombined}>Export</button>
      <button class="btn-secondary" onclick={() => { if (content) download('export.md') }}>Download</button>
      <button class="btn-secondary" class:btn-success={copied} onclick={doCopy} disabled={!content}>{copied ? '✓ Copied!' : 'Copy'}</button>
    </div>
  </div>

  <div>
    <pre class="whitespace-pre-wrap bg-gray-800 p-3 rounded border border-gray-700">{content}</pre>
  </div>
</div>
