import { createRouter, createWebHistory } from 'vue-router'

import AdminLayout from '@/layouts/AdminLayout.vue'
import StudentLayout from '@/layouts/StudentLayout.vue'
import { useSessionStore } from '@/stores/session'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/student/courses',
    },
    {
      path: '/connect',
      name: 'connect',
      component: () => import('@/pages/auth/ConnectPage.vue'),
      meta: { title: '连接学生身份', public: true },
    },
    {
      path: '/student',
      component: StudentLayout,
      meta: { requiresStudent: true },
      children: [
        { path: '', redirect: '/student/courses' },
        {
          path: 'courses',
          name: 'student-courses',
          component: () => import('@/pages/student/CourseCatalogPage.vue'),
          meta: { title: '选课中心' },
        },
        {
          path: 'enrollments',
          name: 'student-enrollments',
          component: () => import('@/pages/student/MyEnrollmentsPage.vue'),
          meta: { title: '我的选课' },
        },
        {
          path: 'schedule',
          name: 'student-schedule',
          component: () => import('@/pages/student/SchedulePage.vue'),
          meta: { title: '本学期课表' },
        },
        {
          path: 'account',
          name: 'student-account',
          component: () => import('@/pages/student/AccountPage.vue'),
          meta: { title: '身份设置' },
        },
      ],
    },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { public: true },
      children: [
        {
          path: '',
          name: 'admin-dashboard',
          component: () => import('@/pages/admin/AdminDashboardPage.vue'),
          meta: { title: '运行概览' },
        },
        {
          path: 'courses',
          name: 'admin-courses',
          component: () => import('@/pages/admin/CourseManagementPage.vue'),
          meta: { title: '课程能力' },
        },
        {
          path: 'enrollments',
          name: 'admin-enrollments',
          component: () => import('@/pages/admin/EnrollmentMonitorPage.vue'),
          meta: { title: '选课能力' },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/student/courses',
    },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach((to) => {
  const session = useSessionStore()
  if (to.matched.some((record) => record.meta.requiresStudent) && !session.isAuthenticated) {
    return { name: 'connect', query: { redirect: to.fullPath } }
  }
  if (to.name === 'connect' && session.isAuthenticated) {
    return { name: 'student-courses' }
  }
  return true
})

router.afterEach((to) => {
  document.title = `${String(to.meta.title || '选课系统')} · CourseForge`
})

export default router
