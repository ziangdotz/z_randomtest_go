
<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import * as Runtime from '../wailsjs/runtime'
import appLogoUrl from './assets/images/appLogo.svg'
import RandomtestMain from './components/RandomtestMain.vue';
import About from './components/About.vue';

/* 延迟函数 */
const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

const appName = "随机数检测工具"

const headerHeight = 36 // CSS 中定义的 .app_top 高度

const handleResizeWindow = (width, height) => {
	Runtime.WindowSetSize(Math.floor(width), Math.floor(height + headerHeight));
	Runtime.WindowCenter();
};

// const filePath = ref('')
// const fileName = computed(() => filePath.value.split(/[\\/]/).pop());
// const mediaUrl = computed(() => {
// 	const origin = window.location.origin;
// 	const formattedPath = filePath.value.replace(/\\/g, '/'); // 替换反斜杠并对特殊字符编码（防止中文路径失效）
// 	return `${origin}/media-stream/${formattedPath}`; // 构造访问地址，对应后端的 /media-stream/
// });

const isMaximise = ref(false)
const initWidthAndHeight = ref({w: 0, h: 0})
/* 全屏切换逻辑 */
const windowToggleMaximise = async () => {
	if (isMaximise.value) {
		await Runtime.WindowUnmaximise();
		isMaximise.value = false
	} else {
		await Runtime.WindowMaximise();
		isMaximise.value = true
	}
}

onMounted(() => {
	// Runtime.EventsOn('wails_file', async (path) => {
	// 	filePath.value = path
	// })
})

const isMoreDropMenuOpen = ref(false) // 控制 Header 更多按钮左下角菜单的状态
const isAboutPanelOpen = ref(false)
const activeTheme = ref('dark')
const isDarkTheme = computed(() => activeTheme.value === 'dark')
const showGlobalOverlay = computed(() => isMoreDropMenuOpen.value || isAboutPanelOpen.value)

const applyTheme = (theme) => {
	document.documentElement.setAttribute('data-theme', theme)
	activeTheme.value = theme
	localStorage.setItem('app-theme', theme)
}

const toggleTheme = () => {
	const nextTheme = activeTheme.value === 'dark' ? 'light' : 'dark'
	applyTheme(nextTheme)
}

const toggleMenu = () => {
	isMoreDropMenuOpen.value = !isMoreDropMenuOpen.value
}
const handleMoreDropMenuAction = async (action) => {
	isMoreDropMenuOpen.value = false;
	await sleep(250);
	if (action === 'newWindow') {
		/* 获取当前缩放倍率 */
		const windowDpr = window.devicePixelRatio || 1;
		/* 调用 Go 后端暴露的 NewWindow 方法 */
		if (window.go?.main?.App) {
			window.go.main.App.NewWindow(windowDpr).catch(err => {
				console.error("无法执行 NewWindow:", err)
			})
		}
	} else if (action === 'aboutApp') {
		isAboutPanelOpen.value = true
	} else if (action === 'toggleTheme') {
		toggleTheme()
	} else if (action === 'reload') {
		Runtime.WindowReloadApp(); // 重新加载应用程序前端
	}
	isMoreDropMenuOpen.value = false
}

const closeAboutPanel = () => {
	isAboutPanelOpen.value = false
}

const handleOverlayClick = () => {
	if (isMoreDropMenuOpen.value) {
		isMoreDropMenuOpen.value = false
		return
	}
	if (isAboutPanelOpen.value) {
		isAboutPanelOpen.value = false
	}
}

const setOverlayScrollLock = (locked) => {
	const action = locked ? 'add' : 'remove'
	document.documentElement.classList[action]('overlay-open')
	document.body.classList[action]('overlay-open')
}

watch(showGlobalOverlay, (locked) => {
	setOverlayScrollLock(locked)
}, { immediate: true })

onBeforeUnmount(() => {
	setOverlayScrollLock(false)
})

onMounted(() => {
	const savedTheme = localStorage.getItem('app-theme')
	if (savedTheme === 'light' || savedTheme === 'dark') {
		applyTheme(savedTheme)
		return
	}

	const existingTheme = document.documentElement.getAttribute('data-theme')
	if (existingTheme === 'light' || existingTheme === 'dark') {
		activeTheme.value = existingTheme
		return
	}

	applyTheme('dark')
})

function minimiseWindow() {
	Runtime.WindowMinimise();
}

