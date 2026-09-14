const state = {
  groups: [],
  emotes: [],
  characters: [],
  total: 0,
  loading: false,
  saving: false,
  importing: false,
  activeView: "all",
  activeGroup: "",
  search: "",
  selectedIds: new Set(),
  focusedId: "",
  hoveredId: "",
  bulkGroup: "",
  groupsLoading: true,
  groupMenuId: "",
  imports: [],
};

const $ = (id) => document.getElementById(id);
const icons = {
  more: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 10a2 2 0 1 0 0 4 2 2 0 0 0 0-4Zm6 0a2 2 0 1 0 0 4 2 2 0 0 0 0-4Zm6 0a2 2 0 1 0 0 4 2 2 0 0 0 0-4Z"/></svg>',
  delete: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 8h10l-.7 11H7.7L7 8Zm2-3h6l1 2H8l1-2ZM5 7h14v2H5V7Zm4 4v6h2v-6H9Zm4 0v6h2v-6h-2Z"/></svg>',
  picture: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Zm0 2v14h14V5H5Zm2.5 10 3-3 2.5 2.5 2-2 3.5 3.5h-11Zm2-6.5A1.75 1.75 0 1 1 9.5 12a1.75 1.75 0 0 1 0-3.5Z"/></svg>',
};

function toast(message) {
  const box = document.createElement("div");
  box.textContent = message;
  $("toast").appendChild(box);
  setTimeout(() => box.remove(), 2600);
}

async function call(action, payload = {}) {
  const result = await window.amitiaUI.invokeAction("command", { action, payload });
  return result && Object.prototype.hasOwnProperty.call(result, "result") ? result.result : result;
}

function escapeHTML(value) {
  return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
}

function parseKeywords(value) {
  return String(value || "").split(/[，,；;\n]/).map((item) => item.trim()).filter(Boolean);
}

function formatBytes(value) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function focusedEmote() {
  return state.emotes.find((item) => item.id === state.focusedId) || null;
}

function coverUrl(group) {
  return (state.emotes.find((item) => item.id === group.coverEmoteId) || {}).thumbnailUrl || "";
}

async function loadGroups() {
  try {
    state.groups = await call("groups.list");
    if (!Array.isArray(state.groups)) state.groups = [];
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  } finally {
    state.groupsLoading = false;
    renderGroups();
    renderGroupOptions();
  }
}

let charactersPromise = null;
function loadCharacters() {
  if (state.characters.length) return Promise.resolve(state.characters);
  if (charactersPromise) return charactersPromise;
  charactersPromise = call("characters.list", { includeDisabled: true })
    .then((result) => {
      state.characters = (result.items || []).map((item) => item.data || item);
      renderDetail();
      renderImports();
      return state.characters;
    })
    .catch(() => [])
    .finally(() => {
      charactersPromise = null;
    });
  return charactersPromise;
}

async function loadEmotes() {
  state.loading = true;
  renderGrid();
  try {
    const data = await call("emotes.list", {
      groupId: state.activeGroup || undefined,
      view: state.activeGroup ? undefined : state.activeView,
      q: state.search,
      pageSize: 200,
    });
    state.emotes = data.items || [];
    state.total = data.total || 0;
    state.selectedIds = new Set([...state.selectedIds].filter((id) => state.emotes.some((item) => item.id === id)));
    if (state.focusedId && !state.emotes.some((item) => item.id === state.focusedId)) state.focusedId = "";
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  } finally {
    state.loading = false;
    renderGroups();
    renderGrid();
    renderDetail();
  }
}

async function loadAll() {
  renderGroups();
  renderGrid();
  renderDetail();
  renderGroupOptions();
  await Promise.all([loadGroups(), loadEmotes()]);
}

