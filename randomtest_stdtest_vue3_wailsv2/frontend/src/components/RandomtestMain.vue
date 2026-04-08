<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import * as Runtime from '../wailsjs/runtime'
import { AnalyzeInputPath, GetTestFormDefaults, PickInputPath, SaveResultCSVByDialog, StartRandomnessTest, StopRandomnessTest } from '../wailsjs/go/main/App'

/* state */
const testInputParams = reactive({
	pValueThreshold: '0.01',
	qValueThreshold: '0.0001',
	inputDataPath: 'xxx\\data_0',
	randomnessTestStandard: 'NIST',
	randomnessTestLevel: '2级 (1000000 bits)'
})
const inputPathIsFile = ref(false)

const testSampleStats = reactive({
	sampleSizeBits: 1000000,
	sampleCount: 1000,
	minimumPassCount: 981
})

const runtimeLogText = ref('')
const resultRows = ref([])

const progress = ref(0)
const isTesting = ref(false)
const stopRequestedByUser = ref(false)

const progressTrackRef = ref(null)

/* derived state */
const hasInputDataPath = computed(() => testInputParams.inputDataPath.trim().length > 0)
const hasRuntimeLogText = computed(() => runtimeLogText.value.trim().length > 0)
const hasTableResult = computed(() => {
	return resultRows.value.some((row) => {
		const name = String(row?.name ?? '').trim()
		const p = String(row?.pValue ?? '').trim()
		const q = String(row?.qValue ?? '').trim()
		const result = String(row?.result ?? '').trim()
		return name.length > 0 || p.length > 0 || q.length > 0 || result.length > 0
	})
})
const progressDisplay = computed(() => `${Math.round(progress.value)} %`)
const startBtnText = computed(() => (isTesting.value ? '停止测试' : '开始测试'))

const tableHeaders = ['序号', '测试项', 'P值', 'Q值', '结果']
const tableRows = computed(() => {
	const rows = resultRows.value.map((row, idx) => ({
		id: Number(row?.index ?? idx + 1),
		name: String(row?.name ?? ''),
		p: String(row?.pValue ?? ''),
		q: String(row?.qValue ?? ''),
		result: String(row?.result ?? '')
	}))
	if (rows.length >= 34) {
		return rows
	}
	const pad = Array.from({ length: 34 - rows.length }, (_, i) => ({
		id: rows.length + i + 1,
		name: '',
		p: '',
		q: '',
		result: ''
	}))
	return [...rows, ...pad]
})

/* pure helpers */
const parseLevelNumber = (levelLabel) => {
	const matched = String(levelLabel).match(/(\d+)\s*级/)
	if (!matched) {
		return 2
	}
	const level = Number(matched[1])
	if (Number.isNaN(level) || level < 1 || level > 3) {
		return 2
	}
	return level
}

const parseLevelBits = (levelLabel) => {
	const matched = String(levelLabel).match(/\((\d+)\s*bits\)/i)
	if (!matched) {
		return 1000000
	}
	const bits = Number(matched[1])
	if (Number.isNaN(bits) || bits <= 0) {
		return 1000000
	}
	return bits
}

const formatDurationMs = (elapsedMs) => {
	const ms = Math.max(0, Number(elapsedMs) || 0)
	if (ms < 1000) {
		return `${ms.toFixed(1)} ms`
	}
	const totalSeconds = ms / 1000
	if (totalSeconds < 60) {
		return `${totalSeconds.toFixed(3)} s`
	}
	const minutes = Math.floor(totalSeconds / 60)
	const seconds = totalSeconds % 60
	return `${minutes} min ${seconds.toFixed(3)} s`
}

/* mutators */
const appendRuntimeLog = (message) => {
	const line = String(message ?? '').trimEnd()
	if (!line) {
		return
	}
	if (runtimeLogText.value.length === 0) {
		runtimeLogText.value = line
		return
	}
	runtimeLogText.value = `${runtimeLogText.value}\n${line}`
}

