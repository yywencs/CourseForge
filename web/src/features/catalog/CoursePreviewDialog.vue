<script setup lang="ts">
import { ExternalLink, PlayCircle, Send, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, shallowRef, useTemplateRef, watch } from 'vue'

import { listDanmakuSegment, publishDanmaku } from '@/api/danmaku'
import type { HistoricalDanmaku, PublishDanmakuRequest } from '@/types/danmaku'
import type { TeachingClassSummary } from '@/types/enrollment'
import { createRequestId } from '@/utils/requestId'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ course?: TeachingClassSummary }>()
const previewDialog = useTemplateRef<HTMLDialogElement>('previewDialog')
const video = useTemplateRef<HTMLVideoElement>('video')
const danmakuLayer = useTemplateRef<HTMLDivElement>('danmakuLayer')
const content = shallowRef('')
const submitting = shallowRef(false)
const feedback = shallowRef('')
const retryRequest = shallowRef<PublishDanmakuRequest>()
const danmakuEnabled = shallowRef(true)
const historyFeedback = shallowRef('')

const segmentDurationMS = 60_000
const prefetchBeforeEndMS = 10_000
const danmakuTravelDurationMS = 7_000
const danmakuLaneCount = 6

const segmentCache = new Map<number, HistoricalDanmaku[]>()
const loadingSegments = new Map<number, Promise<HistoricalDanmaku[]>>()
const segmentRetryAt = new Map<number, number>()
const displayedDanmakuIDs = new Set<number>()
const activeTimers = new Map<string, number>()
const activeElements = new Map<string, HTMLSpanElement>()
const laneAvailableAt = Array.from({ length: danmakuLaneCount }, () => 0)
let historyGeneration = 0
let lastPlaybackMS = -1
let activeSequence = 0

const isDirectVideo = computed(() =>
  Boolean(props.course?.videoUrl?.match(/\.(mp4|webm)(\?.*)?$/i)),
)
const contentCharacters = computed(() => Array.from(content.value.trim()).length)
const canSubmit = computed(() =>
  Boolean(props.course?.videoId) && contentCharacters.value > 0 &&
  contentCharacters.value <= 200 && !submitting.value,
)

watch(open, async (visible) => {
  if (visible) {
    resetComposer()
    resetDanmakuState()
  } else {
    resetDanmakuState()
  }
  await nextTick()
  const dialog = previewDialog.value
  if (!dialog) return
  if (visible && props.course && !dialog.open) {
    // 原生 modal dialog 自动约束焦点，并在关闭后把焦点还给触发按钮。
    dialog.showModal()
    return
  }
  if (!visible && dialog.open) dialog.close()
})

function close(): void {
  const dialog = previewDialog.value
  if (dialog?.open) dialog.close()
  open.value = false
}

function resetComposer(): void {
  content.value = ''
  feedback.value = ''
  retryRequest.value = undefined
  submitting.value = false
}

function currentSegmentIndex(videoTimeMS: number): number {
  return Math.floor(Math.max(0, videoTimeMS) / segmentDurationMS) + 1
}

function resetDanmakuState(): void {
  historyGeneration += 1
  segmentCache.clear()
  loadingSegments.clear()
  segmentRetryAt.clear()
  displayedDanmakuIDs.clear()
  historyFeedback.value = ''
  lastPlaybackMS = -1
  clearActiveDanmakus()
}

function clearActiveDanmakus(): void {
  for (const timer of activeTimers.values()) window.clearTimeout(timer)
  activeTimers.clear()
  activeElements.clear()
  danmakuLayer.value?.replaceChildren()
  laneAvailableAt.fill(0)
}

function loadSegment(segmentIndex: number): Promise<HistoricalDanmaku[]> {
  const videoID = props.course?.videoId
  if (!videoID || segmentIndex <= 0) return Promise.resolve([])
  const durationMS = Math.floor((video.value?.duration ?? Number.NaN) * 1000)
  if (segmentIndex > 1 && Number.isFinite(durationMS) &&
    (segmentIndex - 1) * segmentDurationMS >= durationMS) {
    return Promise.resolve([])
  }
  const cached = segmentCache.get(segmentIndex)
  if (cached) return Promise.resolve(cached)
  const loading = loadingSegments.get(segmentIndex)
  if (loading) return loading
  if ((segmentRetryAt.get(segmentIndex) ?? 0) > Date.now()) return Promise.resolve([])

  const generation = historyGeneration
  const request = listDanmakuSegment(videoID, segmentIndex)
    .then((page) => {
      if (generation !== historyGeneration || props.course?.videoId !== videoID) return []
      if (page.segment_index !== segmentIndex) throw new Error('弹幕分段响应不匹配')
      const items = [...page.items].sort((left, right) =>
        left.video_time_ms - right.video_time_ms || left.id - right.id,
      )
      segmentCache.set(segmentIndex, items)
      segmentRetryAt.delete(segmentIndex)
      historyFeedback.value = ''
      return items
    })
    .catch((error: unknown) => {
      if (generation === historyGeneration) {
        segmentRetryAt.set(segmentIndex, Date.now() + 5_000)
        if (segmentIndex === currentSegmentIndex(currentVideoTimeMS())) {
          historyFeedback.value = error instanceof Error ? error.message : '历史弹幕加载失败'
        }
      }
      return []
    })
    .finally(() => {
      if (loadingSegments.get(segmentIndex) === request) loadingSegments.delete(segmentIndex)
    })
  loadingSegments.set(segmentIndex, request)
  return request
}

