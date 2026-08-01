<script setup lang="ts">
import { ExternalLink, PlayCircle, X } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, useTemplateRef, watch } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{ course?: TeachingClassSummary }>()
const previewDialog = useTemplateRef<HTMLDialogElement>('previewDialog')

const isDirectVideo = computed(() =>
  Boolean(props.course?.videoUrl?.match(/\.(mp4|webm)(\?.*)?$/i)),
)

watch(open, async (visible) => {
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
          <video v-if="isDirectVideo" :src="course.videoUrl" controls preload="metadata" />
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
  width: min(850px, calc(100% - 32px));
  max-height: calc(100vh - 32px);
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
}
</style>
