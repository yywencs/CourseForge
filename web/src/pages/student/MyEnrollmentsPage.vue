<script setup lang="ts">
import { CheckCircle2, Clock3, Radio } from '@lucide/vue'
</script>

<template>
  <section class="student-page compact-page">
    <div class="page-title">
      <span>选课记录</span>
      <h1>我的选课</h1>
      <p>Redis 已完成决策后即可看到结果，MySQL 落库状态会异步更新。</p>
    </div>

    <div class="timeline-card">
      <div class="timeline-card__item is-complete">
        <CheckCircle2 :size="20" />
        <div>
          <strong>分布式系统设计</strong>
          <span>选课成功 · 3.5 学分</span>
        </div>
        <time>刚刚</time>
      </div>
      <div class="timeline-card__item is-processing">
        <Radio :size="20" />
        <div>
          <strong>智能交互产品实践</strong>
          <span>正在确认申请结果</span>
        </div>
        <time>10:42</time>
      </div>
      <div class="timeline-card__item">
        <Clock3 :size="20" />
        <div>
          <strong>选课结果查询接口待接入</strong>
          <span>后续由 application_id 查询最终持久化状态</span>
        </div>
      </div>
    </div>
  </section>
</template>