function closeWindow() {
	if(isMaximise.value) {
		Runtime.WindowUnmaximise();
		isMaximise.value = false
	}
	Runtime.Quit();
}
</script>

<template>
	<div class="app"
		tabindex="0">
		<header class="app_top">
			<div class="header_left_group">
				<div class="info">
					<div class="app_logo">
						<img class="app_logo_img"
							:src="appLogoUrl"
							alt="App Logo" />
					</div>
					<span class="app_name_text">{{ appName }}</span>
				</div>
			</div>
			<div class="header_right_group">
				<div class="all_icon_btn_group">
					<div class="all_icon_btn_group_more_menu_wrapper">
						<button class="icon_btn"
							@click="toggleMenu"
							title="更多">
							<svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
								<circle cx="12" cy="12" r="2"></circle>
								<circle cx="19" cy="12" r="2"></circle>
								<circle cx="5" cy="12" r="2"></circle>
							</svg>
						</button>

						<Transition name="dropdown_menu_zoom">
							<div v-if="isMoreDropMenuOpen" class="dropdown_menu">
								<div class="dropdown_menu_list">
									<button class="dropdown_menu_list_item"
										@click="handleMoreDropMenuAction('aboutApp')">
										<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
											stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
											<circle cx="12" cy="12" r="10"></circle>
											<line x1="12" y1="16" x2="12" y2="12"></line>
											<line x1="12" y1="8" x2="12.01" y2="8"></line>
										</svg>
										关于应用
									</button>
									<button class="dropdown_menu_list_item"
										@click="handleMoreDropMenuAction('newWindow')">
										<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
											stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
											<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-14a2 2 0 0 1-2-2v-4"></path>
											<rect x="3" y="3" width="12" height="12" rx="2" ry="2"></rect>
											<line x1="9" y1="9" x2="15" y2="15"></line>
										</svg>
										新建窗口
									</button>
									<button class="dropdown_menu_list_item"
										@click="handleMoreDropMenuAction('toggleTheme')">
										<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
											stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
											<g v-if="isDarkTheme">
												<circle cx="12" cy="12" r="4"></circle>
												<line x1="12" y1="2" x2="12" y2="5"></line>
												<line x1="12" y1="19" x2="12" y2="22"></line>
												<line x1="2" y1="12" x2="5" y2="12"></line>
												<line x1="19" y1="12" x2="22" y2="12"></line>
												<line x1="4.9" y1="4.9" x2="7" y2="7"></line>
												<line x1="17" y1="17" x2="19.1" y2="19.1"></line>
												<line x1="17" y1="7" x2="19.1" y2="4.9"></line>
												<line x1="4.9" y1="19.1" x2="7" y2="17"></line>
											</g>
											<path v-else d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"></path>
										</svg>
										切换主题
									</button>
									<div class="dropdown_menu_divider"></div>
									<button class="dropdown_menu_list_item danger"
										@click="handleMoreDropMenuAction('reload')">
										<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
											stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
											<path d="M21 2v6h-6"></path>
											<path d="M3 12a9 9 0 0 1 15-6.7L21 8"></path>
											<path d="M3 22v-6h6"></path>
											<path d="M21 12a9 9 0 0 1-15 6.7L3 16"></path>
										</svg>
										重新加载
									</button>

									<!--<button class="dropdown_menu_list_item"
										@click="handleMoreDropMenuAction('appSetting')">
										<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor"
											stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
											<circle cx="12" cy="12" r="3"></circle>
											<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
										</svg>
										设置
									</button> -->
								</div>
							</div>
						</Transition>
					</div>

					<button class="icon_btn"
						@click="minimiseWindow"
						title="缩小到任务栏">
						<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<line x1="5" y1="12" x2="19" y2="12"></line>
						</svg>
					</button>

					<button v-if="!isMaximise"
						class="icon_btn"
						@click="windowToggleMaximise"
						title="全屏">
						<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.2">
							<rect x="5" y="5" width="14" height="14" rx="1.5" />
						</svg>
					</button>
					<button v-else
						class="icon_btn"
						@click="windowToggleMaximise"
						title="取消全屏">
						<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.5">
							<path d="M8 8V6a1.5 1.5 0 0 1 1.5-1.5h8A1.5 1.5 0 0 1 19 6v8a1.5 1.5 0 0 1-1.5 1.5H16" stroke-linecap="round" stroke-linejoin="round" />
							<rect x="5" y="8" width="11" height="11" rx="1.5" stroke-linecap="round" stroke-linejoin="round" />
						</svg>
					</button>

					<button class="icon_btn close_btn"
						@click="closeWindow"
						title="关闭">
						<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
							<line x1="18" y1="6" x2="6" y2="18"></line>
							<line x1="6" y1="6" x2="18" y2="18"></line>
						</svg>
					</button>
				</div>
			</div>
		</header>

		<main class="app_viewer">
			<RandomtestMain />
		</main>

		<Transition name="about_panel_slide">
			<About v-if="isAboutPanelOpen"
				@close="closeAboutPanel" />
		</Transition>

		<Transition name="overlay_fade">
			<div v-if="showGlobalOverlay" class="global_overlay"
				@click="handleOverlayClick"></div>
		</Transition>
	</div>