function renderGroups() {
  document.querySelectorAll("[data-view]").forEach((button) => {
    button.classList.toggle("active", button.dataset.view === state.activeView && !state.activeGroup);
  });
  const list = $("group-list");
  if (!state.groups.length) {
    list.innerHTML = `<div class="muted">${state.groupsLoading ? "正在加载分组..." : "还没有分组"}</div>`;
    return;
  }
  list.innerHTML = state.groups.map((group, index) => `
    <div class="group ${state.activeGroup === group.id ? "active" : ""}">
      <button type="button" class="group-main" data-group="${escapeHTML(group.id)}">
        <span class="cover">${coverUrl(group) ? `<img src="${escapeHTML(coverUrl(group))}" alt="">` : ""}</span>
        <span class="group-name">${escapeHTML(group.name)}</span>
      </button>
      <button type="button" class="el-button text circle" data-group-menu="${escapeHTML(group.id)}" aria-label="${escapeHTML(group.name)} 操作">${icons.more}</button>
      ${state.groupMenuId === group.id ? `<div class="group-menu">
        <button type="button" data-group-action="rename" data-id="${escapeHTML(group.id)}">重命名</button>
        <button type="button" data-group-action="up" data-id="${escapeHTML(group.id)}" ${index === 0 ? "disabled" : ""}>上移</button>
        <button type="button" data-group-action="down" data-id="${escapeHTML(group.id)}" ${index === state.groups.length - 1 ? "disabled" : ""}>下移</button>
        <button type="button" data-group-action="cover" data-id="${escapeHTML(group.id)}" ${state.selectedIds.size !== 1 ? "disabled" : ""}>设置封面</button>
        <button type="button" data-group-action="delete" data-id="${escapeHTML(group.id)}">删除分组</button>
      </div>` : ""}
    </div>
  `).join("");
}

function renderGrid() {
  $("total").textContent = `${state.total} 个表情`;
  $("bulk").hidden = state.selectedIds.size === 0;
  $("selected-count").textContent = `已选 ${state.selectedIds.size} 项`;
  $("bulk-remove-group").hidden = !state.activeGroup;
  $("select-all").checked = !!state.emotes.length && state.emotes.every((item) => state.selectedIds.has(item.id));
  $("select-all").indeterminate = state.selectedIds.size > 0 && !$("select-all").checked;

  if (!state.emotes.length) {
    $("grid").innerHTML = `<div class="empty">${state.loading ? "" : icons.picture}<strong>${state.loading ? "正在加载表情..." : "暂无表情"}</strong>${state.loading ? "" : "<span>拖入图片，或点击上方按钮开始导入。</span>"}</div>`;
    return;
  }

  $("grid").innerHTML = state.emotes.map((item) => `
    <button type="button" class="card ${state.selectedIds.has(item.id) ? "selected" : ""} ${state.focusedId === item.id ? "focused" : ""}" data-item="${escapeHTML(item.id)}">
      <label class="check"><input type="checkbox" ${state.selectedIds.has(item.id) ? "checked" : ""} aria-label="选择 ${escapeHTML(item.name)}"></label>
      <img src="${escapeHTML(state.hoveredId === item.id ? item.assetUrl : item.thumbnailUrl || item.assetUrl || "")}" alt="${escapeHTML(item.meaning || item.name)}" loading="lazy">
      <strong>${escapeHTML(item.name)}</strong>
      <div class="card-tags"><small>${item.isAnimated ? "动图" : item.fileExtension.toUpperCase()}</small><small class="${item.aiEnabled ? "enabled" : ""}">AI</small></div>
    </button>
  `).join("");

  $("grid").querySelectorAll("[data-item]").forEach((card) => {
    const id = card.dataset.item;
    card.addEventListener("mouseenter", () => {
      state.hoveredId = id;
      const item = state.emotes.find((value) => value.id === id);
      const image = card.querySelector("img");
      if (item && image && item.assetUrl) image.src = item.assetUrl;
    });
    card.addEventListener("mouseleave", () => {
      state.hoveredId = "";
      const item = state.emotes.find((value) => value.id === id);
      const image = card.querySelector("img");
      if (item && image && item.thumbnailUrl) image.src = item.thumbnailUrl;
    });
    card.addEventListener("click", (event) => {
      if (event.target.closest(".check")) {
        toggleSelection(id);
        return;
      }
      state.focusedId = id;
      renderGrid();
      renderDetail();
    });
    card.addEventListener("dblclick", () => toggleSelection(id));
  });
}

