<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  state: string
}>()

const labels: Record<string, string> = {
  created: '申请已创建',
  reserved: '名额已锁定',
  selected: '选课成功',
  rejected: '选课未通过',
  cancelled: '已取消',
  enrolled: '修读中',
  dropped: '已退课',
  completed: '已完成',
  waiting: '候补中',
  promoting: '正在晋级',
  promoted: '已晋级',
}

const tone = computed(() => {
  if (['selected', 'enrolled', 'promoted', 'completed'].includes(props.state)) {
    return 'success'
  }
  if (['created', 'reserved', 'waiting', 'promoting'].includes(props.state)) {
    return 'warning'
  }
  if (['rejected', 'cancelled', 'dropped'].includes(props.state)) {
    return 'neutral'
  }
  return 'neutral'
})
</script>

<template>
  <span class="state-badge" :class="`is-${tone}`">
    <i />
    {{ labels[state] ?? state }}
  </span>
</template>

<style scoped>
.state-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  width: fit-content;
  padding: 5px 9px;
  border-radius: 999px;
  color: #52635e;
  background: #eef2f0;
  font-size: 11px;
  font-weight: 700;
}

.state-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentcolor;
}

.state-badge.is-success {
  color: #0b7458;
  background: #daf4ea;
}

.state-badge.is-warning {
  color: #8b5a08;
  background: #fff0c9;
}
</style>
