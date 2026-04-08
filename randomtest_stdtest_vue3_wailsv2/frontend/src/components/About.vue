<script setup>
const emit = defineEmits(['close'])

const closePanel = () => {
	emit('close')
}
</script>

<template>
	<aside class="about_panel" role="dialog" aria-modal="true" aria-label="关于应用">
		<header class="about_header">
			<h2>关于应用</h2>
			<button type="button" class="close_btn" @click="closePanel" aria-label="关闭关于窗口">×</button>
		</header>

		<section class="about_body">
			<div class="about_group">
				<h3>应用信息</h3>
				<p><strong>名称：</strong>随机数检测工具</p>
				<p><strong>版本：</strong>v1.0.1</p>
				<p><strong>描述：</strong>用于 NIST/GM 标准下的随机性检测参数配置、测试执行与结果查看、导出。</p>
			</div>

			<div class="about_group">
				<h3>功能简介</h3>
				<ul>
					<li>支持 NIST 与 GM 标准的检测流程。</li>
					<li>支持查看运行日志与测试结果。</li>
					<li>支持保存测试结果为 CSV 文件。</li>
				</ul>
			</div>

			<div class="about_group">
				<h3>技术栈</h3>
				<p>Vue 3 + Wails v2 + Go</p>
			</div>
		</section>
	</aside>
</template>

<style scoped>
.about_panel {
	position: absolute;
	top: auto;
	right: clamp(8px, 1vw, 14px);
	bottom: clamp(8px, 1vw, 14px);
	z-index: 1002;
	display: flex;
	flex-direction: column;
	width: calc(100vw * 0.4);
	height: calc(100vh * 0.5);
	overflow: hidden;
	border: 1px solid var(--panel-border);
	border-radius: 14px;
	background: linear-gradient(155deg, var(--panel-bg-start) 0%, var(--panel-bg-end) 100%);
	box-shadow: var(--topbar-menu-shadow);
	--wails-draggable: no-drag;
	touch-action: none;
	-webkit-backdrop-filter: blur(8px);
	backdrop-filter: blur(8px);
	backface-visibility: hidden;
	transform: translateZ(0);
	will-change: transform, opacity;
}

.about_header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 14px 16px;
	background: transparent;
	border-bottom: 1px solid var(--topbar-menu-divider);
}

.about_header h2 {
	margin: 0;
	color: var(--app-text);
	font-size: 16px;
	font-weight: 600;
}

.close_btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 30px;
	height: 30px;
	padding: 0;
	border: none;
	border-radius: 6px;
	background: transparent;
	color: var(--topbar-icon);
	font-size: 24px;
	line-height: 1;
	cursor: pointer;
}

.close_btn:hover {
	background: var(--topbar-hover);
	color: var(--topbar-icon-active);
}

.about_body {
	padding: 14px 16px 18px;
	overflow-y: auto;
	background: transparent;
	color: var(--rt-main-text);
	-ms-overflow-style: none;
	scrollbar-width: none;
}

.about_body::-webkit-scrollbar {
	width: 0;
	height: 0;
}

.about_group {
	margin-bottom: 16px;
	padding: 12px 12px 10px;
	border: 1px solid var(--panel-border);
	border-radius: 12px;
	background: linear-gradient(180deg, var(--window-bg) 0%, var(--rt-panel-bg-solid) 100%);
}

.about_group h3 {
	margin: 0 0 8px;
	color: var(--rt-title-text);
	font-size: 14px;
	font-weight: 600;
}

.about_group p {
	margin: 6px 0;
	color: var(--rt-main-text);
	font-size: 13px;
	line-height: 1.5;
}

.about_group strong {
	color: var(--app-text);
}

.about_group ul {
	margin: 0;
	padding-left: 18px;
}

.about_group li {
	margin: 6px 0;
	color: var(--rt-main-text);
	font-size: 13px;
	line-height: 1.5;
}

.about_group li::marker {
	color: var(--topbar-icon);
}

.about_group:last-child {
	margin-bottom: 0;
}
</style>
