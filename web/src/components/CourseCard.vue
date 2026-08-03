<script setup lang="ts">
import { Clock3, MapPin, Play, UserRound, UsersRound } from '@lucide/vue'
import { computed } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const props = withDefaults(defineProps<{
  course: TeachingClassSummary
  index?: number
  submitting?: boolean
  joiningWaitlist?: boolean
  selected?: boolean
  waitlisted?: boolean
}>(), { index: 1 })

const emit = defineEmits<{
  select: [course: TeachingClassSummary]
  joinWaitlist: [course: TeachingClassSummary]
  playVideo: [course: TeachingClassSummary]
}>()

const remaining = computed(() => Math.max(0, props.course.capacity - props.course.selectedCount))
const occupancy = computed(() => props.course.capacity > 0
  ? Math.min(100, Math.round((props.course.selectedCount / props.course.capacity) * 100))
  : 100,
)
const primaryLabel = computed(() => {
  if (props.selected) return '已在选课记录中'
  if (props.waitlisted) return '已加入候补'
  if (props.submitting) return '正在提交…'
  if (props.joiningWaitlist) return '正在加入候补…'
  return remaining.value > 0 ? '选择这门课' : '加入候补'
})

function handlePrimaryAction(): void {
  if (props.selected || props.waitlisted || props.submitting || props.joiningWaitlist) return
  if (remaining.value > 0) {
    emit('select', props.course)
    return
  }
  emit('joinWaitlist', props.course)
}
</script>

<template>
  <article class="course-card" data-testid="course-card">
    <div class="course-index">
      {{ String(index).padStart(2, '0') }}
    </div>

    <div class="course-body">
      <header>
        <span class="course-code">{{ course.courseCode }}</span>
        <div class="course-tags" aria-label="课程标签">
          <span v-for="tag in course.tags" :key="tag">{{ tag }}</span>
        </div>
      </header>
      <h3>{{ course.courseName }}</h3>
      <p>{{ course.introduction }}</p>
      <dl class="course-meta">
        <div><dt><UserRound :size="15" /></dt><dd>{{ course.teacherName }}</dd></div>
        <div><dt><Clock3 :size="15" /></dt><dd>{{ course.schedule }}</dd></div>
        <div><dt><MapPin :size="15" /></dt><dd>{{ course.location }}</dd></div>
      </dl>
    </div>

    <button v-if="course.hasVideo && course.videoId" class="course-media" type="button" @click="emit('playVideo', course)">
      <span><Play :size="20" fill="currentColor" /></span>
      <strong>课程预览</strong>
      <small>了解课程内容</small>
    </button>
    <div v-else class="course-media is-empty" aria-label="暂无课程预览">
      <span>{{ course.courseCode.slice(0, 2) }}</span>
      <strong>暂无预览</strong>
      <small>以课程信息为准</small>
    </div>

    <div class="course-decision">
      <div class="credit"><strong>{{ course.credits }}</strong><span>学分</span></div>
      <div class="seat-status">
        <span><UsersRound :size="14" />已选 {{ course.selectedCount }} / {{ course.capacity }}</span>
        <strong :class="{ 'is-full': remaining === 0 }">
          {{ remaining > 0 ? `剩余 ${remaining} 个名额` : '名额已满' }}
        </strong>
        <div aria-hidden="true"><i :style="{ width: `${occupancy}%` }" /></div>
      </div>
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
  display: grid;
  min-width: 0;
  grid-template-columns: 76px minmax(260px, 1fr) 120px 170px;
  border-bottom: 1px solid var(--line);
  background: var(--surface);
  transition: background 180ms ease-out;
}

.course-card:last-child {
  border-bottom: 0;
}

.course-card:hover {
  background: #faf7f0;
}

.course-index {
  display: grid;
  place-items: start center;
  padding-top: 21px;
  border-right: 1px solid var(--line);
  color: var(--brand);
  font-family: var(--font-display);
  font-size: 32px;
  font-variation-settings: "wdth" 72, "wght" 680;
  letter-spacing: -0.035em;
}

.course-body {
  min-width: 0;
  padding: 20px;
}

.course-body header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.course-code {
  color: var(--brand);
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.04em;
}

.course-tags {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
}

.course-tags span {
  padding: 3px 6px;
  border: 1px solid var(--line);
  border-radius: 5px;
  color: var(--muted);
  font-size: 9px;
}

