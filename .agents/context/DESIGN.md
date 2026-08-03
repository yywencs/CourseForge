---
name: CourseForge
description: 以校园课程节目单为视觉母题的可信高校选课系统
colors:
  ink: "#252a2f"
  ink-soft: "#424a51"
  canvas: "#f5f4f1"
  surface: "#ffffff"
  surface-muted: "#efefeb"
  academic-blue: "#506d8f"
  academic-blue-deep: "#405d7e"
  academic-blue-pale: "#edf2f6"
  signal-orange: "#a96f52"
  signal-orange-pale: "#f7efea"
  success: "#39735c"
  warning: "#8a642b"
  danger: "#a34c46"
  muted: "#687078"
  line: "#c8c8c2"
  line-soft: "#e4e3de"
typography:
  display:
    fontFamily: "Archivo Variable, Noto Sans SC Variable, sans-serif"
    fontSize: "clamp(38px, 5vw, 56px)"
    fontWeight: 680
    lineHeight: 1.12
    letterSpacing: "-0.025em"
    fontVariation: "\"wdth\" 82, \"wght\" 680"
  headline:
    fontFamily: "Archivo Variable, Noto Sans SC Variable, sans-serif"
    fontSize: "clamp(32px, 4vw, 46px)"
    fontWeight: 680
    lineHeight: 1.12
    letterSpacing: "-0.025em"
    fontVariation: "\"wdth\" 82, \"wght\" 680"
  title:
    fontFamily: "Archivo Variable, Noto Sans SC Variable, sans-serif"
    fontSize: "clamp(21px, 2vw, 27px)"
    fontWeight: 660
    lineHeight: 1.2
    letterSpacing: "-0.02em"
    fontVariation: "\"wdth\" 82, \"wght\" 660"
  body:
    fontFamily: "Noto Sans SC Variable, PingFang SC, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.7
    letterSpacing: "normal"
  label:
    fontFamily: "Noto Sans SC Variable, PingFang SC, sans-serif"
    fontSize: "11px"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "normal"
  mono:
    fontFamily: "SFMono-Regular, Cascadia Code, Menlo, monospace"
    fontSize: "10px"
    fontWeight: 750
    lineHeight: 1.4
    letterSpacing: "0.04em"
rounded:
  tag: "5px"
  control: "8px"
  field: "10px"
  panel: "14px"
  pill: "999px"
spacing:
  xs: "5px"
  sm: "8px"
  md: "12px"
  lg: "18px"
  xl: "24px"
  2xl: "32px"
  3xl: "44px"
components:
  button-primary:
    backgroundColor: "{colors.academic-blue}"
    textColor: "{colors.surface}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "9px 10px"
    height: "40px"
  button-primary-hover:
    backgroundColor: "{colors.academic-blue-deep}"
    textColor: "{colors.surface}"
    rounded: "{rounded.control}"
  button-waitlist:
    backgroundColor: "transparent"
    textColor: "{colors.signal-orange}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "9px 10px"
    height: "40px"
  input-default:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "12px 13px"
    height: "50px"
  card-course:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.panel}"
  nav-active:
    backgroundColor: "{colors.academic-blue}"
    textColor: "{colors.surface}"
    typography: "{typography.label}"
    padding: "10px 11px"
    height: "46px"
---

# Design System: CourseForge

## Overview

**Creative North Star: "校园课程节目单（Campus Course Programme）"**

CourseForge 把高校选课组织成一张安静、清晰的校园课程节目单：暖白纸面承载信息，柔和灰蓝标出可信动作，陶土色只提示直播与候补。它拒绝通用教务表格和圆角 SaaS 仪表盘的无差别容器感，以适中的页面标题、细分隔线、连续课程条目与决策侧栏，让学生先理解课程，再靠近名额和动作。

同一视觉语言覆盖三个工作场景：学生端强调课程判断与稳定反馈；登录页以浅灰蓝建立入口氛围；教务端使用浅色导航、纸面面板和紧凑编号结构，提高运行扫描效率。界面直接、可信、克制；装饰从真实任务结构中长出，不伪造学校品牌、教师照片、课程封面或直播状态。

**Key Characteristics:**

- 暖白纸面、浅色导航和清晰的一像素分隔线
- 柔和灰蓝主动作与少量陶土色信号形成稳定双重识别
- 中等尺度标题、可靠中文黑体与紧凑等宽编号协作
- 学生端节目条目、登录页海报、教务端运行清单共享同一语法
- 扁平为默认，仅对真正浮起的模态层使用强阴影

## Colors

调色板以暖白和中性灰为主，灰蓝只服务主动作，陶土色只服务内容信号；状态色只表达真实系统状态。

### Primary

