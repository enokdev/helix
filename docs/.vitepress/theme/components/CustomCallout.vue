<script setup>
import { computed } from 'vue'

const props = defineProps({
  type: {
    type: String,
    default: 'info' // info, tip, warning, danger
  },
  title: String,
  icon: String
})

const icons = {
  info: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>`,
  tip: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/><path d="M5 3v4"/><path d="M19 17v4"/><path d="M3 5h4"/><path d="M17 19h4"/></svg>`,
  warning: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`,
  danger: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`
}

const activeIcon = computed(() => props.icon || icons[props.type])
</script>

<template>
  <div :class="['custom-callout', `callout-${props.type}`]">
    <div class="callout-icon" v-html="activeIcon"></div>
    <div class="callout-content">
      <strong v-if="props.title">{{ props.title }}</strong>
      <slot></slot>
    </div>
  </div>
</template>

<style scoped>
.callout-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  color: var(--vp-c-brand-1);
}

.callout-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.callout-tip {
  border-left-color: #27c93f;
  background: rgba(39, 201, 63, 0.1);
}
.callout-tip .callout-icon { color: #27c93f; }
.callout-tip strong { color: #27c93f; }

.callout-warning {
  border-left-color: #ffbd2e;
  background: rgba(255, 189, 46, 0.1);
}
.callout-warning .callout-icon { color: #ffbd2e; }
.callout-warning strong { color: #ffbd2e; }

.callout-danger {
  border-left-color: #ff5f56;
  background: rgba(255, 95, 86, 0.1);
}
.callout-danger .callout-icon { color: #ff5f56; }
.callout-danger strong { color: #ff5f56; }
</style>