function currentVideoTimeMS(): number {
  return Math.max(0, Math.floor((video.value?.currentTime ?? 0) * 1000))
}

function showDanmaku(item: HistoricalDanmaku): void {
  if (!danmakuEnabled.value || displayedDanmakuIDs.has(item.id)) return
  displayedDanmakuIDs.add(item.id)
  const now = performance.now()
  const lane = laneAvailableAt.findIndex((availableAt) => availableAt <= now)
  if (lane < 0) return

  laneAvailableAt[lane] = now + danmakuTravelDurationMS
  const key = `${historyGeneration}-${item.id}-${activeSequence++}`
  const element = document.createElement('span')
  element.className = 'danmaku-item'
  element.textContent = item.content
  element.style.top = `${14 + lane * 34}px`
  danmakuLayer.value?.append(element)
  activeElements.set(key, element)
  const timer = window.setTimeout(() => {
    activeElements.get(key)?.remove()
    activeElements.delete(key)
    activeTimers.delete(key)
  }, danmakuTravelDurationMS)
  activeTimers.set(key, timer)
}

function dispatchDueDanmakus(fromMS: number, toMS: number): void {
  if (!danmakuEnabled.value || toMS < fromMS) return
  const firstSegment = currentSegmentIndex(Math.max(0, fromMS))
  const lastSegment = currentSegmentIndex(toMS)
  for (let segmentIndex = firstSegment; segmentIndex <= lastSegment; segmentIndex += 1) {
    for (const item of segmentCache.get(segmentIndex) ?? []) {
      if (item.video_time_ms > fromMS && item.video_time_ms <= toMS) showDanmaku(item)
    }
  }
}

function handleLoadedMetadata(): void {
  const currentMS = currentVideoTimeMS()
  lastPlaybackMS = currentMS === 0 ? -1 : currentMS
  void loadSegment(currentSegmentIndex(currentMS))
}

function handleTimeUpdate(): void {
  const currentMS = currentVideoTimeMS()
  const segmentIndex = currentSegmentIndex(currentMS)
  void loadSegment(segmentIndex)
  const segmentEndMS = segmentIndex * segmentDurationMS
  if (segmentEndMS - currentMS <= prefetchBeforeEndMS) void loadSegment(segmentIndex + 1)

  // 大跨度变化交给 seeked 处理；这里不集中补放被跳过时间内的弹幕。
  if (lastPlaybackMS >= 0 && (currentMS < lastPlaybackMS || currentMS - lastPlaybackMS > 2_000)) {
    lastPlaybackMS = currentMS
    return
  }
  dispatchDueDanmakus(lastPlaybackMS, currentMS)
  lastPlaybackMS = currentMS
}

function handleSeeked(): void {
  const currentMS = currentVideoTimeMS()
  clearActiveDanmakus()
  displayedDanmakuIDs.clear()
  lastPlaybackMS = currentMS
  const segmentIndex = currentSegmentIndex(currentMS)
  void loadSegment(segmentIndex)
  void loadSegment(segmentIndex + 1)
}

function toggleDanmaku(): void {
  danmakuEnabled.value = !danmakuEnabled.value
  clearActiveDanmakus()
  lastPlaybackMS = currentVideoTimeMS()
  if (danmakuEnabled.value) void loadSegment(currentSegmentIndex(lastPlaybackMS))
}

function handleContentInput(): void {
  // 内容发生变化后，它已经不是上一次失败请求的幂等重试。
  const normalizedContent = content.value.trim()
  if (retryRequest.value && normalizedContent !== retryRequest.value.content) {
    retryRequest.value = undefined
  }
  // 程序在成功后清空输入框时不覆盖成功提示；用户开始编辑下一条时再清除。
  if (normalizedContent) feedback.value = ''
}

