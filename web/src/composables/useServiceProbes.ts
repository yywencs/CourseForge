import { computed, onScopeDispose, shallowRef } from 'vue'

import {
  queryAdminReadiness,
  queryAdminStatus,
  queryApiHealth,
  queryApiReadiness,
} from '@/api/system'
import type { ServiceProbe } from '@/types/api'

export function useServiceProbes() {
  const probes = shallowRef<ServiceProbe[]>([
    {
      name: '学生 API',
      description: '负责 JWT 鉴权和选课请求编排',
      status: 'unknown',
      detail: '等待检测',
    },
    {
      name: '核心依赖',
      description: 'MySQL、Redis、RabbitMQ 就绪状态',
      status: 'unknown',
      detail: '等待检测',
    },
    {
      name: '教务服务',
      description: '独立 Admin 进程与扩展路由',
      status: 'unknown',
      detail: '等待检测',
    },
  ])
  const isRefreshing = shallowRef(false)
  const healthyCount = computed(
    () => probes.value.filter((item) => item.status === 'healthy').length,
  )

  async function refresh(): Promise<void> {
    isRefreshing.value = true
    const checkedAt = new Date().toISOString()
    const [apiHealth, apiReadiness, adminStatus, adminReadiness] =
      await Promise.allSettled([
        queryApiHealth(),
        queryApiReadiness(),
        queryAdminStatus(),
        queryAdminReadiness(),
      ])

    const apiHealthy =
      apiHealth.status === 'fulfilled' && apiHealth.value.status === 'ok'
    const readiness =
      apiReadiness.status === 'fulfilled' ? apiReadiness.value : undefined
    const adminHealthy =
      adminStatus.status === 'fulfilled' &&
      adminStatus.value.status === 'ok' &&
      adminReadiness.status === 'fulfilled' &&
      adminReadiness.value.status === 'ready'

    probes.value = [
      {
        name: '学生 API',
        description: '负责 JWT 鉴权和选课请求编排',
        status: apiHealthy ? 'healthy' : 'degraded',
        detail: apiHealthy ? '进程存活' : '无法连接 /healthz',
        checkedAt,
      },
      {
        name: '核心依赖',
        description: 'MySQL、Redis、RabbitMQ 就绪状态',
        status:
          readiness?.status === 'ready'
            ? 'healthy'
            : readiness
              ? 'degraded'
              : 'unknown',
        detail:
          readiness?.status === 'ready'
            ? '全部依赖已就绪'
            : readiness?.failed_checks?.join('、') || '未获取就绪状态',
        checkedAt,
      },
      {
        name: '教务服务',
        description: '独立 Admin 进程与扩展路由',
        status: adminHealthy ? 'healthy' : 'degraded',
        detail: adminHealthy ? 'Admin 服务已就绪' : 'Admin 服务未连接',
        checkedAt,
      },
    ]
    isRefreshing.value = false
  }

  void refresh()
  const timer = window.setInterval(() => {
    void refresh()
  }, 15_000)
  onScopeDispose(() => window.clearInterval(timer))

  return {
    probes,
    healthyCount,
    isRefreshing,
    refresh,
  }
}
