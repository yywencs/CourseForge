<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, shallowRef } from 'vue'

import {
  completeCourseVideoUpload,
  listCourseVideoUploadParts,
  listCourseVideos,
  presignCourseVideoUploadParts,
  startCourseVideoUpload,
} from '@/api/catalog'
import type { Course, CourseVideo, VideoUploadTicket } from '@/types/catalog'
import { missingPartNumbers, uploadVideoParts } from '@/features/admin/videoMultipartUpload'

const props = defineProps<{ course: Course }>()
const emit = defineEmits<{ close: []; uploaded: [] }>()

const videos = shallowRef<CourseVideo[]>([])
const file = shallowRef<File>()
const title = shallowRef('课程预览')
const busy = shallowRef(false)
const activeUpload = shallowRef<{
  ticket: VideoUploadTicket
  durationMs?: number
  fileName: string
  fileSize: number
  lastModified: number
  partsUploaded: boolean
}>()
const readyPreview = computed(() => videos.value.find((video) =>
  video.video_kind === 'preview' && video.status === 'ready'))

onMounted(loadVideos)

async function loadVideos(): Promise<void> {
  videos.value = (await listCourseVideos(props.course.id)).items
}

function selectFile(event: Event): void {
  const selected = (event.target as HTMLInputElement).files?.[0]
  file.value = selected
  if (selected && activeUpload.value && !isSameFile(selected, activeUpload.value)) {
    activeUpload.value = undefined
  }
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
    let current = activeUpload.value
    if (!current) {
      const durationMs = await readDuration(selected)
      const ticket = await startCourseVideoUpload(props.course.id, {
        video_kind: 'preview',
        title: title.value.trim() || '课程预览',
        file_name: selected.name,
        content_type: 'video/mp4',
        file_size: selected.size,
        sort_order: 0,
      })
      current = {
        ticket, durationMs, fileName: selected.name,
        fileSize: selected.size, lastModified: selected.lastModified,
        partsUploaded: false,
      }
      activeUpload.value = current
      await uploadPartsWithRetry(selected, ticket, ticket.parts)
      current.partsUploaded = true
    } else if (!current.partsUploaded) {
      await resumeUpload(selected, current.ticket)
      current.partsUploaded = true
    }
    await completeCourseVideoUpload(current.ticket.upload_id, current.durationMs)
    await loadVideos()
    file.value = undefined
    activeUpload.value = undefined
    emit('uploaded')
    ElMessage.success('课程预览视频已上传')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '视频上传失败')
  } finally {
    busy.value = false
  }
}

async function resumeUpload(selected: File, ticket: VideoUploadTicket): Promise<void> {
  const uploaded = await listCourseVideoUploadParts(ticket.upload_id)
  const missing = missingPartNumbers(
    ticket.parts,
    new Set(uploaded.parts.map((part) => part.part_number)),
  )
  if (missing.length === 0) return
  const refreshed = await presignCourseVideoUploadParts(
    ticket.upload_id,
    ticket.multipart_upload_id,
    missing,
  )
  await uploadPartsWithRetry(selected, ticket, refreshed.parts)
}

async function uploadPartsWithRetry(
  selected: File,
  ticket: VideoUploadTicket,
  parts: VideoUploadTicket['parts'],
): Promise<void> {
  await uploadVideoParts(selected, ticket.part_size_bytes, parts, {
    refreshPartURL: async (partNumber) => {
      const refreshed = await presignCourseVideoUploadParts(
        ticket.upload_id,
        ticket.multipart_upload_id,
        [partNumber],
      )
      const part = refreshed.parts[0]
      if (!part) throw new Error(`无法刷新第 ${partNumber} 个分片的上传地址`)
      return part
    },
  })
}

function isSameFile(selected: File, upload: NonNullable<typeof activeUpload.value>): boolean {
  return selected.name === upload.fileName && selected.size === upload.fileSize &&
    selected.lastModified === upload.lastModified
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
      <button type="submit" class="primary" :disabled="busy">
        {{ busy ? '正在上传…' : activeUpload ? '继续上传' : '上传预览视频' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.video-uploader { display: grid; gap: 18px; }.video-uploader header { display: flex; align-items: start; justify-content: space-between; gap: 20px; }.video-uploader h2 { margin: 0; font-size: 22px; }.video-uploader header p { margin: 5px 0 0; color: var(--muted); font-size: 11px; }.quiet, .primary { min-height: 38px; padding: 0 14px; border: 1px solid var(--line); border-radius: 8px; cursor: pointer; }.quiet { background: white; }.primary { border-color: var(--brand); color: white; background: var(--brand); }.primary:disabled { opacity: .55; cursor: wait; }.video-record { display: flex; align-items: center; justify-content: space-between; padding: 13px; border: 1px solid var(--line); border-radius: 8px; }.video-record div { display: grid; gap: 4px; }.video-record span { color: var(--muted); font-size: 10px; }.ready-note { margin: 0; padding: 12px; border-radius: 8px; color: var(--muted); background: var(--surface-muted); font-size: 11px; }.upload-form { display: grid; gap: 14px; padding-top: 16px; border-top: 1px solid var(--line-soft); }.upload-form label { display: grid; gap: 7px; color: var(--muted); font-size: 11px; }.upload-form input { padding: 10px 12px; border: 1px solid var(--line); border-radius: 8px; }.upload-form p { margin: 0; color: var(--muted); font-size: 10px; }.upload-form .primary { justify-self: end; }
</style>