const recalcSampleStats = () => {
	testSampleStats.sampleSizeBits = parseLevelBits(testInputParams.randomnessTestLevel)
	const sampleCount = Number(testSampleStats.sampleCount)
	if (Number.isNaN(sampleCount) || sampleCount <= 0) {
		testSampleStats.minimumPassCount = 0
		return
	}
	testSampleStats.minimumPassCount = Math.ceil(sampleCount * 0.98056)
}

const syncProgressCssVar = () => {
	if (!progressTrackRef.value) {
		return
	}
	progressTrackRef.value.style.setProperty('--rt-progress-width', `${progress.value}%`)
}

const syncSampleStatsFromPath = async () => {
	const inputPath = testInputParams.inputDataPath.trim()
	if (!inputPath || inputPath.includes('xxx\\')) {
		recalcSampleStats()
		return
	}
	try {
		const result = await AnalyzeInputPath({
			inputDataPath: inputPath,
			isFile: inputPathIsFile.value,
			randomnessTestStandard: testInputParams.randomnessTestStandard,
			randomnessTestLevel: parseLevelNumber(testInputParams.randomnessTestLevel),
			pValueThreshold: Number(testInputParams.pValueThreshold)
		})
		inputPathIsFile.value = Boolean(result?.isFile)
		testSampleStats.sampleSizeBits = Number(result?.sampleSizeBits ?? testSampleStats.sampleSizeBits)
		testSampleStats.sampleCount = Number(result?.sampleCount ?? testSampleStats.sampleCount)
		testSampleStats.minimumPassCount = Number(result?.minimumPassSampleCount ?? testSampleStats.minimumPassCount)
	} catch (error) {
		appendRuntimeLog(`[WARN] 路径分析失败: ${error?.message ?? error}`)
		recalcSampleStats()
	}
}

const clearInputDataPath = () => {
	testInputParams.inputDataPath = ''
}

const clearRuntimeLogText = () => {
	runtimeLogText.value = ''
}

const handleSaveResultCsv = async () => {
	if (!hasTableResult.value) {
		appendRuntimeLog('[WARN] 当前没有可保存的表格结果')
		return
	}
	try {
		const savedPath = await SaveResultCSVByDialog(
			testInputParams.randomnessTestStandard,
			testInputParams.randomnessTestLevel
		)
		appendRuntimeLog(`[INFO] 结果已保存到: ${savedPath}`)
	} catch (error) {
		appendRuntimeLog(`[ERROR] 保存 CSV 失败: ${error?.message ?? error}`)
	}
}

/* event handlers */
const handlePickInputPath = async () => {
	try {
		const picked = await PickInputPath()
		if (picked?.path) {
			testInputParams.inputDataPath = picked.path
			inputPathIsFile.value = false
			await syncSampleStatsFromPath()
			appendRuntimeLog(`[INFO] 已选择目录: ${picked.path}`)
		}
	} catch (error) {
		appendRuntimeLog(`[WARN] 选择目录取消或失败: ${error?.message ?? error}`)
	}
}

const handleBackendOutput = (event) => {
	if (!event || typeof event !== 'object') {
		return
	}
	if (event.type === 'progress') {
		progress.value = Number(event.percent ?? 0)
		return
	}
	if (event.type === 'result_rows') {
		resultRows.value = Array.isArray(event.rows) ? event.rows : []
		return
	}
	if (event.type === 'log' && event.message) {
		appendRuntimeLog(event.message)
	}
}

const handleBackendState = (state) => {
	if (!state || typeof state !== 'object' || typeof state.running !== 'boolean') {
		return
	}
	if (!state.running && stopRequestedByUser.value) {
		progress.value = 0
		stopRequestedByUser.value = false
	}
	isTesting.value = state.running
}

