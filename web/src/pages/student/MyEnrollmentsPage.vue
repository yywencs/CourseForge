<script setup lang="ts">
import EnrollmentWorkspace from '@/features/enrollment/EnrollmentWorkspace.vue'
</script>

<template>
  <section class="student-page compact-page">
    <header class="page-heading">
      <span>Selection records</span>
      <h1>我的选课</h1>
      <p>申请结果、正式选课和候补队列来自后端真实状态；异步落库期间会持续轮询。</p>
    </header>
    <EnrollmentWorkspace />
  </section>
</template>