- **学术蓝（Academic Blue）**：用于主按钮、当前导航、视频入口、轮次板和关键数量，表达可信、稳定、可执行。
- **深学术蓝（Deep Academic Blue）**：只用于学术蓝交互面的悬停反馈。
- **淡学术蓝（Pale Academic Blue）**：用于轻量选中、预览条目悬停和课程块的低强度强调。

### Secondary

- **信号橙（Signal Orange）**：用于直播预留、候补动作、海报重点字和交替节目编号；它是注意信号，不替代主动作。
- **淡信号橙（Pale Signal Orange）**：用于橙色语义的浅背景与温和提示。

### Neutral

- **墨黑（Programme Ink）**：标题、骨架导航、强分隔和面板标题栏。
- **柔墨（Soft Ink）**：课程元信息等次级但仍需清晰阅读的内容。
- **暖纸（Warm Canvas）**：全局背景，保持校园印刷品的温度。
- **纸白（Paper Surface）**：表单、课程列表、工具栏与工作面板。
- **旧纸灰（Muted Paper）**：禁用控件、空预览和低优先级容器。
- **正文灰（Editorial Muted）**：说明、辅助标签与元数据。
- **铅字线 / 淡铅字线（Lead Line / Soft Lead Line）**：分别用于结构边界和内部行分隔。

### Tertiary

- **成功绿、警示棕与危险红**：只表示完成、风险和失败等真实状态；必须同时配合文字、图标或形态，不单独依赖颜色。

### Named Rules

**The Two-Ink Rule.** 学术蓝负责主要决策，信号橙负责直播、候补与节目提示；不要让两者竞争同一个主动作。

**The Honest Status Rule.** 状态色必须对应真实数据，并始终伴随可读文字或图标。

## Typography

**Display Font:** Archivo Variable（拉丁字形），回退到 Noto Sans SC Variable 与系统无衬线体

**Body Font:** Noto Sans SC Variable，回退到 PingFang SC 与系统无衬线体

**Label/Mono Font:** SFMono-Regular，回退到 Cascadia Code 与 Menlo

**Character:** 拉丁标题使用 Archivo Variable 的压缩字宽与高字重，形成节目海报的强识别；中文标题实际由 Noto Sans SC Variable 承担，Archivo 不覆盖中文字形。正文保持克制、紧凑和高可读，课程代码与小型编号使用等宽字体建立账目感。

### Hierarchy

- **Display**（中高字重、舒展行高）：课程目录与登录入口的主标题；允许两行排布，但不再依赖高饱和色或超大字号建立层级。
- **Headline**（高字重、单倍行高）：学生页与教务页的页面级标题。
- **Title**（高字重、紧凑行高）：课程名、面板名和关键区块标题。
- **Body**（常规字重、宽松行高）：说明与课程简介；正文行长通常不超过 60–70ch。
- **Label**（加粗、小字号）：按钮、导航、字段标签、状态与元信息。
- **Mono**（加粗、小字号、轻微字距）：课程代码、申请编号和机器可读标识，不承载长句。

### Named Rules

**The Script-Aware Display Rule.** Archivo 只负责拉丁字形；中文标题由 Noto Sans SC 承载，不得声称中文使用 Archivo，也不要以图片字替代真实中文文本。

**The Poster, Not a Billboard Rule.** 超大标题只建立页面入口和节目层级，操作密集区回到紧凑标题与标签，避免整页持续高声量。

## Layout

桌面学生端采用固定浅色侧栏与流式内容舞台：主内容容器最大宽度 1360px，左右各保留 32px 安全区，目录页以课程主栏和 320px 决策侧栏组成双栏。课程条目把编号、课程信息、媒体入口和选课决策放在同一水平层，主要动作紧邻名额状态。登录页使用不对称双栏，以浅灰蓝入口区区别登录表单；教务端使用 238px 浅色侧栏和最大 1280px 的高密度工作区。

间距以 8–12px 的紧凑内部节奏和 18–44px 的区块节奏组合，分隔线承担对齐，不靠大量卡片留白。1100px 附近，目录侧栏移到主栏下并排成两列；学生导航在 1050px 收窄为图标轨，在 680px 改为固定底部导航。教务端在 800px 收窄侧栏，在 580px 改为顶部三项导航；登录页在 900px 变成单列。320px 是全局最小视口宽度。

**The Decision-Proximity Rule.** 名额、当前状态与主要动作保持在同一视觉单元；响应式重排也不能把动作与判断依据拆散。

## Elevation & Depth

系统以扁平纸面和色块叠层为主：常规卡片不使用阴影，深度来自墨黑边框、分隔线、背景色差和粘性导航。只有原生模态对话框真正浮到页面上方；少数表单焦点使用低强度蓝色环，而不是卡片阴影。

### Shadow Vocabulary

- **Flat Card**（无阴影）：课程、课表、候补和教务面板的默认状态。
- **Field Focus**（低强度蓝色三像素环）：仅在输入焦点时出现，配合边框转为学术蓝。
- **Modal Lift**（大范围深色环境阴影）：仅用于课程预览对话框，必须与深色遮罩共同出现。

