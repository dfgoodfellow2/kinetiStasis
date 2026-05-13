<script>
  /**
   * Brand.svelte - Centralized brand identity for kinetistasis
   * Single source of truth for app name and branding
   */
  
  import Icon from './Icon.svelte'

  let { 
    variant = 'full', // 'full' | 'icon' | 'text'
    size = 'md' // 'sm' | 'md' | 'lg'
  } = $props();
  
  const sizeClasses = {
    sm: 'text-base',
    md: 'text-lg',
    lg: 'text-2xl'
  }
  
  const sizeClass = $derived(sizeClasses[size] ?? sizeClasses.md);
</script>

{#if variant === 'full'}
  <span class="brand-full inline-flex items-center gap-2 font-bold text-emerald-400 {sizeClass}">
    <Icon name="logo" {size} type="svg" alt="Diet Tracker logo" />
    <span class="brand-name">Kinetistasis</span>
  </span>
{:else if variant === 'icon'}
  <Icon name="logo" {size} type="svg" alt="Diet Tracker logo" />
{:else if variant === 'text'}
  <span class="brand-name font-bold text-emerald-400 {sizeClass}">kinetistasis</span>
{/if}