function renderDetail() {
  const item = focusedEmote();
  if (!item) {
    $("detail").innerHTML = `<div class="empty-detail">选择一个表情查看详情</div>`;
    return;
  }
  if (!state.characters.length) void loadCharacters();
  const characterOptions = state.characters.length
    ? state.characters.map((character) => `<option value="${escapeHTML(character.id)}" ${(item.characterIds || []).includes(character.id) ? "selected" : ""}>${escapeHTML(character.displayName || character.name || character.id)}</option>`).join("")
    : `<option disabled>正在加载角色...</option>`;
  $("detail").innerHTML = `
    <img class="preview" src="${escapeHTML(item.assetUrl || item.thumbnailUrl || "")}" alt="${escapeHTML(item.meaning || item.name)}">
    <div class="form-item"><label>名称</label><input class="el-input-native" id="detail-name" value="${escapeHTML(item.name)}"></div>
    <div class="form-item"><label>含义</label><textarea class="el-input-native" id="detail-meaning" rows="3">${escapeHTML(item.meaning || "")}</textarea><small>AI 可用时含义不能为空。</small></div>
    <div class="form-item"><label>关键词</label><input class="el-input-native" id="detail-keywords" value="${escapeHTML((item.keywords || []).join("，"))}" placeholder="用逗号分隔"></div>
    <div class="form-item"><label class="el-switch"><input type="checkbox" id="detail-ai" ${item.aiEnabled ? "checked" : ""}><span class="switch-track"></span><span>允许 AI 使用</span></label></div>
    <fieldset>
      <legend>适用角色</legend>
      <div class="radio-group">
        <label class="radio-label"><input type="radio" name="role-scope" value="all_characters" ${item.roleScope === "all_characters" ? "checked" : ""}>全部角色</label>
        <label class="radio-label"><input type="radio" name="role-scope" value="selected_characters" ${item.roleScope === "selected_characters" ? "checked" : ""}>指定角色</label>
      </div>
      <select class="el-select" id="detail-characters" multiple ${item.roleScope === "selected_characters" ? "" : "hidden"}>${characterOptions}</select>
    </fieldset>
    <fieldset>
      <legend>所在分组</legend>
      <select class="el-select" id="detail-groups" multiple>
        ${state.groups.map((group) => `<option value="${escapeHTML(group.id)}" ${(item.groupIds || []).includes(group.id) ? "selected" : ""}>${escapeHTML(group.name)}</option>`).join("")}
      </select>
    </fieldset>
    <div class="metadata">
      <span>${escapeHTML(item.fileExtension.toUpperCase())}</span><span>${formatBytes(item.fileSize)}</span>
      <span>${item.width} × ${item.height}</span><span>${item.isAnimated ? `${item.frameCount} 帧` : "静态"}</span>
      <span>平台降级：${item.fallbackPath ? "可用" : "不可用"}</span>
    </div>
    <div class="detail-actions"><button type="button" class="el-button primary" id="save-detail" ${state.saving ? "disabled" : ""}>保存</button><button type="button" class="el-button danger plain" id="delete-detail">删除</button></div>
  `;
  document.querySelectorAll("input[name='role-scope']").forEach((radio) => {
    radio.onchange = () => {
      const selected = document.querySelector("input[name='role-scope']:checked");
      $("detail-characters").hidden = selected?.value !== "selected_characters";
    };
  });
  $("save-detail").onclick = saveDetail;
  $("delete-detail").onclick = deleteFocused;
}

function renderGroupOptions() {
  const options = state.groups.map((group) => `<option value="${escapeHTML(group.id)}">${escapeHTML(group.name)}</option>`).join("");
  $("bulk-group").innerHTML = `<option value="">加入分组</option>${options}`;
  $("default-group").innerHTML = options;
}

