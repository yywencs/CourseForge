<script setup lang="ts">
const weekdays = ['周一', '周二', '周三', '周四', '周五']
</script>

<template>
  <section class="student-page compact-page">
    <div class="page-title">
      <span>2026—2027 秋季学期</span>
      <h1>本学期课表</h1>
      <p>用清晰的时间冲突提示辅助学生在提交前完成选择。</p>
    </div>

    <div class="schedule-grid">
      <div class="schedule-grid__corner">节次</div>
      <div v-for="day in weekdays" :key="day" class="schedule-grid__head">{{ day }}</div>
      <template v-for="period in 4" :key="period">
        <div class="schedule-grid__period">{{ period * 2 - 1 }}–{{ period * 2 }}节</div>
        <div v-for="day in weekdays" :key="`${period}-${day}`" class="schedule-grid__cell">
          <article v-if="period === 2 && day === '周二'" class="schedule-course">
            <strong>分布式系统设计</strong>
            <span>格物楼 A308</span>
          </article>
          <article v-else-if="period === 4 && day === '周四'" class="schedule-course is-mint">
            <strong>智能交互产品实践</strong>
            <span>创新中心 C201</span>
          </article>
        </div>
      </template>
    </div>
  </section>
</template>
