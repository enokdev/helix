<script setup>
defineProps({
  icon: String, // Can be an emoji (deprecated) or SVG string
  title: String,
  details: String,
  link: String
})
</script>

<template>
  <a :href="link" class="feature-card-wrapper">
    <div class="feature-card">
      <div v-if="icon" class="feature-icon" v-html="icon.startsWith('<svg') ? icon : `<span>${icon}</span>`"></div>
      <div class="feature-title">{{ title }}</div>
      <div class="feature-details">{{ details }}</div>
    </div>
  </a>
</template>

<style scoped>
.feature-card-wrapper {
  text-decoration: none;
  color: inherit;
}

.feature-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--vp-c-brand-1);
}

.feature-icon :deep(svg) {
  width: 100%;
  height: 100%;
}

.feature-icon :deep(span) {
  font-size: 2rem;
}
</style>