function toggleSelection(id) {
  if (state.selectedIds.has(id)) state.selectedIds.delete(id);
  else state.selectedIds.add(id);
  renderGrid();
  renderGroups();
}

function selectView(view) {
  state.activeView = view;
  state.activeGroup = "";
  state.selectedIds.clear();
  state.groupMenuId = "";
  renderGroups();
  void loadEmotes();
}

function selectGroup(id) {
  state.activeGroup = id;
  state.selectedIds.clear();
  state.groupMenuId = "";
  renderGroups();
  void loadEmotes();
}

async function createGroup() {
  const name = prompt("分组只用于整理和浏览。\n\n新建分组");
  if (!name?.trim()) return;
  await call("groups.create", { name: name.trim() });
  await loadGroups();
}

async function groupAction(action, id) {
  const group = state.groups.find((item) => item.id === id);
  if (!group) return;
  state.groupMenuId = "";
  if (action === "rename") {
    const name = prompt("输入新的分组名称", group.name);
    if (!name?.trim()) return;
    await call("groups.update", { id, name: name.trim() });
    await loadGroups();
    return;
  }
  if (action === "up" || action === "down") {
    const index = state.groups.findIndex((item) => item.id === id);
    const delta = action === "up" ? -1 : 1;
    const nextIndex = index + delta;
    if (nextIndex < 0 || nextIndex >= state.groups.length) return;
    const copy = [...state.groups];
    const [moved] = copy.splice(index, 1);
    copy.splice(nextIndex, 0, moved);
    state.groups = copy;
    renderGroups();
    await call("groups.reorder", { ids: copy.map((item) => item.id) });
    return;
  }
  if (action === "cover") {
    const emoteId = [...state.selectedIds][0];
    if (!emoteId) return;
    await call("groups.update", { id, coverEmoteId: emoteId });
    await loadGroups();
    return;
  }
  if (action === "delete") {
    if (!confirm(`删除分组“${group.name}”？表情会保留。`)) return;
    await call("groups.delete", { id });
    if (state.activeGroup === id) state.activeGroup = "";
    await loadGroups();
    await loadEmotes();
  }
}

async function saveDetail() {
  const item = focusedEmote();
  if (!item || state.saving) return;
  const roleScope = document.querySelector("input[name='role-scope']:checked")?.value || "all_characters";
  const payload = {
    id: item.id,
    name: $("detail-name").value,
    meaning: $("detail-meaning").value,
    keywords: parseKeywords($("detail-keywords").value),
    aiEnabled: $("detail-ai").checked,
    roleScope,
    characterIds: roleScope === "selected_characters" ? [...$("detail-characters").selectedOptions].map((option) => option.value) : [],
    groupIds: [...$("detail-groups").selectedOptions].map((option) => option.value),
  };
  state.saving = true;
  renderDetail();
  try {
    const updated = await call("emotes.update", payload);
    const index = state.emotes.findIndex((value) => value.id === item.id);
    if (index >= 0) state.emotes[index] = updated;
    toast("已保存");
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  } finally {
    state.saving = false;
    renderGrid();
    renderDetail();
  }
}

async function deleteFocused() {
  const item = focusedEmote();
  if (!item || !confirm(`删除后“${item.name}”将停止 AI 使用并清理文件。\n\n确认删除？`)) return;
  await call("emotes.delete", { ids: [item.id] });
  state.focusedId = "";
  await loadEmotes();
}

async function bulkUpdate(update) {
  if (!state.selectedIds.size) return;
  await call("emotes.batch_update", { ids: [...state.selectedIds], update });
  await loadEmotes();
}

async function addSelectedToGroup() {
  const groupId = $("bulk-group").value;
  if (!groupId || !state.selectedIds.size) return;
  await call("groups.add", { groupId, emoteIds: [...state.selectedIds] });
  $("bulk-group").value = "";
  toast("已加入分组");
  await loadEmotes();
}