### Named Rules

**The Flat-by-Default Rule.** 纸面容器依靠边框和色阶分层；除模态层与键盘焦点外，不添加悬浮阴影。

## Shapes

形态以轻度圆角的纸张容器和明确直角信号块并存。主面板使用 14px 柔和圆角，输入与工具控件使用 8–10px 圆角，小标签使用 5px；播放键、头像和关闭按钮使用完整圆形。品牌标记、课程编号块、选中导航和节目色条保持直角，避免把所有元素统一成胶囊。

边框是视觉骨架：面板外框多为一像素墨线，页面标题分隔可加重到两像素，内部行使用淡铅字线。溢出内容在面板圆角内裁切，保留像装订节目单一样的完整轮廓。

**The Selective Radius Rule.** 圆角只用于可容纳内容的纸面和控件；品牌、编号、媒体信号与导航色块优先保持直角。

## Components

### Buttons

- **Shape:** 主要表单和决策按钮使用轻度圆角；列表媒体入口可保持直角以延续节目条目。
- **Primary:** 学术蓝底、纸白字、加粗小标签；用于登录、选课和确定性推进。
- **Hover / Focus:** 悬停转为深学术蓝；键盘焦点保留三像素可见轮廓，禁用态转为旧纸灰并显示不可用光标。
- **Waitlist / Ghost:** 候补使用透明底、信号橙描边和文字；次级工具按钮使用纸白底墨线，激活时反转为墨黑底纸白字。

### Chips

- **Style:** 课程标签使用细铅字边框、小圆角与正文灰；状态徽章可以使用全圆胶囊，但必须包含状态文字或状态点。
- **State:** 成功、等待和危险状态使用各自语义色与浅背景；标签本身不冒充交互按钮。

### Cards / Containers

- **Corner Style:** 主面板使用统一柔和圆角，内部节目行不重复加圆角。
- **Background:** 纸白放在暖纸画布上；墨黑标题栏和蓝橙信号区提供层级。
- **Shadow Strategy:** 常态无阴影，遵循 Flat-by-Default Rule。
- **Border:** 外框用墨线，内部行用铅字线或淡铅字线。
- **Internal Padding:** 紧凑行使用 12–20px，页面级区块使用 24–44px。

### Inputs / Fields

- **Style:** 纸白背景、单像素墨线和轻度圆角；搜索框可把图标、输入与结果计数放在同一行。
- **Focus:** 边框转为学术蓝并显示低强度蓝色焦点环；全局 `:focus-visible` 始终可见。
- **Error / Disabled:** 错误以危险红文字、浅红底与说明文案共同表达；禁用转为旧纸灰，保留可读标签。

### Navigation

学生端和教务端共享浅色导航轨道：默认项使用中性灰，当前项用淡灰蓝底与灰蓝文字标记，不再依赖大面积深色反转。窄屏先压缩为图标轨，手机端再转成底部学生导航或顶部教务导航；任何形态都保留语义化链接、可见标签和键盘焦点。

### Course Programme Entry

课程条目是签名组件：大号两位编号、课程代码与标签、课程说明和元信息、真实媒体入口、学分与名额、主动作形成一条可扫描节目。只有存在真实 `videoUrl` 时才显示可预览入口；无地址时显示明确空状态。直播未接入时只能显示“功能预留”，不得出现“正在直播”、虚构开始时间或可用提醒动作。

### Course Preview Dialog

课程预览使用原生模态对话框和深色 16:9 媒体区。直接 MP4/WebM 地址使用原生视频控件；其他真实内容地址显示外部打开说明与链接。关闭按钮保持高对比圆形，Escape、遮罩点击和焦点归还行为必须保留。

## Do's and Don'ts

### Do:

- **Do** 用节目编号、细分隔线和同层决策信息组织课程与教务清单。
- **Do** 让学术蓝承担主要动作，让信号橙承担直播、候补和时间敏感提示。
- **Do** 在学生端、登录页和教务端复用暖纸、墨线、压缩标题和直角信号块。
- **Do** 只有存在真实 `videoUrl` 时才显示可预览入口；直播未接入时只能显示“功能预留”。
- **Do** 保持键盘焦点可见、状态含文字，并尊重减少动态效果的系统偏好。

### Don't:

- **Don't** 把界面退化成通用教务表格、等权卡片网格或圆角 SaaS 仪表盘。
- **Don't** 伪造学校品牌、课程封面、教师照片、课程成效、直播时间或“正在直播”状态。
- **Don't** 给常规纸面容器添加漂浮阴影，或把品牌标记、编号和导航色块全部胶囊化。
- **Don't** 用 Archivo 描述中文标题字形；中文由 Noto Sans SC 回退承担。
- **Don't** 只用颜色表达选课、候补、成功、警告或错误状态。
