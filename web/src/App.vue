<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <!--
  THESIS: CourseForge 是一张正在运作的校园课程节目单，拒绝通用教务表格和圆角 SaaS 仪表盘。
  OWN-WORLD: 暖白纸面、浅色导航、柔和灰蓝与少量陶土色；适中标题、细分隔线、内容流和决策侧栏。
  STORY: 学生先理解课程与时间，再确认名额并选课；视频预览和直播宣讲自然进入同一条决策路径。
  FIRST VIEWPORT: 左侧课程搜索与课程内容流占主位，右侧固定呈现轮次、直播预告与已选摘要，主要动作贴近名额。
  FORM: 校园讲座海报墙 / 课程节目单，候选方向 7，seed b9167015。
  FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
  -->
  <RouterView />
</template>