async function removeSelectedFromGroup() {
  for (const emoteId of state.selectedIds) await call("groups.remove", { groupId: state.activeGroup, emoteId });
  await loadEmotes();
}

async function deleteSelected() {
  if (!state.selectedIds.size || !confirm(`确认删除 ${state.selectedIds.size} 个表情？`)) return;
  await call("emotes.delete", { ids: [...state.selectedIds] });
  state.selectedIds.clear();
  await loadEmotes();
}

function openFiles(files) {
  void loadCharacters();
  const allowed = /\.(png|jpe?g|gif|webp)$/i;
  state.imports = files.filter((file) => allowed.test(file.name)).map((file) => {
    const relative = file.webkitRelativePath || file.name;
    const parts = relative.split("/");
    return {
      key: crypto.randomUUID(),
      file,
      preview: URL.createObjectURL(file),
      name: file.name.replace(/\.[^.]+$/, ""),
      meaning: "",
      keywords: "",
      groupIds: [],
      aiEnabled: false,
      roleScope: "all_characters",
      characterIds: [],
      folderGroup: parts.length > 1 ? parts[0] : "",
      relativePath: relative,
      status: "待导入",
    };
  });
  if (!state.imports.length) {
    toast("没有可导入的图片");
    return;
  }
  renderImports();
  $("import-dialog").showModal();
}

function characterOptions(selectedIds = []) {
  if (!state.characters.length) return `<option disabled>正在加载角色...</option>`;
  return state.characters.map((character) => `<option value="${escapeHTML(character.id)}" ${selectedIds.includes(character.id) ? "selected" : ""}>${escapeHTML(character.displayName || character.name || character.id)}</option>`).join("");
}

function groupOptions(selectedIds = []) {
  return state.groups.map((group) => `<option value="${escapeHTML(group.id)}" ${selectedIds.includes(group.id) ? "selected" : ""}>${escapeHTML(group.name)}</option>`).join("");
}

function renderImports() {
  $("import-count").textContent = `${state.imports.length} 个文件`;
  $("import-footer-count").textContent = `${state.imports.length} 个待导入文件`;
  $("import-list").innerHTML = state.imports.map((item, index) => `
    <div class="import-item" data-import-index="${index}">
      <img src="${escapeHTML(item.preview)}" alt="${escapeHTML(item.name)}">
      <div class="import-main">
        <div class="import-row primary-row">
          <input class="el-input-native" data-import-field="name" value="${escapeHTML(item.name)}" aria-label="名称" placeholder="名称">
          <input class="el-input-native" data-import-field="meaning" value="${escapeHTML(item.meaning)}" aria-label="含义" placeholder="含义；为空时自动关闭 AI">
        </div>
        <div class="import-row settings-row">
          <input class="el-input-native" data-import-field="keywords" value="${escapeHTML(item.keywords)}" aria-label="关键词" placeholder="关键词（中文或英文逗号分隔）">
          <select class="el-select" data-import-field="groupIds" multiple>${groupOptions(item.groupIds)}</select>
          <select class="el-select" data-import-field="roleScope">
            <option value="all_characters" ${item.roleScope === "all_characters" ? "selected" : ""}>全部角色</option>
            <option value="selected_characters" ${item.roleScope === "selected_characters" ? "selected" : ""}>指定角色</option>
          </select>
          <select class="el-select" data-import-field="characterIds" multiple ${item.roleScope === "selected_characters" ? "" : "disabled"}>${characterOptions(item.characterIds)}</select>
          <label class="el-switch"><input type="checkbox" data-import-field="aiEnabled" ${item.aiEnabled ? "checked" : ""}><span class="switch-track"></span><span>AI</span></label>
        </div>
      </div>
      <div class="import-item-actions">
        <span class="status" data-status="${escapeHTML(item.status)}">${escapeHTML(item.status)}</span>
        <button type="button" class="el-button text circle" data-remove-import="${index}" aria-label="移除">${icons.delete}</button>
      </div>
    </div>
  `).join("");

  $("import-list").querySelectorAll(".import-item").forEach((row) => {
    const item = state.imports[Number(row.dataset.importIndex)];
    row.querySelectorAll("[data-import-field]").forEach((control) => {
      const key = control.dataset.importField;
      control.addEventListener("input", () => {
        item[key] = control.type === "checkbox" ? control.checked : control.value;
      });
      control.addEventListener("change", () => {
        if (key === "groupIds" || key === "characterIds") item[key] = [...control.selectedOptions].map((option) => option.value);
        else if (key === "roleScope") {
          item.roleScope = control.value;
          const characterSelect = row.querySelector("[data-import-field='characterIds']");
          characterSelect.disabled = item.roleScope !== "selected_characters";
          characterSelect.placeholder = item.roleScope === "selected_characters" ? "选择角色" : "全部角色";
        } else if (control.type === "checkbox") item.aiEnabled = control.checked;
        else item[key] = control.value;
      });
    });
  });
  $("import-list").querySelectorAll("[data-remove-import]").forEach((button) => {
    button.onclick = () => {
      const index = Number(button.dataset.removeImport);
      URL.revokeObjectURL(state.imports[index].preview);
      state.imports.splice(index, 1);
      renderImports();
    };
  });
}