const handleStartTest = async () => {
	if (isTesting.value) {
		try {
			stopRequestedByUser.value = true
			await StopRandomnessTest()
			appendRuntimeLog('[INFO] 已发送停止测试请求')
		} catch (error) {
			stopRequestedByUser.value = false
			appendRuntimeLog(`[WARN] 停止请求失败: ${error?.message ?? error}`)
		}
		return
	}
	const pValueThreshold = Number(testInputParams.pValueThreshold)
	const qValueThreshold = Number(testInputParams.qValueThreshold)
	if (Number.isNaN(pValueThreshold) || Number.isNaN(qValueThreshold)) {
		appendRuntimeLog('[ERROR] P/Q 阈值必须是数字')
		return
	}
	if (!testInputParams.inputDataPath.trim()) {
		appendRuntimeLog('[ERROR] 请输入或选择输入目录路径')
		return
	}

	const req = {
		pValueThreshold,
		qValueThreshold,
		inputDataPath: testInputParams.inputDataPath.trim(),
		isFile: inputPathIsFile.value,
		randomnessTestStandard: testInputParams.randomnessTestStandard,
		randomnessTestLevel: parseLevelNumber(testInputParams.randomnessTestLevel),
		sampleCount: Number(testSampleStats.sampleCount)
	}

	progress.value = 0
	isTesting.value = true
	stopRequestedByUser.value = false
	resultRows.value = []
	appendRuntimeLog('[INFO] 开始提交参数并执行测试...')
	const requestStartWallTime = new Date()
	const requestStartPerf = performance.now()

	try {
		await StartRandomnessTest(req)
		const requestEndWallTime = new Date()
		const elapsedMs = performance.now() - requestStartPerf
		appendRuntimeLog(
			`[INFO] 总测试时间: ${formatDurationMs(elapsedMs)}（请求发出: ${requestStartWallTime.toLocaleString()}，结果返回: ${requestEndWallTime.toLocaleString()}）`
		)
		appendRuntimeLog('[INFO] 测试完成')
	} catch (error) {
		appendRuntimeLog(`[ERROR] ${error?.message ?? error}`)
	} finally {
		isTesting.value = false
		progress.value = 0
		stopRequestedByUser.value = false
	}
}

/* lifecycle */
onMounted(async () => {
	Runtime.EventsOn('randomtest_output', handleBackendOutput)
	Runtime.EventsOn('randomtest_state', handleBackendState)
	syncProgressCssVar()
	try {
		const defaults = await GetTestFormDefaults()
		testInputParams.pValueThreshold = String(defaults?.pValueThreshold ?? testInputParams.pValueThreshold)
		testInputParams.qValueThreshold = String(defaults?.qValueThreshold ?? testInputParams.qValueThreshold)
		testInputParams.randomnessTestStandard = defaults?.randomnessTestStandard || testInputParams.randomnessTestStandard
		testInputParams.randomnessTestLevel = defaults?.randomnessTestLevel || testInputParams.randomnessTestLevel
		testInputParams.inputDataPath = defaults?.inputDataPath || testInputParams.inputDataPath
		inputPathIsFile.value = Boolean(defaults?.isFile)
		testSampleStats.sampleCount = Number(defaults?.sampleCount || testSampleStats.sampleCount)
		testSampleStats.sampleSizeBits = Number(defaults?.sampleSizeBits || testSampleStats.sampleSizeBits)
		testSampleStats.minimumPassCount = Number(defaults?.minimumPassSampleCount || testSampleStats.minimumPassCount)
		await syncSampleStatsFromPath()
	} catch (error) {
		appendRuntimeLog(`[WARN] 读取默认参数失败: ${error?.message ?? error}`)
	}
})

watch(
	() => [testInputParams.randomnessTestStandard, testInputParams.randomnessTestLevel],
	() => {
		void syncSampleStatsFromPath()
	}
)

watch(
	() => testInputParams.pValueThreshold,
	() => {
		if (testInputParams.inputDataPath.trim() && !testInputParams.inputDataPath.includes('xxx\\')) {
			void syncSampleStatsFromPath()
			return
		}
		recalcSampleStats()
	}
)

onBeforeUnmount(() => {
	Runtime.EventsOff('randomtest_output', 'randomtest_state')
})

/* watchers */
watch(
	() => [testInputParams.pValueThreshold, testInputParams.randomnessTestLevel, testSampleStats.sampleCount],
	recalcSampleStats,
	{ immediate: true }
)

watch(
	() => progress.value,
	() => {
		syncProgressCssVar()
	},
	{ immediate: true }
)
</script>

