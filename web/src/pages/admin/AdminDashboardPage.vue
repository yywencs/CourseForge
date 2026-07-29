<script setup lang="ts">
import { Activity, BookOpenCheck, CircleGauge, UsersRound } from '@lucide/vue'

const metrics = [
  { label: '本轮申请', value: '18,642', change: '+12.4%', icon: Activity },
  { label: '选课成功', value: '16,908', change: '90.7%', icon: BookOpenCheck },
  { label: '活跃学生', value: '7,231', change: '82.1%', icon: UsersRound },
  { label: '名额使用率', value: '76.4%', change: '+4.8%', icon: CircleGauge },
]
</script>

<template>
  <section>
    <div class="admin-page-title">
      <div>
        <span>实时态势</span>
        <h1>教务概览</h1>
      </div>
      <el-tag type="warning" effect="plain">框架演示数据</el-tag>
    </div>

    <div class="metric-grid">
      <article v-for="metric in metrics" :key="metric.label" class="metric-card">
        <div><component :is="metric.icon" :size="20" /></div>
        <span>{{ metric.label }}</span>
        <strong>{{ metric.value }}</strong>
        <small>{{ metric.change }} 较昨日</small>
      </article>
    </div>

    <div class="admin-panel">
      <div class="admin-panel__heading">
        <div><span>系统链路</span><h2>选课处理状态</h2></div>
        <el-button>查看监控详情</el-button>
      </div>
      <div class="pipeline-status">
        <div><i class="is-ok" /><span>Redis 原子预占</span><strong>正常</strong></div>
        <div><i class="is-ok" /><span>RabbitMQ Confirm</span><strong>正常</strong></div>
        <div><i class="is-ok" /><span>MySQL 异步落库</span><strong>正常</strong></div>
        <div><i class="is-warning" /><span>待补偿 Stream</span><strong>3 条</strong></div>
      </div>
    </div>
  </section>
</template>
