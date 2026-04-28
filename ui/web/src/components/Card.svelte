<script>
  let { title = '', children } = $props()
</script>
<div class="bg-gray-800 rounded-xl border border-gray-700 p-4 mb-4">
  {#if title}
    <h2 class="text-lg font-bold text-emerald-400 mb-3">{title}</h2>
  {/if}
  {@render children()}
</div>