function applyDefaults() {
  const meaning = $("default-meaning").value;
  const keywords = $("default-keywords").value;
  const groupIds = [...$("default-group").selectedOptions].map((option) => option.value);
  const aiEnabled = $("default-ai").checked;
  for (const item of state.imports) Object.assign(item, { meaning, keywords, groupIds, aiEnabled });
  renderImports();
}

function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  const size = 0x8000;
  for (let index = 0; index < bytes.length; index += size) {
    binary += String.fromCharCode(...bytes.subarray(index, index + size));
  }
  return btoa(binary);
}

async function uploadChunks(uploadId, kind, data) {
  const size = 256 * 1024;
  for (let offset = 0; offset < data.byteLength; offset += size) {
    const chunk = data.slice(offset, offset + size);
    await call("upload.chunk", {
      uploadId,
      kind,
      data: arrayBufferToBase64(await chunk.arrayBuffer()),
    });
  }
}

async function imageFallbacks(file) {
  const bitmap = await createImageBitmap(file);
  function canvasBlob(maxSize) {
    const scale = Math.min(1, maxSize / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(bitmap.width * scale));
    canvas.height = Math.max(1, Math.round(bitmap.height * scale));
    canvas.getContext("2d").drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    return new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
  }
  return {
    thumbnail: await canvasBlob(160),
    fallback: await canvasBlob(1024),
    width: bitmap.width,
    height: bitmap.height,
  };
}

async function submitImport() {
  if (!state.imports.length || state.importing) return;
  state.importing = true;
  $("confirm-import").disabled = true;
  try {
    for (const item of state.imports) {
      if (item.status === "成功" || item.status === "重复") continue;
      item.status = "处理中";
      renderImports();
      const extension = item.file.name.split(".").pop().toLowerCase();
      const upload = await call("upload.begin", { extension });
      const original = await item.file.arrayBuffer();
      await uploadChunks(upload.uploadId, "original", original);
      let fallbackInfo;
      try {
        fallbackInfo = await imageFallbacks(item.file);
      } catch {
        fallbackInfo = { thumbnail: new Blob([original], { type: item.file.type || "image/png" }), fallback: new Blob([original], { type: item.file.type || "image/png" }), width: 0, height: 0 };
      }
      await uploadChunks(upload.uploadId, "thumbnail", await fallbackInfo.thumbnail.arrayBuffer());
      await uploadChunks(upload.uploadId, "fallback", await fallbackInfo.fallback.arrayBuffer());
      const result = await call("upload.complete", {
        uploadId: upload.uploadId,
        name: item.name,
        meaning: item.meaning,
        keywords: parseKeywords(item.keywords),
        originalFilename: item.file.name,
        mimeType: item.file.type,
        fileExtension: `.${extension}`,
        fileSize: item.file.size,
        width: fallbackInfo.width,
        height: fallbackInfo.height,
        isAnimated: extension === "gif",
        frameCount: 1,
        aiEnabled: item.aiEnabled,
        roleScope: item.roleScope,
        characterIds: item.characterIds,
        groupIds: item.groupIds,
      });
      item.status = result.status === "duplicate" ? "重复" : "成功";
      renderImports();
    }
    await loadEmotes();
    toast(`导入完成：${state.imports.length} 个文件`);
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  } finally {
    state.importing = false;
    $("confirm-import").disabled = false;
  }
}

