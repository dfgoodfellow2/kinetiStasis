<script>
  let { 
    src, 
    alt = '', 
    size = '24', 
    class: className = '', 
    color = 'currentColor',
    ...rest 
  } = $props();
</script>

<img 
  {src} 
  alt={alt || ''}
  width={size} 
  height={size}
  class={"inline-block flex-shrink-0 " + (className || '')}
  style={"color: " + color}
  aria-hidden={alt ? undefined : 'true'}
  {...rest}
/> 
