<script setup lang="ts">
import ScheduleWorkspace from '@/features/schedule/ScheduleWorkspace.vue'
</script>

<template>
  <section class="student-page compact-page">
    <header class="page-heading">
      <span>Term schedule</span>
      <h1>本学期课表</h1>
      <p>根据正式选课记录生成。已退课程不会进入课表，未知教学班会单独提示。</p>
    </header>
    <ScheduleWorkspace />
  </section>
</template>
