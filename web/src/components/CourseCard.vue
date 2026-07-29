<script setup lang="ts">
import { Clock3, MapPin, PlayCircle, UserRound } from '@lucide/vue'
import { computed } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const props = defineProps<{
  course: TeachingClassSummary
  submitting?: boolean
}>()

defineEmits<{
  select: [course: TeachingClassSummary]
}>()

const remaining = computed(
  () => props.course.capacity - props.course.selectedCount,
)
const occupancy = computed(
  () => Math.round((props.course.selectedCount / props.course.capacity) * 100),
)
</script>

<template>
  <article class="course-card">
    <div class="course-card__topline">
      <span class="course-code">{{ course.courseCode }}</span>
      <span v-if="course.hasVideo" class="video-badge">
        <PlayCircle :size="14" />有介绍视频
      </span>
    </div>

    <div>
      <h3>{{ course.courseName }}</h3>
      <p>{{ course.introduction }}</p>
    </div>

    <div class="course-tags">
      <span v-for="tag in course.tags" :key="tag">{{ tag }}</span>
    </div>

    <dl class="course-meta">
      <div><dt><UserRound :size="15" /></dt><dd>{{ course.teacherName }}</dd></div>
      <div><dt><Clock3 :size="15" /></dt><dd>{{ course.schedule }}</dd></div>
      <div><dt><MapPin :size="15" /></dt><dd>{{ course.location }}</dd></div>
    </dl>

    <div class="seat-progress">
      <div>
        <span>剩余 {{ remaining }} 个名额</span>
        <span>{{ occupancy }}%</span>
      </div>
      <div class="seat-progress__track">
        <i :style="{ width: `${occupancy}%` }" />
      </div>
    </div>

    <div class="course-card__actions">
      <span><strong>{{ course.credits }}</strong> 学分</span>
      <button
        type="button"
        :disabled="remaining <= 0 || submitting"
        @click="$emit('select', course)"
      >
        {{ submitting ? '正在提交…' : remaining > 0 ? '选择这门课' : '名额已满' }}
      </button>
    </div>
  </article>
</template>