<template>
	<section class="randomtest_main">
		<div class="summary_panel">
			<header class="summary_title">
				<span class="summary_dot"></span>
				<h2>结果汇总:</h2>
				<button v-if="hasTableResult" class="summary_save_hint" type="button" @click="handleSaveResultCsv">点击保存</button>
			</header>

			<div class="result_table_wrap">
				<table class="result_table">
					<thead>
						<tr>
							<th v-for="header in tableHeaders" :key="header">{{ header }}</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="row in tableRows" :key="row.id">
							<td>{{ row.id }}</td>
							<td>{{ row.name }}</td>
							<td>{{ row.p }}</td>
							<td>{{ row.q }}</td>
							<td>{{ row.result }}</td>
						</tr>
					</tbody>
				</table>
			</div>
		</div>

		<div class="control_panel">
			<div class="threshold_row">
				<div class="field_pair">
					<label>PValue ≥</label>
					<input v-model="testInputParams.pValueThreshold" type="text" />
				</div>
				<div class="field_pair">
					<label>QValue ≥</label>
					<input v-model="testInputParams.qValueThreshold" type="text" />
				</div>
			</div>

			<div class="selectors_row">
				<div class="field_pair selector_pair">
					<label>测试标准</label>
					<select v-model="testInputParams.randomnessTestStandard">
						<option>NIST</option>
						<option>GM</option>
					</select>
				</div>
				<div class="field_pair selector_pair level_pair">
					<label>测试等级</label>
					<select v-model="testInputParams.randomnessTestLevel">
						<option>1级 (20000 bits)</option>
						<option>2级 (1000000 bits)</option>
						<option>3级 (100000000 bits)</option>
					</select>
				</div>
			</div>

			<div class="path_row">
				<button class="pick_btn" type="button" @click="handlePickInputPath">选择目录</button>
				<div class="selected_path_input_wrap">
					<input v-model="testInputParams.inputDataPath" type="text" />
					<span v-if="hasInputDataPath" class="clear_hint" @click="clearInputDataPath">清除</span>
				</div>
			</div>

			<div class="meta_and_action_row">
				<div class="meta_box">
					<p><span>样本大小 (bits):</span> {{ testSampleStats.sampleSizeBits }} bits</p>
					<p><span>样本数量：</span> {{ testSampleStats.sampleCount }}</p>
					<p><span>最低通过样本量 (≥)：</span> {{ testSampleStats.minimumPassCount }}</p>
				</div>
				<button
					class="start_btn"
					:class="{ 'is-running': isTesting }"
					type="button"
					@click="handleStartTest"
				>
					{{ startBtnText }}
				</button>
			</div>

			<div class="progress_row">
				<div ref="progressTrackRef" class="progress_track">
					<div class="progress_fill"></div>
					<span>{{ progressDisplay }}</span>
				</div>
			</div>

			<div class="terminal_output_row">
				<div class="terminal_output_wrap">
					<textarea
						class="terminal_output_box"
						:value="runtimeLogText"
						wrap="off"
						readonly
						spellcheck="false"
					></textarea>
					<span
						v-if="hasRuntimeLogText"
						class="clear_output_hint"
						@click="clearRuntimeLogText"
					>清除</span>
				</div>
			</div>

		</div>
	</section>
</template>

<style scoped>
/* 1) Page root */
.randomtest_main {
	--wails-draggable: no-drag;
	display: grid;
	grid-template-columns: minmax(0, 5fr) minmax(0, 5fr);
	gap: clamp(8px, 1vw, 14px);
	width: 100%;
	height: 100%;
	min-width: 0;
	padding: clamp(8px, 1vw, 14px);
	box-sizing: border-box;
	overflow: hidden;
	overscroll-behavior: contain;
	background: var(--rt-main-bg-solid);
	color: var(--rt-main-text);
	font-family: "Segoe UI Variable Text", "Segoe UI", "Microsoft YaHei", sans-serif;
}

.summary_panel,
.control_panel {
	min-width: 0;
	min-height: 0;
	padding: clamp(10px, 1vw, 14px);
	box-sizing: border-box;
	border: 1px solid var(--panel-border);
	border-radius: 14px;
	background: var(--rt-panel-bg-solid);
	box-shadow: none;
}

