<script setup lang="ts">
const rows = [
  { code: 'CS-304', name: '分布式系统设计', teacher: '周屿', capacity: 60, selected: 42, state: '开放中' },
  { code: 'AI-217', name: '智能交互产品实践', teacher: '许南乔', capacity: 40, selected: 36, state: '开放中' },
  { code: 'HUM-109', name: '影像叙事与当代文化', teacher: '陈见微', capacity: 80, selected: 80, state: '已满' },
]
</script>

<template>
  <section>
    <div class="admin-page-title">
      <div><span>教学资源</span><h1>课程与教学班</h1></div>
      <el-button type="primary">新建教学班</el-button>
    </div>

    <div class="admin-panel">
      <div class="table-toolbar">
        <el-input placeholder="搜索课程或教师" clearable />
        <el-select placeholder="全部状态"><el-option label="开放中" value="open" /></el-select>
      </div>
      <el-table :data="rows">
        <el-table-column prop="code" label="课程代码" width="130" />
        <el-table-column prop="name" label="课程名称" min-width="210" />
        <el-table-column prop="teacher" label="授课教师" width="120" />
        <el-table-column label="容量" width="150">
          <template #default="{ row }">{{ row.selected }} / {{ row.capacity }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.state === '已满' ? 'danger' : 'success'" effect="plain">{{ row.state }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140">
          <template #default><el-button link type="primary">配置</el-button></template>
        </el-table-column>
      </el-table>
    </div>
  </section>
</template>