async function submitDanmaku(): Promise<void> {
  const videoID = props.course?.videoId
  const player = video.value
  const normalizedContent = content.value.trim()
  const characters = Array.from(normalizedContent).length
  if (!videoID || !player || !normalizedContent || characters > 200 || submitting.value) return

  const request = retryRequest.value ?? {
    client_msg_id: createRequestId(),
    video_time_ms: Math.max(0, Math.floor(player.currentTime * 1000)),
    content: normalizedContent,
  }
  retryRequest.value = request
  submitting.value = true
  feedback.value = ''
  try {
    const published = await publishDanmaku(videoID, request)
    const historyItem: HistoricalDanmaku = {
      id: published.id,
      video_time_ms: published.video_time_ms,
      content: published.content,
      create_time: published.create_time,
    }
    const segmentIndex = currentSegmentIndex(historyItem.video_time_ms)
    const cached = segmentCache.get(segmentIndex)
    if (cached && !cached.some((item) => item.id === historyItem.id)) {
      cached.push(historyItem)
      cached.sort((left, right) => left.video_time_ms - right.video_time_ms || left.id - right.id)
    }
    showDanmaku(historyItem)
    content.value = ''
    retryRequest.value = undefined
    feedback.value = '弹幕已发送并保存'
  } catch (error) {
    feedback.value = `${error instanceof Error ? error.message : '弹幕发送失败'}，可点击重试`
  } finally {
    submitting.value = false
  }
}

function handleCancel(): void {
  // 原生 dialog 会响应 Escape；同步 v-model 保持路由页面状态一致。
  open.value = false
}

function handleBackdropClick(event: MouseEvent): void {
  if (event.target === previewDialog.value) close()
}

onBeforeUnmount(() => {
  resetDanmakuState()
  if (previewDialog.value?.open) previewDialog.value.close()
})
</script>

<template>
  <Teleport to="body">
    <dialog
      ref="previewDialog"
      class="preview-dialog"
      aria-labelledby="course-preview-title"
      @cancel="handleCancel"
      @close="open = false"
      @click="handleBackdropClick"
    >
      <section v-if="course">
        <button class="preview-dialog__close" type="button" aria-label="关闭课程预览" autofocus @click="close">
          <X :size="19" />
        </button>
        <div class="preview-dialog__media">
          <template v-if="isDirectVideo">
            <div class="video-stage">
              <video
                ref="video"
                :src="course.videoUrl"
                controls
                preload="metadata"
                @loadedmetadata="handleLoadedMetadata"
                @timeupdate="handleTimeUpdate"
                @seeked="handleSeeked"
                @ended="clearActiveDanmakus"
              />
              <div ref="danmakuLayer" class="danmaku-layer" aria-live="off" aria-hidden="true"></div>
            </div>
            <form class="danmaku-composer" @submit.prevent="submitDanmaku">
              <header>
                <label for="danmaku-content">发送弹幕</label>
                <span v-if="historyFeedback">{{ historyFeedback }}</span>
                <button
                  class="danmaku-toggle"
                  type="button"
                  :aria-pressed="danmakuEnabled"
                  @click="toggleDanmaku"
                >弹幕 {{ danmakuEnabled ? '开' : '关' }}</button>
              </header>
              <div>
                <input
                  id="danmaku-content"
                  v-model="content"
                  type="text"
                  autocomplete="off"
                  placeholder="在当前播放位置说点什么"
                  aria-describedby="danmaku-feedback"
                  @input="handleContentInput"
                />
                <span :class="{ 'is-over-limit': contentCharacters > 200 }">{{ contentCharacters }}/200</span>
                <button type="submit" :disabled="!canSubmit">
                  <Send :size="16" />{{ submitting ? '发送中' : retryRequest ? '重试' : '发送' }}
                </button>
              </div>
              <p id="danmaku-feedback" aria-live="polite">{{ feedback || '弹幕将绑定到当前播放时间' }}</p>
            </form>
          </template>
          <div v-else class="preview-dialog__placeholder">
            <PlayCircle :size="52" />
            <strong>课程内容将在外部页面打开</strong>
            <span>当前地址不是可直接播放的视频文件，可通过下方入口查看。</span>
          </div>
        </div>
        <div class="preview-dialog__copy">
          <span>{{ course.courseCode }} · {{ course.teacherName }}</span>
          <h2 id="course-preview-title">{{ course.courseName }}</h2>
          <p>{{ course.introduction }}</p>
          <a v-if="course.videoUrl" :href="course.videoUrl" target="_blank" rel="noreferrer">
            打开课程内容页<ExternalLink :size="15" />
          </a>
        </div>
      </section>
    </dialog>
  </Teleport>
</template>

<style scoped>
.preview-dialog {
  position: fixed;
  inset: 0;
  width: min(850px, calc(100% - 32px));
  max-height: calc(100vh - 32px);
  margin: auto;
  overflow: auto;
  padding: 0;
  border: 1px solid var(--ink);
  border-radius: 14px;
  background: var(--surface);
  box-shadow: 0 36px 100px rgba(0, 0, 0, 0.32);
}

