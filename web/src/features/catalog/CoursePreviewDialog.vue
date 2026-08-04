<script setup lang="ts">
import { ExternalLink, PlayCircle, Send, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, shallowRef, useTemplateRef, watch } from 'vue'

import { publishDanmaku } from '@/api/danmaku'
import type { PublishDanmakuRequest } from '@/types/danmaku'
import type { TeachingClassSummary } from '@/types/enrollment'
import { createRequestId } from '@/utils/requestId'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ course?: TeachingClassSummary }>()
const previewDialog = useTemplateRef<HTMLDialogElement>('previewDialog')
const video = useTemplateRef<HTMLVideoElement>('video')
const content = shallowRef('')
const submitting = shallowRef(false)
const feedback = shallowRef('')
const retryRequest = shallowRef<PublishDanmakuRequest>()

const isDirectVideo = computed(() =>
  Boolean(props.course?.videoUrl?.match(/\.(mp4|webm)(\?.*)?$/i)),
)
const contentCharacters = computed(() => Array.from(content.value.trim()).length)
const canSubmit = computed(() =>
  Boolean(props.course?.videoId) && contentCharacters.value > 0 &&
  contentCharacters.value <= 200 && !submitting.value,
)

watch(open, async (visible) => {
  if (visible) resetComposer()
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
    await publishDanmaku(videoID, request)
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
            <video ref="video" :src="course.videoUrl" controls preload="metadata" />
            <form class="danmaku-composer" @submit.prevent="submitDanmaku">
              <label for="danmaku-content">发送弹幕</label>
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

.preview-dialog__media video {
  width: 100%;
  max-height: 480px;
}

.danmaku-composer {
  display: grid;
  width: 100%;
  gap: 7px;
  padding: 14px 18px 16px;
  color: white;
  background: #151515;
}

.danmaku-composer > label {
  font-size: 12px;
  font-weight: 750;
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