</template>

<style scoped>
/* 定义统一的动画参数变量 */
:root {
	--menu-anim-time: 0.1s;
	--menu-bezier: cubic-bezier(0.2, 0, 0.1, 0.5);
}

* {
	user-select: none; /* 禁止全局文字选中 标准语法 */
	-ms-user-select: none; /* 禁止全局文字选中 IE/Edge */
	-moz-user-select: none; /* 禁止全局文字选中 Firefox */
	-webkit-user-select: none; /* 禁止全局文字选中 Safari, Chrome, WebView2 */
}

.app {
	position: relative;
	width: 100%;
	height: 100%;
	padding: 0px;
	box-sizing: border-box;
	background-color: var(--window-bg);
	display: flex;
	flex-direction: column;
	overflow: hidden;
}

.app_top {
	--wails-draggable: drag;
	height: 34px;
	padding: 0px;
	background: transparent;
	display: flex;
	align-items: center;
	justify-content: space-between;
	overflow: visible;
	overscroll-behavior: none;
	touch-action: none;
	cursor: grab;
}

.header_left_group {
	--wails-draggable: no-drag;
	min-width: 0px;
	padding: 0px;
	display: flex;
	align-items: center;
	justify-content: flex-start;
}

.header_right_group {
	flex-shrink: 0;
	display: flex;
	align-items: center;
	justify-content: flex-end;
}

.all_icon_btn_group {
	display: flex;
	align-items: center;
	justify-content: flex-end;
}

.info {
	min-width: 0px;
	margin: 0px 10px;
	flex: 1;
	display: flex;
	align-items: center;
	justify-content: left;
	gap: 10px;
	pointer-events: auto;
}
.app_logo {
	width: 22px;
	height: 22px;
	display: flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
}

.app_logo_img {
	width: 100%;
	height: 100%;
	display: flex;
	object-fit: contain; /* 保持比例 */
	pointer-events: none; /* 防止用户右键保存或拖动图片 */
	-webkit-user-drag: none; /* 禁用 Webkit 内核的图片拖拽 */
}

.app_name_text {
	width: 100%;
	display: block;
	color: var(--app-text);
	font-size: 13px;
	font-weight: 500;
	line-height: 1.0;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis; /* 长文件名自动显示省略号 */
}

.icon_btn {
	--wails-draggable: no-drag; /* 禁止按钮区域的拖动，否则点击无效 */
	width: 42px;
	height: 34px;
	padding: 10px 0px;
	background-color: transparent;
	border: none;
	color: var(--topbar-icon);
	flex-shrink: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	cursor: pointer;
	transition: background-color 0.2s;
}
.icon_btn:hover {
	background-color: var(--topbar-hover);
	color: var(--topbar-icon-active);
}
.icon_btn svg {
	display: block;
	width: 100%;
	height: 100%;
}
.close_btn:hover {
	background-color: var(--topbar-danger);
	color: var(--danger-text-on-bg);
}

.all_icon_btn_group_more_menu_wrapper {
	position: relative;
	display: flex;
	align-items: center;
}

.dropdown_menu {
	position: absolute;
	top: 36px; /* top 36px */
	right: 0; /* 右边对齐 */
	width: max-content; /* 宽度根据内容最大长度自动调整 */
	min-width: 100px;
	height: max-content; /* 高度根据内容最大长度自动调整 */
	max-height: 200px;
	overflow-y: auto; /* 支持滚动 */
	scrollbar-width: none; /* 隐藏滚动条 */
	-ms-overflow-style: none; /* 隐藏滚动条 IE 10+ */
	background-color: var(--window-bg);
	backdrop-filter: blur(12px);
	-webkit-backdrop-filter: blur(12px); /* 兼容性其他内核 */
	background-clip: padding-box; /* 确保背景模糊只作用于内容区域 */
	border: 1px solid var(--topbar-menu-border);
	border-radius: 8px;
	box-shadow: var(--topbar-menu-shadow);
	padding: 0px;
	z-index: 1001;
}

