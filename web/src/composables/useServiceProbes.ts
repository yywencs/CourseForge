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
      name: '学生服务',
      description: '学生登录与选课',
      status: 'unknown',
      detail: '等待检测',
    },
    {
      name: '基础服务',
      description: '系统运行所需服务',
      status: 'unknown',
      detail: '等待检测',
    },
    {
      name: '教务服务',
      description: '教务管理功能',
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
        name: '学生服务',
        description: '学生登录与选课',
        status: apiHealthy ? 'healthy' : 'degraded',
        detail: apiHealthy ? '运行正常' : '暂时无法连接',
        checkedAt,
      },
      {
        name: '基础服务',
        description: '系统运行所需服务',
        status:
          readiness?.status === 'ready'
            ? 'healthy'
            : readiness
              ? 'degraded'
              : 'unknown',
        detail:
          readiness?.status === 'ready'
            ? '运行正常'
            : readiness
              ? '部分服务不可用'
              : '暂未获取状态',
        checkedAt,
      },
      {
        name: '教务服务',
        description: '教务管理功能',
        status: adminHealthy ? 'healthy' : 'degraded',
        detail: adminHealthy ? '运行正常' : '暂时无法连接',
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
