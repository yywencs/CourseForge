<script setup lang="ts">
const rows = [
  { id: '019D…001', student: '2026001001', course: '分布式系统设计', state: 'selected', time: '10:42:16' },
  { id: '019D…002', student: '2026001842', course: '智能交互产品实践', state: 'processing', time: '10:42:15' },
  { id: '019D…003', student: '2026000921', course: '影像叙事与当代文化', state: 'rejected', time: '10:42:13' },
]

function stateType(state: string) {
  if (state === 'selected') return 'success'
  if (state === 'processing') return 'warning'
  return 'danger'
}
</script>

<template>
  <section>
    <div class="admin-page-title">
      <div><span>异步处理链路</span><h1>选课申请监控</h1></div>
      <el-button>导出记录</el-button>
    </div>

    <div class="admin-panel">
      <div class="table-toolbar">
        <el-input placeholder="申请单号 / 学号" clearable />
        <el-select placeholder="全部结果"><el-option label="选课成功" value="selected" /></el-select>
      </div>
      <el-table :data="rows">
        <el-table-column prop="id" label="申请单" width="150" />
        <el-table-column prop="student" label="学号" width="150" />
        <el-table-column prop="course" label="课程" min-width="220" />
        <el-table-column label="处理状态" width="140">
          <template #default="{ row }">
            <el-tag :type="stateType(row.state)" effect="plain">{{ row.state }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="time" label="申请时间" width="140" />
      </el-table>
    </div>
  </section>
</template>
