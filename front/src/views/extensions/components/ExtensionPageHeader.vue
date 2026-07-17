<template>
  <header class="extension-page-header">
    <div class="heading-group">
      <div class="title-line">
        <nav aria-label="扩展页面层级">
          <ol class="breadcrumb-list">
            <li><RouterLink class="center-link" to="/extensions">扩展中心</RouterLink></li>
            <li class="separator" aria-hidden="true">/</li>
            <li class="current-title" aria-current="page"><h1>{{ title }}</h1></li>
          </ol>
        </nav>
        <slot name="title-extra" />
      </div>
      <p v-if="description" class="description">{{ description }}</p>
      <div v-if="$slots.meta" class="meta"><slot name="meta" /></div>
    </div>
    <div v-if="$slots.actions" class="header-actions"><slot name="actions" /></div>
  </header>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  description?: string
}>()
</script>

<style scoped>
.extension-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  min-width: 0;
}

.heading-group {
  min-width: 0;
  flex: 1;
}

.title-line {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.title-line nav {
  min-width: 0;
}

.breadcrumb-list {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  margin: 0;
  padding: 0;
  list-style: none;
}

h1,
.center-link,
.separator {
  font-size: 24px;
  line-height: 32px;
}

h1 {
  margin: 0;
  color: var(--ac-color-text);
  font-weight: 600;
}

.center-link {
  position: relative;
  display: inline-flex;
  align-items: center;
  color: var(--ac-color-primary);
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  transition: color 180ms ease;
}

.center-link::before {
  position: absolute;
  inset: -6px 0;
  content: "";
}

.center-link:hover {
  color: var(--el-color-primary-light-3);
  text-decoration: underline;
  text-underline-offset: 4px;
}

.center-link:focus-visible {
  border-radius: 4px;
  outline: 3px solid var(--ac-color-primary);
  outline-offset: 3px;
}

.separator {
  color: var(--ac-color-text-muted);
  font-weight: 400;
}

.current-title {
  min-width: 0;
  overflow-wrap: anywhere;
}

.description,
.meta {
  margin: 6px 0 0;
}

.description {
  color: var(--ac-color-text-secondary);
  line-height: 1.6;
}

.meta {
  color: var(--ac-color-text-muted);
}

.header-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  min-height: 44px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .extension-page-header {
    align-items: stretch;
    flex-direction: column;
    gap: 12px;
  }

  .breadcrumb-list {
    flex-wrap: wrap;
  }

  .header-actions {
    justify-content: flex-start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .center-link {
    transition: none;
  }
}
</style>
