import { createRouter, createWebHistory } from 'vue-router'

import AdminLayout from '@/layouts/AdminLayout.vue'
import StudentLayout from '@/layouts/StudentLayout.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/student/courses',
    },
    {
      path: '/student',
      component: StudentLayout,
      children: [
        {
          path: '',
          redirect: '/student/courses',
        },
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
      ],
    },
    {
      path: '/admin',
      component: AdminLayout,
      children: [
        {
          path: '',
          name: 'admin-dashboard',
          component: () => import('@/pages/admin/AdminDashboardPage.vue'),
          meta: { title: '教务概览' },
        },
        {
          path: 'courses',
          name: 'admin-courses',
          component: () => import('@/pages/admin/CourseManagementPage.vue'),
          meta: { title: '课程与教学班' },
        },
        {
          path: 'enrollments',
          name: 'admin-enrollments',
          component: () => import('@/pages/admin/EnrollmentMonitorPage.vue'),
          meta: { title: '选课申请监控' },
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

router.afterEach((to) => {
  document.title = `${String(to.meta.title || '选课系统')} · CourseForge`
})

export default router
