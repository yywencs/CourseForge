<script setup lang="ts">
import { ExternalLink, PlayCircle, X } from '@lucide/vue'
import { computed } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const open = defineModel<boolean>({ required: true })
const props = defineProps<{
  course?: TeachingClassSummary
}>()

const isDirectVideo = computed(() =>
  Boolean(props.course?.videoUrl?.match(/\.(mp4|webm)(\?.*)?$/i)),
)
</script>

<template>
  <Teleport to="body">
    <Transition name="preview">
      <div v-if="open && course" class="preview-backdrop" @click.self="open = false">
        <section class="preview-dialog" role="dialog" aria-modal="true" aria-label="课程预览">
          <button class="preview-dialog__close" type="button" aria-label="关闭" @click="open = false">
            <X :size="19" />
          </button>
          <div class="preview-dialog__media">
            <video
              v-if="isDirectVideo"
              :src="course.videoUrl"
              controls
              preload="metadata"
            />
            <div v-else class="preview-dialog__placeholder">
              <PlayCircle :size="52" />
              <strong>课程视频位已就绪</strong>
              <span>配置 MP4/WebM 地址后可在这里直接播放，也可以跳转到课程内容页。</span>
            </div>
          </div>
          <div class="preview-dialog__copy">
            <span>{{ course.courseCode }} · {{ course.teacherName }}</span>
            <h2>{{ course.courseName }}</h2>
            <p>{{ course.introduction }}</p>
            <a
              v-if="course.videoUrl"
              :href="course.videoUrl"
              target="_blank"
              rel="noreferrer"
            >
              打开课程内容页
              <ExternalLink :size="15" />
            </a>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.preview-backdrop {
  position: fixed;
  z-index: 100;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(5, 21, 17, 0.68);
  backdrop-filter: blur(8px);
}

.preview-dialog {
  position: relative;
  overflow: hidden;
  width: min(850px, 100%);
  border-radius: 24px;
  background: var(--surface);
  box-shadow: 0 36px 100px rgba(0, 0, 0, 0.32);
}

.preview-dialog__close {
  position: absolute;
  z-index: 1;
  top: 14px;
  right: 14px;
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 50%;
  color: white;
  background: rgba(4, 24, 19, 0.5);
  cursor: pointer;
}

.preview-dialog__media {
  display: grid;
  min-height: 330px;
  place-items: center;
  color: white;
  background:
    radial-gradient(circle at 72% 18%, rgba(80, 197, 168, 0.32), transparent 30%),
    #0a362c;
}

.preview-dialog__media video {
  width: 100%;
  max-height: 480px;
}

.preview-dialog__placeholder {
  display: grid;
  max-width: 400px;
  justify-items: center;
  gap: 12px;
  padding: 50px 24px;
  text-align: center;
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
  color: var(--brand);
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 750;
}

.preview-dialog__copy h2 {
  margin: 8px 0;
  font-family: var(--font-display);
  font-size: 30px;
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

.preview-enter-active,
.preview-leave-active {
  transition: opacity 160ms ease;
}

.preview-enter-from,
.preview-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .preview-enter-active,
  .preview-leave-active {
    transition: none;
  }
}
</style>
