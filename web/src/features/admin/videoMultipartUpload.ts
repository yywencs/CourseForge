import type { VideoUploadPartTicket } from '@/types/catalog'

const MAX_AUTOMATIC_RETRIES = 3
const INITIAL_RETRY_DELAY_MS = 500

interface UploadVideoPartsOptions {
  refreshPartURL: (partNumber: number) => Promise<VideoUploadPartTicket>
  fetcher?: typeof fetch
  wait?: (milliseconds: number) => Promise<void>
}

export async function uploadVideoParts(
  file: File,
  partSize: number,
  parts: VideoUploadPartTicket[],
  options: UploadVideoPartsOptions,
): Promise<void> {
  let cursor = 0
  const workerCount = Math.min(4, parts.length)
  const results = await Promise.allSettled(Array.from({ length: workerCount }, async () => {
    while (cursor < parts.length) {
      const part = parts[cursor++]
      if (!part) return
      const start = (part.part_number - 1) * partSize
      await uploadPartWithRetry(
        file.slice(start, Math.min(start + partSize, file.size)),
        part,
        options,
      )
    }
  }))
  const failure = results.find((result): result is PromiseRejectedResult => result.status === 'rejected')
  if (failure) throw failure.reason
}

export async function uploadPartWithRetry(
  body: Blob,
  initialPart: VideoUploadPartTicket,
  options: UploadVideoPartsOptions,
): Promise<void> {
  const fetcher = options.fetcher ?? fetch
  const wait = options.wait ?? delay
  let part = initialPart

  for (let retry = 0; retry <= MAX_AUTOMATIC_RETRIES; retry++) {
    let response: Response | undefined
    let networkError: unknown
    try {
      response = await fetcher(part.upload_url, { method: part.method, body })
      if (response.ok) return
    } catch (error) {
      networkError = error
    }

    const retryable = networkError !== undefined || isRetryableStatus(response?.status)
    if (!retryable || retry === MAX_AUTOMATIC_RETRIES) {
      const detail = response ? `HTTP ${response.status}` : '网络错误'
      throw new Error(`第 ${part.part_number} 个分片上传失败（${detail}），请点击继续上传`)
    }
    if (response?.status === 401 || response?.status === 403) {
      part = await options.refreshPartURL(part.part_number)
    }
    await wait(INITIAL_RETRY_DELAY_MS * 2 ** retry)
  }
}

export function missingPartNumbers(
  allParts: readonly VideoUploadPartTicket[],
  uploadedPartNumbers: ReadonlySet<number>,
): number[] {
  return allParts
    .map((part) => part.part_number)
    .filter((partNumber) => !uploadedPartNumbers.has(partNumber))
}

function isRetryableStatus(status?: number): boolean {
  return status === 401 || status === 403 || status === 408 || status === 429 ||
    (status !== undefined && status >= 500)
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, milliseconds))
}
