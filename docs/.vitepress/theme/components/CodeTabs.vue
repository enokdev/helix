<script setup>
import { ref } from 'vue'

const props = defineProps({
  tabs: {
    type: Array,
    required: true // Array of { label: string, key: string }
  }
})

const activeTab = ref(props.tabs[0].key)
</script>

<template>
  <div class="code-tabs">
    <div class="tabs-header">
      <button 
        v-for="tab in tabs" 
        :key="tab.key"
        :class="['tab-btn', { active: activeTab === tab.key }]"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>
    <div class="tabs-content">
      <div v-for="tab in tabs" :key="tab.key" v-show="activeTab === tab.key">
        <slot :name="tab.key"></slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
.code-tabs {
  margin: 1.5rem 0;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  overflow: hidden;
  background: var(--vp-c-bg-soft);
}

.tabs-header {
  display: flex;
  background: var(--vp-c-bg-mute);
  border-bottom: 1px solid var(--vp-c-divider);
  padding: 0 1rem;
}

.tab-btn {
  padding: 0.75rem 1.25rem;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--vp-c-text-2);
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  cursor: pointer;
}

.tab-btn:hover {
  color: var(--vp-c-text-1);
}

.tab-btn.active {
  color: var(--vp-c-brand-1);
  border-bottom-color: var(--vp-c-brand-1);
}

.tabs-content {
  padding: 0;
}

/* Remove default margin from code blocks inside tabs */
.tabs-content :deep(div[class*="language-"]) {
  margin: 0 !important;
  border-radius: 0 !important;
}
</style>
