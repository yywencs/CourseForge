<script setup lang="ts">
import {
  Clock3,
  MapPin,
  PlayCircle,
  UserRound,
  UsersRound,
} from '@lucide/vue'
import { computed } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const props = defineProps<{
  course: TeachingClassSummary
  submitting?: boolean
  joiningWaitlist?: boolean
  selected?: boolean
  waitlisted?: boolean
}>()

const emit = defineEmits<{
  select: [course: TeachingClassSummary]
  joinWaitlist: [course: TeachingClassSummary]
  playVideo: [course: TeachingClassSummary]
}>()

const remaining = computed(() =>
  Math.max(0, props.course.capacity - props.course.selectedCount),
)
const occupancy = computed(() =>
  props.course.capacity > 0
    ? Math.min(
        100,
        Math.round(
          (props.course.selectedCount / props.course.capacity) * 100,
        ),
      )
    : 100,
)
const primaryLabel = computed(() => {
  if (props.selected) return '已在选课记录中'
  if (props.waitlisted) return '已加入候补'
  if (props.submitting) return '正在锁定名额…'
  if (props.joiningWaitlist) return '正在加入候补…'
  return remaining.value > 0 ? '选择这门课' : '加入候补'
})

function handlePrimaryAction(): void {
  if (
    props.selected ||
    props.waitlisted ||
    props.submitting ||
    props.joiningWaitlist
  ) {
    return
  }
  if (remaining.value > 0) {
    emit('select', props.course)
    return
  }
  emit('joinWaitlist', props.course)
}
</script>

<template>
  <article class="course-card" data-testid="course-card">
    <div class="course-card__topline">
      <span class="course-code">{{ course.courseCode }}</span>
      <button
        v-if="course.hasVideo"
        class="video-button"
        type="button"
        @click="emit('playVideo', course)"
      >
        <PlayCircle :size="15" />
        课程预览
      </button>
    </div>

    <div class="course-card__copy">
      <h3>{{ course.courseName }}</h3>
      <p>{{ course.introduction }}</p>
    </div>

    <div class="course-tags" aria-label="课程标签">
      <span v-for="tag in course.tags" :key="tag">{{ tag }}</span>
    </div>

    <dl class="course-meta">
      <div>
        <dt><UserRound :size="16" /></dt>
        <dd>{{ course.teacherName }}</dd>
      </div>
      <div>
        <dt><Clock3 :size="16" /></dt>
        <dd>{{ course.schedule }}</dd>
      </div>
      <div>
        <dt><MapPin :size="16" /></dt>
        <dd>{{ course.location }}</dd>
      </div>
    </dl>

    <div class="seat-meter">
      <div class="seat-meter__labels">
        <span><UsersRound :size="14" /> {{ course.selectedCount }}/{{ course.capacity }}</span>
        <strong :class="{ 'is-full': remaining === 0 }">
          {{ remaining > 0 ? `余 ${remaining}` : '已满' }}
        </strong>
      </div>
      <div class="seat-meter__track" aria-hidden="true">
        <i :style="{ width: `${occupancy}%` }" />
      </div>
    </div>

    <div class="course-card__actions">
      <span><strong>{{ course.credits }}</strong> 学分</span>
      <button
        class="primary-action"
        type="button"
        :class="{ 'is-waitlist': remaining === 0 && !selected }"
        :disabled="selected || waitlisted || submitting || joiningWaitlist"
        data-testid="course-primary-action"
        @click="handlePrimaryAction"
      >
        {{ primaryLabel }}
      </button>
    </div>
  </article>
</template>

<style scoped>
.course-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 20px;
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: var(--radius-card);
  background: var(--surface);
  box-shadow: var(--shadow-card);
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    box-shadow 180ms ease;
}

.course-card:hover {
  transform: translateY(-4px);
  border-color: #bad7cf;
  box-shadow: 0 22px 46px rgba(16, 54, 45, 0.12);
}

.course-card__topline,
.course-card__actions,
.seat-meter__labels {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.course-code {
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 750;
  letter-spacing: 0.04em;
}

.video-button {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0;
  border: 0;
  color: #8d5a08;
  background: transparent;
  font-size: 12px;
  cursor: pointer;
}

.course-card__copy h3 {
  margin: 0 0 9px;
  font-family: var(--font-display);
  font-size: 24px;
  line-height: 1.28;
}

.course-card__copy p {
  min-height: 62px;
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.65;
}

.course-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.course-tags span {
  padding: 5px 8px;
  border-radius: 7px;
  color: #315f53;
  background: #eaf4f0;
  font-size: 11px;
}

.course-meta {
  display: grid;
  gap: 9px;
  margin: 0;
}

.course-meta div {
  display: flex;
  gap: 9px;
  color: #3d514c;
  font-size: 13px;
}

.course-meta dt {
  color: #7c918b;
}

.course-meta dd {
  margin: 0;
}

.seat-meter {
  display: grid;
  gap: 8px;
}

.seat-meter__labels {
  color: var(--muted);
  font-size: 11px;
}

.seat-meter__labels span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.seat-meter__labels strong {
  color: var(--success);
}

.seat-meter__labels strong.is-full {
  color: var(--danger);
}

.seat-meter__track {
  overflow: hidden;
  height: 6px;
  border-radius: 999px;
  background: #e3ece8;
}

.seat-meter__track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--brand), var(--brand-bright));
}

.course-card__actions {
  margin-top: auto;
  padding-top: 18px;
  border-top: 1px solid var(--line-soft);
}

.course-card__actions > span {
  color: var(--muted);
  font-size: 12px;
}

.course-card__actions strong {
  color: var(--ink);
  font-family: var(--font-display);
  font-size: 25px;
}

.primary-action {
  padding: 11px 15px;
  border: 1px solid var(--brand);
  border-radius: 11px;
  color: white;
  background: var(--brand);
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.primary-action.is-waitlist {
  border-color: #d8a33a;
  color: #6b4300;
  background: #ffdfa0;
}

.primary-action:disabled {
  border-color: #d9e1de;
  color: #7f8d89;
  background: #e8eeeb;
  cursor: not-allowed;
}

@media (prefers-reduced-motion: reduce) {
  .course-card {
    transition: none;
  }
}
</style>