document.querySelectorAll("[data-view]").forEach((button) => {
  button.onclick = () => selectView(button.dataset.view);
});

$("group-list").addEventListener("click", (event) => {
  const menuButton = event.target.closest("[data-group-menu]");
  if (menuButton) {
    state.groupMenuId = state.groupMenuId === menuButton.dataset.groupMenu ? "" : menuButton.dataset.groupMenu;
    renderGroups();
    return;
  }
  const groupButton = event.target.closest("[data-group]");
  if (groupButton) {
    selectGroup(groupButton.dataset.group);
    return;
  }
  const actionButton = event.target.closest("[data-group-action]");
  if (actionButton) void groupAction(actionButton.dataset.groupAction, actionButton.dataset.id);
});

document.addEventListener("click", (event) => {
  if (!state.groupMenuId) return;
  if (event.target.closest(".group-menu") || event.target.closest("[data-group-menu]")) return;
  state.groupMenuId = "";
  renderGroups();
});

$("create-group").onclick = createGroup;
$("search").oninput = () => {
  state.search = $("search").value;
  $("clear-search").hidden = !state.search;
  clearTimeout(window.__emoteSearchTimer);
  window.__emoteSearchTimer = setTimeout(() => void loadEmotes(), 250);
};
$("clear-search").onclick = () => {
  state.search = "";
  $("search").value = "";
  $("clear-search").hidden = true;
  void loadEmotes();
};
$("refresh").onclick = () => void loadEmotes();
$("import-files").onclick = () => $("file-input").click();
$("import-folder").onclick = () => $("folder-input").click();
$("file-input").onchange = (event) => {
  openFiles([...event.target.files]);
  event.target.value = "";
};
$("folder-input").onchange = (event) => {
  openFiles([...event.target.files]);
  event.target.value = "";
};
$("bulk-group").onchange = addSelectedToGroup;
$("bulk-ai-on").onclick = () => bulkUpdate({ aiEnabled: true });
$("bulk-ai-off").onclick = () => bulkUpdate({ aiEnabled: false });
$("bulk-remove-group").onclick = removeSelectedFromGroup;
$("bulk-delete").onclick = deleteSelected;
$("select-all").onchange = (event) => {
  if (event.target.checked) state.emotes.forEach((item) => state.selectedIds.add(item.id));
  else state.emotes.forEach((item) => state.selectedIds.delete(item.id));
  renderGrid();
  renderGroups();
};
$("apply-defaults").onclick = applyDefaults;
$("confirm-import").onclick = submitImport;
$("cancel-import").onclick = () => {
  $("import-dialog").close();
  state.imports = [];
};
$("close-import").onclick = () => {
  $("import-dialog").close();
  state.imports = [];
};
$("import-dialog").addEventListener("close", () => {
  state.imports = [];
  renderImports();
});

const dropZone = $("drop-zone");
dropZone.addEventListener("dragover", (event) => {
  event.preventDefault();
  dropZone.classList.add("drop-active");
});
dropZone.addEventListener("dragleave", () => dropZone.classList.remove("drop-active"));
dropZone.addEventListener("drop", (event) => {
  event.preventDefault();
  dropZone.classList.remove("drop-active");
  openFiles([...event.dataTransfer.files]);
});

renderGroups();
renderGrid();
renderDetail();
renderGroupOptions();
window.amitiaUI.ready().then(async () => {
  void loadAll();
}).catch((error) => toast(error.message || String(error)));