.dropdown_menu_list_item {
	width: 100%;
	padding: 8px 10px;
	background: transparent;
	border: none;
	border-radius: 4px;
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 10px;
	color: var(--topbar-icon);
	font-size: 14px;
	font-weight: 400;
	text-align: center;
	line-height: 1.2;
	transition: background 0.2s;
	cursor: pointer;
}
.dropdown_menu_list_item:hover {
	background-color: var(--topbar-hover);
	color: var(--topbar-icon-active);
}
.dropdown_menu_list_item.danger {
	color: var(--topbar-danger);
}
.dropdown_menu_list_item.danger:hover {
	background-color: rgba(255, 82, 82, 0.2);
	color: var(--topbar-danger);
}

.dropdown_menu_list_item svg {
	display: block;
	flex-shrink: 0;
}

.dropdown_menu_divider {
	margin: 4px 5px;
	height: 1px;
	background-color: var(--topbar-menu-divider);
}

/* 菜单动画 */
.dropdown_menu_zoom-enter-active,
.dropdown_menu_zoom-leave-active {
	transition:
		opacity var(--menu-anim-time) var(--menu-bezier),
		transform var(--menu-anim-time) var(--menu-bezier);
}

.dropdown_menu_zoom-enter-from,
.dropdown_menu_zoom-leave-to {
	opacity: 0;
	transform: translateY(-5px) scale(0.9); /* 略微缩小并上移 */
	transform-origin: top right; /* 动画从右上角开始 */
}

.app_viewer {
	width: 100%;
	background-color: transparent;
	flex: 1;
	display: flex;
	justify-content: stretch;
	align-items: stretch;
	min-height: 0;
	overflow: hidden;
	overscroll-behavior: none;
}

/******** 全局遮罩 ********/
.global_overlay {
	position: absolute;
	inset: 0;
	z-index: 1000;
	--wails-draggable: no-drag;
	background-color: var(--overlay-bg);
	backdrop-filter: blur(0.7px) saturate(100%) brightness(100%);
	-webkit-backdrop-filter: blur(0.7px) saturate(100%) brightness(100%); /* 兼容性其他内核 */
	background: var(--overlay-bg);
	pointer-events: auto; /* 确保拦截点击 */
	touch-action: none;
}

/* 全局遮罩动画 */
.overlay_fade-enter-active {
	transition: opacity 100ms cubic-bezier(0.22, 1, 0.36, 1);
}

.overlay_fade-leave-active {
	transition: opacity 150ms cubic-bezier(0.32, 0, 0.67, 0);
}

.overlay_fade-enter-from,
.overlay_fade-leave-to {
	opacity: 0;
}


.about_panel_slide-enter-active {
	transition:
		opacity 180ms cubic-bezier(0.22, 1, 0.36, 1),
		transform 220ms cubic-bezier(0.16, 1, 0.3, 1);
	will-change: opacity, transform;
	transform-origin: top right;
}

.about_panel_slide-leave-active {
	transition:
		opacity 180ms cubic-bezier(0.32, 0, 0.67, 0),
		transform 200ms cubic-bezier(0.32, 0, 0.67, 0);
	will-change: opacity, transform;
	transform-origin: top right;
}

.about_panel_slide-enter-from {
	opacity: 0;
	transform: translate3d(16px, 0, 0) scale(0.992);
}

.about_panel_slide-enter-to,
.about_panel_slide-leave-from {
	opacity: 1;
	transform: translate3d(0, 0, 0) scale(1);
}

.about_panel_slide-leave-to {
	opacity: 0;
	transform: translate3d(10px, 0, 0) scale(0.996);
}

@media (prefers-reduced-motion: reduce) {
	.about_panel_slide-enter-active,
	.about_panel_slide-leave-active {
		transition: opacity 120ms linear;
	}

	.about_panel_slide-enter-from,
	.about_panel_slide-enter-to,
	.about_panel_slide-leave-from,
	.about_panel_slide-leave-to {
		transform: none;
	}
}
</style>