/* 2) Left summary panel */
.summary_panel {
	display: flex;
	flex-direction: column;
	overflow: hidden;
}

.summary_title {
	--wails-draggable: no-drag;
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 8px;
	overflow: hidden;
	overscroll-behavior: none;
	touch-action: none;
}

.summary_dot {
	width: 10px;
	height: 10px;
	border-radius: 3px;
	background: linear-gradient(135deg, var(--rt-title-dot-start), var(--rt-title-dot-end));
	box-shadow: 0 0 8px var(--rt-title-dot-shadow);
}

.summary_title h2 {
	margin: 0;
	color: var(--rt-title-text);
	font-size: clamp(18px, 1.5vw, 26px);
	font-weight: 650;
	line-height: 1;
	letter-spacing: 0.2px;
}

.summary_save_hint {
	margin-left: auto;
	height: 28px;
	padding: 0 10px;
	border: 1px solid var(--field-border);
	border-radius: 8px;
	background: var(--field-bg);
	color: var(--rt-open-file-hint-text);
	font-size: clamp(11px, 0.8vw, 13px);
	font-weight: 600;
	cursor: pointer;
	transition: opacity 0.16s ease, filter 0.16s ease;
}

.summary_save_hint:hover {
	filter: brightness(1.12);
}

.result_table_wrap {
	--wails-draggable: no-drag;
	flex: 1;
	width: 100%;
	max-width: 100%;
	min-width: 0;
	min-height: 0;
	max-height: 100%;
	position: relative;
	isolation: isolate;
	overflow: auto;
	overscroll-behavior: none;
	overscroll-behavior-x: none;
	overscroll-behavior-y: none;
	touch-action: pan-x pan-y;
	contain: layout paint;
	border: 1px solid var(--rt-table-wrap-border);
	border-radius: 0;
	background: var(--rt-table-wrap-bg);
	scrollbar-width: auto;
	scrollbar-color: var(--rt-table-scroll-thumb) var(--rt-table-scroll-track);
}

.result_table_wrap::-webkit-scrollbar {
	width: 10px;
	height: 10px;
}

.result_table_wrap::-webkit-scrollbar-track {
	border-radius: 0;
	background: var(--rt-table-scroll-track);
}

.result_table_wrap::-webkit-scrollbar-thumb {
	border-radius: 0;
	background: var(--rt-table-scroll-thumb);
}

.result_table_wrap::-webkit-scrollbar-thumb:hover {
	background: var(--rt-table-scroll-thumb-hover);
}

.result_table {
	width: max(100%, 560px);
	min-width: 560px;
	border-collapse: collapse;
	background: var(--rt-table-wrap-bg);
	color: var(--rt-table-text);
	font-size: clamp(10px, 0.72vw, 12px);
	transform: translateZ(0);
}

.result_table th,
.result_table td {
	height: clamp(22px, 1.9vh, 26px);
	padding: 0 6px;
	border: 1px solid var(--rt-table-cell-border);
	background: var(--rt-table-cell-bg);
	text-align: left;
	vertical-align: middle;
	white-space: nowrap;
}

.result_table th {
	position: sticky;
	top: 0;
	z-index: 4;
	background: var(--rt-table-head-bg);
	color: var(--rt-table-head-text);
	font-weight: 650;
}

.result_table th:nth-child(1),
.result_table td:nth-child(1) {
	position: sticky;
	left: 0;
	z-index: 3;
	width: 52px;
	min-width: 52px;
	background: var(--rt-table-sticky-bg);
	box-shadow: inset -1px 0 0 var(--rt-table-cell-border);
	text-align: center;
}

.result_table th:nth-child(1) {
	z-index: 6;
	background: var(--rt-table-head-bg);
}

.result_table th:nth-child(2),
.result_table td:nth-child(2) {
	width: 160px;
	min-width: 160px;
	text-align: center;
}

.result_table th:nth-child(3),
.result_table td:nth-child(3),
.result_table th:nth-child(4),
.result_table td:nth-child(4),
.result_table th:nth-child(5),
.result_table td:nth-child(5) {
	text-align: center;
}

