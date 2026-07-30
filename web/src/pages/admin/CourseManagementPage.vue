<script setup lang="ts">
import { Check, CircleDashed, Database, Film, Route } from '@lucide/vue'

const capabilities = [
  {
    title: '课程与教学班模型',
    detail: 'MySQL 已具备课程、教师、教学班、时间安排和专业范围表。',
    state: 'ready',
  },
  {
    title: '课程目录查询 API',
    detail: '尚未提供 HTTP 查询接口，学生端当前使用展示适配层与教学班 ID 直达。',
    state: 'pending',
  },
  {
    title: '教务课程 CRUD',
    detail: 'Admin 服务已保留 RouteRegistrar 扩展点，尚未注册课程管理模块。',
    state: 'pending',
  },
  {
    title: '课程视频内容',
    detail: 'Outbox 可复用，前端已提供视频预览容器；视频资源与管理 API 待实现。',
    state: 'pending',
  },
]
</script>

<template>
  <section>
    <header class="admin-page-heading">
      <span>Course capability map</span>
      <h1>课程能力</h1>
      <p>这里按真实后端边界展示，不提供不会落库的“新建教学班”假按钮。</p>
    </header>

    <div class="capability-grid">
      <article v-for="item in capabilities" :key="item.title">
        <span :class="`is-${item.state}`">
          <Check v-if="item.state === 'ready'" :size="16" />
          <CircleDashed v-else :size="16" />
        </span>
        <div>
          <h2>{{ item.title }}</h2>
          <p>{{ item.detail }}</p>
        </div>
      </article>
    </div>

    <section class="extension-panel">
      <div class="extension-panel__copy">
        <span>Next backend slice</span>
        <h2>课程内容模块可以沿用现有可靠消息设施</h2>
        <p>
          课程或视频元数据在 MySQL 事务中修改时写入通用 Outbox，后台获得
          RabbitMQ Confirm 后再驱动搜索索引、转码或通知。
        </p>
      </div>
      <div class="extension-flow">
        <div><Database :size="18" /><strong>课程事务</strong><span>MySQL</span></div>
        <Route :size="18" />
        <div><Route :size="18" /><strong>Outbox</strong><span>Confirm</span></div>
        <Route :size="18" />
        <div><Film :size="18" /><strong>视频任务</strong><span>异步处理</span></div>
      </div>
    </section>
  </section>
</template>

<style scoped>
.admin-page-heading {
  margin-bottom: 22px;
}

.admin-page-heading > span,
.extension-panel__copy > span {
  color: #98630c;
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.admin-page-heading h1 {
  margin: 5px 0;
  font-family: var(--font-display);
  font-size: 36px;
}

.admin-page-heading p,
.extension-panel__copy p {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.6;
}

.capability-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.capability-grid article {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 13px;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: white;
}

.capability-grid article > span {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 9px;
  color: var(--warning);
  background: #fff0c7;
}

.capability-grid article > span.is-ready {
  color: var(--success);
  background: #d9f3e9;
}

.capability-grid h2 {
  margin: 0 0 5px;
  font-size: 14px;
}

.capability-grid p {
  margin: 0;
  color: var(--muted);
  font-size: 10px;
  line-height: 1.6;
}

.extension-panel {
  display: grid;
  grid-template-columns: minmax(240px, 0.8fr) minmax(460px, 1.2fr);
  gap: 28px;
  margin-top: 16px;
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: white;
}

.extension-panel__copy h2 {
  margin: 6px 0 9px;
  font-family: var(--font-display);
  font-size: 25px;
}

.extension-flow {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.extension-flow > div {
  display: grid;
  min-width: 110px;
  justify-items: center;
  gap: 6px;
  padding: 17px;
  border-radius: 11px;
  color: var(--brand);
  background: var(--surface-muted);
}

.extension-flow > svg {
  color: #a0b4ae;
}

.extension-flow strong {
  font-size: 11px;
}

.extension-flow span {
  color: var(--muted);
  font-size: 9px;
}

@media (max-width: 950px) {
  .extension-panel {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 650px) {
  .capability-grid {
    grid-template-columns: 1fr;
  }

  .extension-flow {
    align-items: stretch;
    flex-direction: column;
  }

  .extension-flow > svg {
    align-self: center;
    transform: rotate(90deg);
  }
}
</style>