.course-body h3 {
  margin: 8px 0 7px;
  font-family: var(--font-display);
  font-size: clamp(21px, 2vw, 27px);
  font-variation-settings: "wdth" 82, "wght" 660;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.course-body > p {
  max-width: 60ch;
  margin: 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.65;
}

.course-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  margin: 15px 0 0;
}

.course-meta div {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--ink-soft);
  font-size: 10px;
}

.course-meta dt {
  color: var(--muted);
}

.course-meta dd {
  margin: 0;
}

.course-media {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 5px;
  margin: 16px 0;
  padding: 12px;
  border: 0;
  border-left: 1px solid var(--line-soft);
  border-right: 1px solid var(--line-soft);
  color: var(--brand);
  background: var(--brand-pale);
  cursor: pointer;
}

.course-media:hover {
  background: #e2eaf1;
}

.course-media > span {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid currentColor;
  border-radius: 50%;
}

.course-media strong {
  font-size: 11px;
}

.course-media small {
  color: var(--muted);
  font-size: 8px;
}

.course-media.is-empty {
  color: var(--muted);
  background: var(--surface-muted);
  cursor: default;
}

.course-media.is-empty > span {
  border: 0;
  border-radius: 0;
  color: var(--ink);
  font-family: var(--font-display);
  font-size: 22px;
  font-variation-settings: "wdth" 65, "wght" 820;
}

.course-media.is-empty small {
  color: var(--muted);
}

.course-decision {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
  padding: 18px;
}

.credit {
  display: flex;
  align-items: baseline;
  gap: 5px;
}

.credit strong {
  font-family: var(--font-display);
  font-size: 26px;
  font-variation-settings: "wdth" 76, "wght" 680;
}

.credit span,
.seat-status span {
  color: var(--muted);
  font-size: 9px;
}

.seat-status {
  display: grid;
  gap: 5px;
}

.seat-status > span {
  display: flex;
  align-items: center;
  gap: 5px;
}

.seat-status strong {
  color: var(--success);
  font-size: 11px;
}

.seat-status strong.is-full {
  color: var(--danger);
}

.seat-status > div {
  overflow: hidden;
  height: 4px;
  background: var(--line-soft);
}

.seat-status i {
  display: block;
  height: 100%;
  background: var(--brand);
}

.primary-action {
  min-height: 40px;
  padding: 9px 10px;
  border: 1px solid var(--brand);
  border-radius: 8px;
  color: white;
  background: var(--brand);
  font-size: 11px;
  font-weight: 750;
  cursor: pointer;
}

.primary-action:hover:not(:disabled) {
  background: var(--brand-hover);
}

.primary-action.is-waitlist {
  border-color: var(--signal);
  color: var(--signal);
  background: transparent;
}

.primary-action:disabled {
  border-color: var(--line);
  color: var(--muted);
  background: var(--surface-muted);
  cursor: not-allowed;
}

@media (max-width: 850px) {
  .course-card {
    grid-template-columns: 62px minmax(0, 1fr) 150px;
  }

  .course-media {
    display: none;
  }
}

@media (max-width: 600px) {
  .course-card {
    grid-template-columns: 48px minmax(0, 1fr);
  }

  .course-index {
    padding-top: 18px;
    font-size: 25px;
  }

  .course-body {
    padding: 17px 14px;
  }

  .course-body header {
    align-items: flex-start;
    flex-direction: column;
  }

  .course-tags {
    justify-content: flex-start;
  }

  .course-body h3 {
    font-size: 22px;
  }

  .course-media:not(.is-empty) {
    display: grid;
    grid-column: 2;
    grid-template-columns: auto 1fr;
    justify-items: start;
    margin: 0 14px 12px;
    padding: 10px 12px;
    border: 0;
    border-radius: 8px;
  }

  .course-media:not(.is-empty) > span {
    width: 32px;
    height: 32px;
    grid-row: 1 / 3;
  }

  .course-media.is-empty {
    display: none;
  }

  .course-decision {
    grid-column: 2;
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: end;
    padding: 0 14px 17px;
    border-top: 0;
  }

  .credit {
    display: flex;
    grid-column: 1;
  }

  .seat-status {
    grid-column: 2;
  }

  .primary-action {
    width: 100%;
    min-width: 120px;
    grid-column: 1 / -1;
  }
}
</style>
