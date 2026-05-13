<script>
  /**
   * Icon.svelte - Central icon system with semantic naming
   * Maps emoji/icons to semantic names for consistent usage
   */
  
  import SvgIcon from './SvgIcon.svelte';
  import { getIconPath, loadIcon } from '../lib/icons';

  let {
    name = '', // 'logo' | 'home' | 'log-meal' | 'checkin' | 'history' | 'workout' | 'profile'
    size = 'md', // 'sm' | 'md' | 'lg' | 'xl'
    type = 'emoji', // 'emoji' | 'svg'
    alt = '',
    class: className = ''
  } = $props();
  
  const sizeMap = {
    sm: 'text-sm',
    md: 'text-base',
    lg: 'text-xl',
    xl: 'text-2xl'
  };

  const svgSizeMap = {
    sm: '16',
    md: '24',
    lg: '28',
    xl: '32'
  };
  
  const iconMap = {
    // Keep existing emoji mappings
    logo: '🥗',
    home: '🏠',
    'log-meal': '➕',
    checkin: '✅',
    history: '📊',
    workout: '🏋️',
    profile: '👤'
  };

  // removed svgIconMap (dead code)

  const sizeClass = $derived(sizeMap[size] ?? sizeMap.md);
  const icon = $derived(iconMap[name] ?? '');
  let svgSrc = $state(null);

  // Load SVG URL dynamically when requested
  async function updateSvg() {
    if (type === 'svg' && name) {
      const path = getIconPath(name);
      if (path) {
        try {
          const mod = await loadIcon(name);
          svgSrc = (mod && mod.default) || null;
        } catch (e) {
          console.warn(`Failed to load SVG icon: ${name}`, e);
          svgSrc = null;
        }
      } else {
        svgSrc = null;
      }
    } else {
      svgSrc = null;
    }
  }

  $effect(() => {
    // Access reactive values directly so Svelte tracks them
    if (type === 'svg' && name) {
      updateSvg();
    } else {
      svgSrc = null;
    }
  });
</script>

{#if type === 'svg' && svgSrc}
  <SvgIcon src={svgSrc} alt={alt || name} size={svgSizeMap[size] || '24'} class={className} />
{:else}
  <span class="{sizeClass} {className}" aria-hidden="true">{icon || '❓'}</span>
{/if}
