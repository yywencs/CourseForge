<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, shallowRef } from 'vue'

import {
  completeCourseVideoUpload,
  listCourseVideos,
  startCourseVideoUpload,
} from '@/api/catalog'
import type { Course, CourseVideo } from '@/types/catalog'

const props = defineProps<{ course: Course }>()
const emit = defineEmits<{ close: []; uploaded: [] }>()

const videos = shallowRef<CourseVideo[]>([])
const file = shallowRef<File>()
const title = shallowRef('课程预览')
const busy = shallowRef(false)
const readyPreview = computed(() => videos.value.find((video) =>
  video.video_kind === 'preview' && video.status === 'ready'))

onMounted(loadVideos)

async function loadVideos(): Promise<void> {
  videos.value = (await listCourseVideos(props.course.id)).items
}

function selectFile(event: Event): void {
  file.value = (event.target as HTMLInputElement).files?.[0]
}

async function upload(): Promise<void> {
  const selected = file.value
  if (!selected) {
    ElMessage.warning('请选择 MP4 视频')
    return
  }
  if (selected.type !== 'video/mp4' || !selected.name.toLowerCase().endsWith('.mp4')) {
    ElMessage.warning('当前仅支持 MP4 视频')
    return
  }
  busy.value = true
  try {
    const durationMs = await readDuration(selected)
    const ticket = await startCourseVideoUpload(props.course.id, {
      video_kind: 'preview',
      title: title.value.trim() || '课程预览',
      file_name: selected.name,
      content_type: 'video/mp4',
      file_size: selected.size,
      sort_order: 0,
    })
    const response = await fetch(ticket.upload_url, {
      method: ticket.method,
      headers: ticket.headers,
      body: selected,
    })
    if (!response.ok) throw new Error(`对象存储上传失败（${response.status}）`)
    await completeCourseVideoUpload(ticket.video.id, durationMs)
    await loadVideos()
    file.value = undefined
    emit('uploaded')
    ElMessage.success('课程预览视频已上传')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '视频上传失败')
  } finally {
    busy.value = false
  }
}

function readDuration(selected: File): Promise<number | undefined> {
  return new Promise((resolve) => {
    const element = document.createElement('video')
    const objectURL = URL.createObjectURL(selected)
    const finish = (duration?: number) => {
      URL.revokeObjectURL(objectURL)
      resolve(duration)
    }
    element.preload = 'metadata'
    element.onloadedmetadata = () => finish(Number.isFinite(element.duration)
      ? Math.max(1, Math.round(element.duration * 1000))
      : undefined)
    element.onerror = () => finish()
    element.src = objectURL
  })
}
</script>

<template>
  <section class="video-uploader">
    <header>
      <div><h2>课程预览视频</h2><p>{{ course.course_code }} · {{ course.course_name }}</p></div>
      <button type="button" class="quiet" @click="emit('close')">关闭</button>
    </header>

    <article v-for="video in videos" :key="video.id" class="video-record">
      <div><strong>{{ video.title }}</strong><span>{{ video.video_kind }} · {{ video.status }}</span></div>
      <span>{{ video.duration_ms ? `${Math.round(video.duration_ms / 1000)} 秒` : '时长待确认' }}</span>
    </article>

    <p v-if="readyPreview" class="ready-note">当前课程已有可播放的预览视频；替换和删除将在下一步实现。</p>
    <form v-else class="upload-form" @submit.prevent="upload">
      <label><span>视频标题</span><input v-model="title" maxlength="128" required /></label>
      <label><span>MP4 文件</span><input type="file" accept="video/mp4,.mp4" required @change="selectFile" /></label>
      <p>文件会由浏览器直接上传到对象存储，不经过 CourseForge API。</p>
      <button type="submit" class="primary" :disabled="busy">{{ busy ? '正在上传…' : '上传预览视频' }}</button>
    </form>
  </section>
</template>

<style scoped>
.video-uploader { display: grid; gap: 18px; }.video-uploader header { display: flex; align-items: start; justify-content: space-between; gap: 20px; }.video-uploader h2 { margin: 0; font-size: 22px; }.video-uploader header p { margin: 5px 0 0; color: var(--muted); font-size: 11px; }.quiet, .primary { min-height: 38px; padding: 0 14px; border: 1px solid var(--line); border-radius: 8px; cursor: pointer; }.quiet { background: white; }.primary { border-color: var(--brand); color: white; background: var(--brand); }.primary:disabled { opacity: .55; cursor: wait; }.video-record { display: flex; align-items: center; justify-content: space-between; padding: 13px; border: 1px solid var(--line); border-radius: 8px; }.video-record div { display: grid; gap: 4px; }.video-record span { color: var(--muted); font-size: 10px; }.ready-note { margin: 0; padding: 12px; border-radius: 8px; color: var(--muted); background: var(--surface-muted); font-size: 11px; }.upload-form { display: grid; gap: 14px; padding-top: 16px; border-top: 1px solid var(--line-soft); }.upload-form label { display: grid; gap: 7px; color: var(--muted); font-size: 11px; }.upload-form input { padding: 10px 12px; border: 1px solid var(--line); border-radius: 8px; }.upload-form p { margin: 0; color: var(--muted); font-size: 10px; }.upload-form .primary { justify-self: end; }
</style>
