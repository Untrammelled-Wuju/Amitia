const bridge = window.amitiaUI;

const state = {
  context: {},
  snapshot: null,
  view: "overview",
  busy: false,
};

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function value(form, name) {
  const element = form.elements.namedItem(name);
  if (!element) return "";
  if (element.type === "checkbox") return element.checked;
  if (element.type === "number" || element.type === "range") return Number(element.value);
  return element.value;
}

function bool(value) {
  return value === true || value === "true" || value === "1" || value === 1;
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function setMessage(message, error = false) {
  const element = document.getElementById("message");
  if (!element) return;
  element.textContent = message || "";
  element.className = error ? "message error" : "message";
}

async function invoke(action, payload = {}) {
  if (!bridge) throw new Error("宿主桥接不可用");
  state.busy = true;
  setMessage("正在处理...");
  try {
    const response = await bridge.invokeAction("command", {
      action,
      payload: {
        ...payload,
        characterId: state.context.characterId || "",
      },
    });
    if (!response || response.ok !== true) {
      throw new Error(response && response.error ? response.error : "操作失败");
    }
    if (response.result && response.result.stateLife) state.snapshot = response.result;
    setMessage("已更新");
    render();
    return response.result;
  } catch (error) {
    setMessage(error instanceof Error ? error.message : String(error), true);
    throw error;
  } finally {
    state.busy = false;
  }
}

function stat(label, value) {
  return `<div class="panel stat"><div class="stat-label">${escapeHTML(label)}</div><div class="stat-value">${escapeHTML(value)}</div></div>`;
}

function renderOverview() {
  const snapshot = state.snapshot;
  if (!snapshot) return `<div class="empty">正在读取生活状态...</div>`;
  const life = snapshot.stateLife || {};
  const schedule = snapshot.schedule || {};
  const conflicts = Array.isArray(snapshot.conflicts) ? snapshot.conflicts : [];
  return `
    <div class="grid">
      ${stat("当前状态", life.currentActivity || life.currentState || "-")}
      ${stat("精力", life.energy ?? 0)}
      ${stat("可回复", life.available ? "是" : "否")}
      ${stat("休息中", life.sleeping ? "是" : "否")}
    </div>
    <div class="section-grid section-gap">
      <section class="panel">
        <h2>今日作息</h2>
        <div class="list">
          <div class="list-item"><div><div class="list-title">起床</div><div class="list-meta">${escapeHTML(formatTime(schedule.wakeTime))}</div></div></div>
          <div class="list-item"><div><div class="list-title">午饭</div><div class="list-meta">${escapeHTML(formatTime(schedule.lunchTime))}</div></div></div>
          <div class="list-item"><div><div class="list-title">晚饭</div><div class="list-meta">${escapeHTML(formatTime(schedule.dinnerTime))}</div></div></div>
          <div class="list-item"><div><div class="list-title">睡眠</div><div class="list-meta">${escapeHTML(formatTime(schedule.sleepTime))}</div></div></div>
        </div>
      </section>
      <section class="panel">
        <h2>日程冲突</h2>
        ${conflicts.length ? `<ul class="error-list">${conflicts.map((item) => `<li>${escapeHTML(item.message)}</li>`).join("")}</ul>` : `<div class="empty">当前没有日程冲突</div>`}
      </section>
    </div>
    <section class="panel">
      <h2>今日时间线</h2>
      ${renderTimeline((snapshot.timeline && snapshot.timeline.events) || [])}
    </section>
  `;
}

function renderTimeline(events) {
  if (!events.length) return `<div class="empty">今天还没有时间线安排</div>`;
  return `<div class="timeline">${events.map((item) => `
    <div class="timeline-item">
      <div class="timeline-time">${escapeHTML(formatTime(item.startTime))} - ${escapeHTML(formatTime(item.endTime))}</div>
      <div><div class="list-title">${escapeHTML(item.reason || item.state)}</div><div class="list-meta">${escapeHTML(item.state)} · ${escapeHTML(item.sourceType)}</div></div>
    </div>
  `).join("")}</div>`;
}

function renderSchedule() {
  const timeline = state.snapshot && state.snapshot.timeline ? state.snapshot.timeline : { events: [] };
  return `<section class="panel"><h2>日程与时间线</h2>${renderTimeline(timeline.events || [])}</section>`;
}

function renderRoutine() {
  const profile = state.snapshot || {};
  const sleep = profile.sleepSetting || {};
  const work = profile.workProfile || {};
  return `
    <div class="section-grid">
      <form class="panel" data-form="sleep.update">
        <h2>睡眠设置</h2>
        <div class="form-grid">
          <label>入睡时间<input type="time" name="bedTime" value="${escapeHTML(sleep.bedTime || "23:00")}" /></label>
          <label>起床时间<input type="time" name="wakeTime" value="${escapeHTML(sleep.wakeTime || "07:00")}" /></label>
          <label>启用睡眠<select name="enabled"><option value="true" ${sleep.enabled ? "selected" : ""}>启用</option><option value="false" ${!sleep.enabled ? "selected" : ""}>停用</option></select></label>
          <label>睡眠回复模式<select name="sleepReplyMode"><option value="NO_REPLY" ${sleep.sleepReplyMode === "NO_REPLY" ? "selected" : ""}>不回复</option><option value="SHORT_REPLY" ${sleep.sleepReplyMode === "SHORT_REPLY" ? "selected" : ""}>简短回复</option><option value="NORMAL_REPLY" ${sleep.sleepReplyMode === "NORMAL_REPLY" ? "selected" : ""}>正常回复</option></select></label>
        </div>
        <div class="actions"><button class="primary" type="submit">保存睡眠设置</button></div>
      </form>
      <form class="panel" data-form="work.update">
        <h2>工作与通勤</h2>
        <div class="form-grid">
          <label>启用工作档期<select name="enabled"><option value="true" ${work.enabled ? "selected" : ""}>启用</option><option value="false" ${!work.enabled ? "selected" : ""}>停用</option></select></label>
          <label>工作日起始<input name="workStartTime" type="time" value="${escapeHTML(work.workStartTime || "09:00")}" /></label>
          <label>工作日结束<input name="workEndTime" type="time" value="${escapeHTML(work.workEndTime || "18:00")}" /></label>
          <label>午休开始<input name="lunchBreakStartTime" type="time" value="${escapeHTML(work.lunchBreakStartTime || "12:00")}" /></label>
          <label>午休结束<input name="lunchBreakEndTime" type="time" value="${escapeHTML(work.lunchBreakEndTime || "13:30")}" /></label>
          <label>通勤最短分钟<input name="commuteMinMinutes" type="number" min="0" value="${Number(work.commuteMinMinutes ?? 15)}" /></label>
          <label>通勤最长分钟<input name="commuteMaxMinutes" type="number" min="0" value="${Number(work.commuteMaxMinutes ?? 45)}" /></label>
          <label>准备最短分钟<input name="prepareMinMinutes" type="number" min="0" value="${Number(work.prepareMinMinutes ?? 20)}" /></label>
          <label>准备最长分钟<input name="prepareMaxMinutes" type="number" min="0" value="${Number(work.prepareMaxMinutes ?? 60)}" /></label>
        </div>
        <div class="actions"><button class="primary" type="submit">保存工作档期</button></div>
      </form>
    </div>
  `;
}

function renderEvents() {
  const data = state.snapshot || {};
  const fixed = Array.isArray(data.fixedEvents) ? data.fixedEvents : [];
  const special = Array.isArray(data.specialEvents) ? data.specialEvents : [];
  const classes = Array.isArray(data.classAdjustments) ? data.classAdjustments : [];
  return `
    <section class="panel">
      <h2>固定事件</h2>
      <div class="list">${fixed.length ? fixed.map((item) => `
        <div class="list-item">
          <div><div class="list-title">${escapeHTML(item.title)}</div><div class="list-meta">${escapeHTML(item.startTime)} - ${escapeHTML(item.endTime)} · ${escapeHTML(item.eventType)}</div></div>
          <div class="actions"><button type="button" data-command="fixed.toggle" data-id="${item.id}">${item.enabled ? "停用" : "启用"}</button><button class="danger" type="button" data-command="fixed.delete" data-id="${item.id}">删除</button></div>
        </div>
      `).join("") : `<div class="empty">暂无固定事件</div>`}</div>
      <form data-form="fixed.create">
        <div class="form-grid section-gap">
          <label>标题<input name="title" required placeholder="例如：晨会" /></label>
          <label>类型<input name="eventType" value="CUSTOM_BUSY" /></label>
          <label>开始<input name="startTime" type="time" required /></label>
          <label>结束<input name="endTime" type="time" required /></label>
        </div>
        <div class="actions"><button class="primary" type="submit">新增固定事件</button></div>
      </form>
    </section>
    <section class="panel">
      <h2>特殊事件</h2>
      <div class="list">${special.length ? special.map((item) => `
        <div class="list-item">
          <div><div class="list-title">${escapeHTML(item.title)}</div><div class="list-meta">${escapeHTML(item.startDate || "")} · ${escapeHTML(item.eventType)}</div></div>
          <div class="actions"><button type="button" data-command="special.toggle" data-id="${item.id}">${item.enabled ? "停用" : "启用"}</button><button class="danger" type="button" data-command="special.delete" data-id="${item.id}">删除</button></div>
        </div>
      `).join("") : `<div class="empty">暂无特殊事件</div>`}</div>
      <form data-form="special.create">
        <div class="form-grid section-gap">
          <label>标题<input name="title" required placeholder="例如：考试周" /></label>
          <label>类型<input name="eventType" value="CUSTOM" /></label>
          <label>开始日期<input name="startDate" type="date" required /></label>
          <label>结束日期<input name="endDate" type="date" /></label>
          <label>开始时间<input name="startTime" type="time" /></label>
          <label>结束时间<input name="endTime" type="time" /></label>
        </div>
        <div class="actions"><button class="primary" type="submit">新增特殊事件</button></div>
      </form>
    </section>
    <section class="panel">
      <h2>调课与停课</h2>
      <div class="list">${classes.length ? classes.map((item) => `
        <div class="list-item">
          <div><div class="list-title">${escapeHTML(item.className)}</div><div class="list-meta">${escapeHTML(item.date)} · 第 ${Number(item.slotIndex) + 1} 节 · ${escapeHTML(item.adjustType)}</div></div>
          <div class="actions"><button class="danger" type="button" data-command="class.delete" data-id="${item.id}">删除</button></div>
        </div>
      `).join("") : `<div class="empty">暂无调课记录</div>`}</div>
      <form data-form="class.create">
        <div class="form-grid section-gap">
          <label>日期<input name="date" type="date" required /></label>
          <label>节次<input name="slotIndex" type="number" min="0" value="0" /></label>
          <label>课程名称<input name="className" required /></label>
          <label>调整类型<select name="adjustType"><option value="swap">调课</option><option value="reschedule">改期</option><option value="makeup">补课</option><option value="canceled">停课</option></select></label>
        </div>
        <div class="actions"><button class="primary" type="submit">新增调课记录</button></div>
      </form>
    </section>
  `;
}

function renderTendency() {
  const tendencies = (state.snapshot && state.snapshot.lifestyleTendency) || {};
  const fields = [
    ["punctualityTendency", "守时倾向"],
    ["earlyPrepareTendency", "提前准备"],
    ["selfDisciplineTendency", "自律程度"],
    ["sleepinessTendency", "困倦倾向"],
    ["randomnessTendency", "随机程度"],
    ["activityEnergy", "活动精力"],
    ["socialEnergy", "社交精力"],
    ["careTendency", "关心倾向"],
    ["dailyShareTendency", "日常分享"],
  ];
  return `
    <form class="panel" data-form="tendency.update">
      <h2>生活倾向</h2>
      <div class="list">${fields.map(([key, label]) => `
        <label>${label}
          <div class="range-row">
            <input type="range" name="${key}" min="0" max="100" value="${Number(tendencies[key] ?? 50)}" />
            <input type="number" min="0" max="100" value="${Number(tendencies[key] ?? 50)}" data-sync-range="${key}" />
          </div>
        </label>
      `).join("")}</div>
      <div class="actions"><button class="primary" type="submit">保存生活倾向</button><button type="button" data-command="tendency.reset">恢复默认</button></div>
    </form>
  `;
}

function renderData() {
  return `
    <section class="panel">
      <h2>插件数据</h2>
      <p>生活系统数据由插件独立持有，不依赖宿主业务表。</p>
      <div class="actions">
        <button class="primary" type="button" data-command="schedule.regenerate">重新生成日程</button>
      </div>
    </section>
    <section class="panel">
      <h2>权限与状态</h2>
      <div class="list">
        <div class="list-item"><div><div class="list-title">状态存储</div><div class="list-meta">扩展状态命名空间 v1</div></div><span class="tag">插件自有</span></div>
        <div class="list-item"><div><div class="list-title">数据来源</div><div class="list-meta">插件状态命名空间</div></div><span class="tag">独立</span></div>
      </div>
    </section>
  `;
}

function render() {
  const view = document.getElementById("view");
  const subtitle = document.getElementById("subtitle");
  if (!view || !subtitle) return;
  subtitle.textContent = state.context.characterName
    ? `${state.context.characterName} 的生活规则、日程与状态`
    : "角色生活规则、日程与状态";
  if (state.view === "schedule") view.innerHTML = renderSchedule();
  else if (state.view === "routine") view.innerHTML = renderRoutine();
  else if (state.view === "events") view.innerHTML = renderEvents();
  else if (state.view === "tendency") view.innerHTML = renderTendency();
  else if (state.view === "data") view.innerHTML = renderData();
  else view.innerHTML = renderOverview();
}

function formPayload(form) {
  const payload = {};
  for (const element of form.elements) {
    if (!element.name) continue;
    if (element.type === "checkbox") payload[element.name] = element.checked;
    else if (element.type === "number" || element.type === "range") payload[element.name] = Number(element.value);
    else if (["enabled", "allowOvertime", "delayedReplyEnabled", "commuteHomeShareEnabled"].includes(element.name)) payload[element.name] = bool(element.value);
    else payload[element.name] = element.value;
  }
  return payload;
}

document.addEventListener("click", async (event) => {
  const viewButton = event.target.closest("[data-view]");
  if (viewButton) {
    state.view = viewButton.dataset.view;
    document.querySelectorAll("[data-view]").forEach((button) => button.classList.toggle("active", button === viewButton));
    render();
    return;
  }
  const commandButton = event.target.closest("[data-command]");
  if (!commandButton || state.busy) return;
  const command = commandButton.dataset.command;
  const payload = { id: Number(commandButton.dataset.id || 0) };
  if (command === "tendency.reset") payload.tendencies = null;
  await invoke(command, payload).catch(() => {});
});

document.addEventListener("submit", async (event) => {
  const form = event.target.closest("[data-form]");
  if (!form) return;
  event.preventDefault();
  if (state.busy) return;
  const action = form.dataset.form;
  const payload = formPayload(form);
  if (action.startsWith("sleep.")) payload.sleep = payload;
  if (action.startsWith("work.")) payload.work = payload;
  if (action.startsWith("tendency.")) payload.tendencies = payload;
  await invoke(action, payload).catch(() => {});
  form.reset();
});

document.addEventListener("input", (event) => {
  const input = event.target;
  if (!(input instanceof HTMLInputElement)) return;
  const key = input.dataset.syncRange;
  if (!key) return;
  const range = document.querySelector(`input[type="range"][name="${key}"]`);
  if (range) range.value = input.value;
});

document.getElementById("refresh-button")?.addEventListener("click", async () => {
  await invoke("snapshot").catch(() => {});
});

async function boot() {
  if (!bridge) {
    setMessage("宿主桥接不可用", true);
    return;
  }
  await bridge.ready();
  state.context = await bridge.getContext();
  await invoke("snapshot").catch(() => {});
}

void boot();
