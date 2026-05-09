<script setup>
defineProps({
  command: String
})
</script>

<template>
  <div class="terminal-window">
    <div class="terminal-header">
      <div class="terminal-dot dot-red"></div>
      <div class="terminal-dot dot-yellow"></div>
      <div class="terminal-dot dot-green"></div>
    </div>
    <div class="terminal-content">
      <slot></slot>
    </div>
  </div>
</template>