.preview-dialog::backdrop {
  background: rgba(10, 10, 10, 0.76);
}

.preview-dialog > section {
  position: relative;
}

.preview-dialog__close {
  position: absolute;
  z-index: 1;
  top: 14px;
  right: 14px;
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.5);
  border-radius: 50%;
  color: white;
  background: rgba(0, 0, 0, 0.55);
  cursor: pointer;
}

.preview-dialog__media {
  display: grid;
  min-height: 330px;
  place-items: center;
  color: white;
  background: var(--ink);
}

.video-stage {
  position: relative;
  width: 100%;
  overflow: hidden;
  background: #000;
}

.video-stage video {
  display: block;
  width: 100%;
  max-height: 480px;
}

.danmaku-layer {
  position: absolute;
  z-index: 2;
  inset: 0 0 42px;
  overflow: hidden;
  pointer-events: none;
}

.danmaku-layer :deep(.danmaku-item) {
  position: absolute;
  left: 100%;
  width: max-content;
  max-width: 75%;
  overflow: hidden;
  color: white;
  font-size: 18px;
  font-weight: 700;
  line-height: 28px;
  text-overflow: ellipsis;
  text-shadow: 0 1px 3px #000, 1px 0 2px #000, -1px 0 2px #000;
  white-space: nowrap;
  animation: danmaku-scroll 7s linear forwards;
  will-change: transform;
}

@keyframes danmaku-scroll {
  to {
    transform: translateX(calc(-100% - 900px));
  }
}

.danmaku-composer .danmaku-toggle {
  padding: 5px 9px;
  border: 1px solid rgba(255, 255, 255, 0.45);
  border-radius: 6px;
  color: white;
  background: rgba(0, 0, 0, 0.62);
  font-size: 11px;
  cursor: pointer;
}

.danmaku-composer {
  display: grid;
  width: 100%;
  gap: 7px;
  padding: 14px 18px 16px;
  color: white;
  background: #151515;
}

.danmaku-composer > header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.danmaku-composer > header label {
  font-size: 12px;
  font-weight: 750;
}

.danmaku-composer > header span {
  min-width: 0;
  flex: 1;
  color: #ffaaa2;
  font-size: 11px;
  font-weight: 500;
}

.danmaku-composer > header .danmaku-toggle {
  margin-left: auto;
}

.danmaku-composer > div {
  display: flex;
  align-items: center;
  gap: 10px;
}

.danmaku-composer input {
  min-width: 0;
  flex: 1;
  padding: 10px 12px;
  border: 1px solid rgba(255, 255, 255, 0.28);
  border-radius: 8px;
  outline: none;
  color: white;
  background: rgba(255, 255, 255, 0.08);
}

.danmaku-composer input:focus {
  border-color: var(--signal);
}

.danmaku-composer span {
  color: rgba(255, 255, 255, 0.55);
  font-family: var(--font-mono);
  font-size: 11px;
}

.danmaku-composer span.is-over-limit {
  color: #ff8c82;
}

.danmaku-composer button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 15px;
  border: 0;
  border-radius: 8px;
  color: var(--ink);
  background: var(--signal);
  font-weight: 750;
  cursor: pointer;
}

.danmaku-composer button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.danmaku-composer p {
  min-height: 16px;
  margin: 0;
  color: rgba(255, 255, 255, 0.6);
  font-size: 11px;
}

.preview-dialog__placeholder {
  display: grid;
  max-width: 420px;
  justify-items: center;
  gap: 12px;
  padding: 50px 24px;
  text-align: center;
}

.preview-dialog__placeholder > svg {
  color: var(--signal);
}

.preview-dialog__placeholder span {
  color: rgba(255, 255, 255, 0.65);
  font-size: 13px;
  line-height: 1.6;
}

.preview-dialog__copy {
  padding: 26px 30px 30px;
}

.preview-dialog__copy > span {
  color: var(--signal);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 750;
}

.preview-dialog__copy h2 {
  margin: 8px 0;
  font-family: var(--font-display);
  font-size: 34px;
  font-variation-settings: "wdth" 75, "wght" 760;
  letter-spacing: -0.03em;
}

.preview-dialog__copy p {
  margin: 0;
  color: var(--muted);
  line-height: 1.7;
}

.preview-dialog__copy a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 18px;
  color: var(--brand);
  font-size: 13px;
  font-weight: 750;
}

@media (max-width: 580px) {
  .preview-dialog__media {
    min-height: 220px;
  }

  .preview-dialog__copy {
    padding: 20px;
  }

  .danmaku-composer > div {
    flex-wrap: wrap;
  }

  .danmaku-composer input {
    flex-basis: calc(100% - 54px);
  }
}
</style>
