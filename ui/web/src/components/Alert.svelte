<script>
  let { type = 'error', message = '' } = $props()
  const classes = {
    error: 'bg-red-900/50 border-red-700 text-red-300',
    success: 'bg-emerald-900/50 border-emerald-700 text-emerald-300',
    warning: 'bg-yellow-900/50 border-yellow-700 text-yellow-300',
  }
</script>
{#if message}
  <div class={"border rounded-lg px-4 py-3 mb-4 text-sm " + (classes[type] || classes.error)}>
    {message}
  </div>
{/if}