/* 3) Right control panel */
.control_panel {
	--action-btn-width: 110px;
	--pair-label-width: 12.5ch;
	--pair-value-width: 12.5ch;
	--row-block-gap: 8px;
	--row-inline-gap: clamp(8px, 0.85vw, 12px);
	display: flex;
	flex-direction: column;
	gap: clamp(12px, 1.45vh, 20px);
	width: auto;
	height: 100%;
	overflow: hidden;
	scrollbar-width: thin;
	scrollbar-color: var(--rt-scroll-thumb) var(--rt-scroll-track);
}

.control_panel::-webkit-scrollbar {
	width: 8px;
	height: 8px;
}

.control_panel::-webkit-scrollbar-track {
	border-radius: 6px;
	background: var(--rt-scroll-track);
}

.control_panel::-webkit-scrollbar-thumb {
	border-radius: 6px;
	background: var(--rt-scroll-thumb);
}

.control_panel::-webkit-scrollbar-thumb:hover {
	background: var(--rt-scroll-thumb-hover);
}

/* 3.1 shared form primitives */
.threshold_row,
.selectors_row,
.path_row,
.meta_and_action_row,
.progress_row,
.terminal_output_row {
	display: flex;
	align-items: center;
	gap: var(--row-inline-gap);
	row-gap: var(--row-block-gap);
}

.selectors_row,
.path_row,
.meta_and_action_row,
.terminal_output_row {
	flex-wrap: wrap;
}

.field_pair {
	display: flex;
	flex: 0 0 auto;
	align-items: center;
	gap: var(--row-inline-gap);
}

.field_pair label {
	display: flex;
	flex: 0 0 auto;
	align-items: center;
	justify-content: center;
	width: var(--pair-label-width);
	height: 30px;
	min-width: 0;
	padding: 0 10px;
	box-sizing: border-box;
	border: 1px solid var(--label-border);
	border-radius: 9px;
	background: var(--label-bg);
	color: var(--label-text);
	font-size: clamp(12px, 0.9vw, 14px);
	font-weight: 600;
	text-align: center;
}

