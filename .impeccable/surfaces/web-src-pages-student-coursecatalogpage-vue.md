---
version: 1
slug: "web-src-pages-student-coursecatalogpage-vue"
primary_target: "web/src/pages/student/CourseCatalogPage.vue"
related_targets: ["web/src/features/catalog/CourseCatalogPanel.vue","web/src/components/CourseCard.vue"]
---

# 选课中心界面简报

- 范围：学生端选课中心，并作为学生端、登录页和教务端共享视觉系统的起点。
- 模式：Operate。学生需要高频扫描课程信息、判断冲突与名额，并立即选课或候补。
- 主要行动：选择教学班；名额已满时加入候补。
- 内容要求：课程名、代码、教师、时间、地点、学分、容量、简介和状态必须清晰；视频预览和直播宣讲是课程决策入口。
- 方向：校园讲座海报墙 / 课程节目单。批准构图为 `.impeccable/mocks/courseforge-c-approved.png`。
- 记忆点：课程列表像一组可操作的校园节目条目，信号橙直播区与学术蓝选课动作构成稳定双重识别。

## 实现清单

| 区域 | 构图承诺 | 实现媒介 |
| --- | --- | --- |
| 学生导航 | 墨黑纵向工具栏，图标与文字保持键盘可达 | Vue、语义导航、Lucide 图标 |
| 标题与搜索 | 大标题紧邻搜索和学期语境 | HTML/CSS |
| 课程列表 | 编号、课程信息、媒体入口和动作同层 | Vue、HTML/CSS |
| 视频预览 | 16:9 媒体位，真实地址可播放，无地址时明确空状态 | 原生 video、Vue 对话框 |
| 直播预告 | 右侧信号橙节目块，当前只做真实的未来入口说明 | Vue、HTML/CSS |
| 选课摘要 | 右侧紧凑状态列表，链接到我的选课 | Vue、RouterLink |
| 主要动作 | 名额旁的实心学术蓝按钮，候补使用描边信号橙 | 原生 button |
| 响应式 | 桌面双栏；平板侧栏下移；手机单列并保留底部导航 | CSS media queries |

## 不应字面照搬

- 草案中的课程封面、教师头像、数量与日程均为构图占位，不作为产品事实。
- 不添加未接入服务的“正在直播”声明，只呈现清楚标注的功能预留状态。
