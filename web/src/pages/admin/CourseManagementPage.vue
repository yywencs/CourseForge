<script setup lang="ts">
import CatalogManager from '@/features/admin/CatalogManager.vue'
</script>

<template>
  <section>
    <header class="admin-page-heading">
      <h1>课程管理</h1>
      <p>维护课程资料、教学班和计划中的选课轮次。</p>
    </header>
    <CatalogManager />
  </section>
</template>

<style scoped>
.admin-page-heading { margin-bottom: 28px; padding-bottom: 24px; border-bottom: 1px solid var(--line); }
</style>