input,
select {
	height: 30px;
	padding: 0 10px;
	box-sizing: border-box;
	border: 1px solid var(--field-border);
	border-radius: 10px;
	background: var(--field-bg);
	color: var(--field-text);
	font-size: clamp(12px, 0.9vw, 14px);
	outline: none;
	transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

input::placeholder {
	color: var(--field-placeholder);
}

input:focus,
select:focus {
	border-color: var(--focus-border);
	box-shadow: 0 0 0 2px var(--focus-ring);
}

/* 3.2 threshold row */
.threshold_row {
	justify-content: flex-start;
	flex-wrap: wrap;
	column-gap: clamp(14px, 1.4vw, 20px);
}

.field_pair input {
	width: var(--pair-value-width);
}

.threshold_row .field_pair input {
	min-width: var(--pair-value-width);
}

/* 3.3 selectors row */
.selectors_row {
	justify-content: flex-start;
	column-gap: clamp(14px, 1.4vw, 20px);
	width: 100%;
}

.selectors_row .level_pair {
	flex: 1 1 0;
	min-width: 0;
}

.selector_pair select {
	height: 30px;
	padding-right: 24px;
	background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 8'%3E%3Cpath d='M1 1l5 5 5-5' fill='none' stroke='%23DAE6FB' stroke-width='1.7' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
	background-repeat: no-repeat;
	background-position: right 8px center;
	background-size: 12px 8px;
	appearance: none;
	-webkit-appearance: none;
	-moz-appearance: none;
}

.selector_pair:not(.level_pair) select {
	width: var(--pair-value-width);
	min-width: var(--pair-value-width);
	font-size: clamp(12px, 0.9vw, 14px);
}

.level_pair select {
	width: 100%;
	min-width: 0;
	max-width: none;
	padding-right: 24px;
}

.selector_pair select::-ms-expand {
	display: none;
}

select option {
	background-color: var(--select-option-bg);
	color: var(--select-option-text);
}

select option:checked {
	background-color: var(--select-option-bg);
	color: var(--select-option-text);
}

select option:hover {
	background-color: var(--select-option-selected-bg);
	color: var(--select-option-selected-text);
}

/* 3.4 path row */
.path_row {
	justify-content: flex-start;
}

.path_row input {
	flex: 1;
	min-width: 0;
}

.selected_path_input_wrap {
	position: relative;
	flex: 1;
	min-width: 0;
}

.selected_path_input_wrap input {
	width: 100%;
	padding-right: 58px;
}

.clear_hint {
	position: absolute;
	top: 50%;
	right: 12px;
	transform: translateY(-50%);
	opacity: 0;
	pointer-events: auto;
	color: var(--rt-open-file-hint-text);
	font-size: clamp(11px, 0.8vw, 13px);
	font-weight: 600;
	white-space: nowrap;
	cursor: pointer;
	transition: opacity 0.16s ease;
}

.selected_path_input_wrap:hover .clear_hint,
.selected_path_input_wrap:focus-within .clear_hint {
	opacity: 0.82;
}

/* 3.5 meta and actions */
.meta_and_action_row {
	justify-content: flex-start;
	align-items: center;
	gap: var(--row-inline-gap);
}

.meta_box {
	flex: 1;
	padding: 8px 10px;
	border: 1px dashed var(--rt-meta-border);
	border-radius: 10px;
	background: var(--rt-meta-bg);
	color: var(--rt-meta-text);
	text-align: left;
}

.meta_box p {
	margin: 0;
	color: inherit;
	font-size: clamp(12px, 0.92vw, 14px);
	line-height: 1.65;
	text-align: left;
}

.meta_box span {
	color: var(--rt-meta-key-text);
	font-weight: 700;
}

.pick_btn,
.start_btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: var(--action-btn-width);
	min-width: var(--action-btn-width);
	height: 32px;
	padding: 0 16px;
	border: 1px solid var(--btn-border);
	border-radius: 10px;
	background: linear-gradient(180deg, var(--btn-bg-start), var(--btn-bg-end));
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2);
	color: var(--btn-text);
	font-size: clamp(12px, 0.95vw, 15px);
	font-weight: 600;
	line-height: 1;
	white-space: nowrap;
	cursor: pointer;
	transition: all 0.16s ease;
}

.pick_btn:hover,
.start_btn:hover {
	filter: brightness(1.12);
}

.start_btn {
	height: 34px;
	border-color: var(--rt-start-btn-border);
	background: linear-gradient(180deg, var(--rt-start-btn-bg-start), var(--rt-start-btn-bg-end));
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.36), 0 0 0 1px rgba(255, 255, 255, 0.08), 0 6px 14px var(--rt-start-btn-shadow);
	color: var(--rt-start-btn-text);
	text-shadow: none;
}

.start_btn:hover {
	filter: brightness(1.06) saturate(1.04);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.42), 0 0 0 1px rgba(255, 255, 255, 0.08), 0 8px 18px var(--rt-start-btn-shadow);
}

.start_btn.is-running {
	border-color: var(--rt-stop-btn-border);
	background: linear-gradient(180deg, var(--rt-stop-btn-bg-start), var(--rt-stop-btn-bg-end));
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.3), 0 0 0 1px rgba(255, 255, 255, 0.08), 0 6px 14px var(--rt-stop-btn-shadow);
	color: var(--rt-stop-btn-text);
}

.start_btn.is-running:hover {
	filter: brightness(1.05) saturate(1.03);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.36), 0 0 0 1px rgba(255, 255, 255, 0.08), 0 8px 18px var(--rt-stop-btn-shadow);
}

/* 3.6 progress row */
.progress_row {
	display: block;
	align-items: center;
	color: var(--rt-meta-key-text);
	font-size: clamp(14px, 1.02vw, 17px);
	font-weight: 700;
	row-gap: 0;
	column-gap: clamp(6px, 0.7vw, 10px);
	grid-template-columns: max-content minmax(0, 1fr);
}

.progress_row .progress_track {
	width: 100%;
	min-width: 0;
}

.progress_track {
	position: relative;
	display: flex;
	align-items: center;
	height: 30px;
	overflow: hidden;
	border: 1px solid var(--rt-progress-track-border);
	border-radius: 10px;
	background: var(--rt-progress-track-bg);
}

.progress_fill {
	position: relative;
	width: var(--rt-progress-width, 0%);
	height: 100%;
	overflow: hidden;
	background: linear-gradient(90deg, var(--rt-progress-fill-start), var(--rt-progress-fill-end));
	background-size: 220% 100%;
	animation: progressGradientShift 1.5s linear infinite;
	transition: width 0.2s ease;
}

.progress_fill::after {
	position: absolute;
	inset: 0;
	content: '';
	transform: translateX(-100%);
	background: linear-gradient(100deg, rgba(255, 255, 255, 0) 15%, rgba(255, 255, 255, 0.28) 50%, rgba(255, 255, 255, 0) 85%);
	animation: progressGlowSweep 1.8s ease-in-out infinite;
}

.progress_track span {
	position: absolute;
	left: 50%;
	transform: translateX(-50%);
	color: var(--rt-progress-text);
	font-size: clamp(12px, 0.95vw, 14px);
	font-weight: 700;
	text-shadow: var(--rt-progress-text-shadow);
}

/* 3.7 terminal output row */
.terminal_output_row {
	display: block;
	flex: 1 1 auto;
	align-items: stretch;
	min-height: 120px;
}

.terminal_output_wrap {
	position: relative;
	width: 100%;
	height: 100%;
	min-height: 120px;
}

.terminal_output_box {
	width: 100%;
	height: 100%;
	min-height: 120px;
	margin: 0;
	padding: 8px 10px;
	box-sizing: border-box;
	overflow-x: auto;
	overflow-y: auto;
	resize: none;
	border: 1px solid var(--rt-output-border);
	border-radius: 10px;
	background: var(--rt-output-bg);
	color: var(--rt-output-text);
	font-family: Consolas, "Courier New", monospace;
	font-size: 12px;
	line-height: 1.45;
	text-align: left;
	white-space: pre;
	overflow-wrap: normal;
	word-break: normal;
	cursor: text;
	user-select: text;
	-ms-user-select: text;
	-moz-user-select: text;
	-webkit-user-select: text;
	scrollbar-width: thin;
	scrollbar-color: var(--rt-scroll-thumb) var(--rt-scroll-track-transparent);
}

.terminal_output_box::-webkit-scrollbar {
	width: 8px;
	height: 8px;
}

.terminal_output_box::-webkit-scrollbar-track {
	border-radius: 10px;
	background: var(--rt-scroll-track-transparent);
}

.terminal_output_box::-webkit-scrollbar-thumb {
	border: 1px solid transparent;
	border-radius: 10px;
	background: var(--rt-scroll-thumb);
	background-clip: padding-box;
}

.clear_output_hint {
	position: absolute;
	top: 10px;
	right: 12px;
	opacity: 0;
	pointer-events: auto;
	color: var(--rt-open-file-hint-text);
	font-size: clamp(11px, 0.8vw, 13px);
	font-weight: 600;
	white-space: nowrap;
	cursor: pointer;
	user-select: none;
	transition: opacity 0.16s ease;
}

.terminal_output_wrap:hover .clear_output_hint,
.terminal_output_wrap:focus-within .clear_output_hint {
	opacity: 0.82;
}

/* 4) Animations */
@keyframes progressGradientShift {
	0% {
		background-position: 0% 50%;
	}
	100% {
		background-position: 100% 50%;
	}
}

@keyframes progressGlowSweep {
	0% {
		transform: translateX(-100%);
	}
	100% {
		transform: translateX(130%);
	}
}

/* 5) Responsive */
@media (max-width: 820px) {
	.randomtest_main {
		grid-template-columns: minmax(0, 5fr) minmax(0, 5fr);
		min-width: 0;
		padding: 8px;
	}

	.path_row input {
		min-width: 130px;
	}

	.result_table {
		min-width: 520px;
	}

	.result_table th,
	.result_table td {
		height: 22px;
	}

	.start_btn,
	.pick_btn {
		height: 32px;
		padding: 0 12px;
	}

	.terminal_output_row,
	.terminal_output_box {
		min-height: 96px;
	}
}
</style>
