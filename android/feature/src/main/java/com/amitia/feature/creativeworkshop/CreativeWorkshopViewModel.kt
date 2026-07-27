package com.amitia.feature.creativeworkshop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.EmptyReason
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class CreativeWorkshopUiState(
    val projectList: ScreenState<List<CwProject>> = ScreenState.Loading,
    val templates: List<CwTemplate> = emptyList(),
    val recentProjects: List<CwProject> = emptyList(),
    val projectDetail: ScreenState<Pair<CwProject, List<CwFileNode>>> = ScreenState.Loading,
    val manifest: ScreenState<CwManifest> = ScreenState.Loading,
    val manifestJson: String = "",
    val schemaNodes: ScreenState<List<CwSchemaNode>> = ScreenState.Loading,
    val schemaJson: String = "",
    val permissions: ScreenState<List<CwPermission>> = ScreenState.Loading,
    val buildConfig: ScreenState<CwBuildConfig> = ScreenState.Loading,
    val buildResult: CwBuildResult? = null,
    val isBuilding: Boolean = false,
    val publishInfo: ScreenState<CwPublishInfo> = ScreenState.Loading,
    val testComponents: ScreenState<List<CwTestComponent>> = ScreenState.Loading,
    val isTesting: Boolean = false
)

@HiltViewModel
class CreativeWorkshopViewModel @Inject constructor() : ViewModel() {

    private val _state = MutableStateFlow(CreativeWorkshopUiState())
    val state: StateFlow<CreativeWorkshopUiState> = _state.asStateFlow()

    init {
        loadProjectList()
    }

    fun loadProjectList() {
        _state.value = _state.value.copy(projectList = ScreenState.Loading)
        viewModelScope.launch {
            delay(400)
            val projects = mockProjects()
            _state.value = _state.value.copy(
                projectList = ScreenState.Content(projects),
                templates = mockTemplates(),
                recentProjects = projects.take(3)
            )
        }
    }

    fun loadProjectDetail(projectId: String) {
        _state.value = _state.value.copy(projectDetail = ScreenState.Loading)
        viewModelScope.launch {
            delay(400)
            val project = mockProjects().find { it.id == projectId }
                ?: return@launch run {
                    _state.value = _state.value.copy(
                        projectDetail = ScreenState.Error(
                            UiError("项目不存在", "未找到项目 $projectId")
                        )
                    )
                }
            _state.value = _state.value.copy(
                projectDetail = ScreenState.Content(project to mockFileTree())
            )
        }
    }

    fun loadManifest(projectId: String) {
        _state.value = _state.value.copy(manifest = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            val manifest = CwManifest(
                name = "my-extension",
                version = "1.0.0",
                description = "示例扩展描述",
                author = "Amitia",
                entryPoint = "index.js",
                dependencies = listOf("core >= 1.0.0"),
                minRuntimeVersion = "1.0.0"
            )
            val json = buildManifestJson(manifest)
            _state.value = _state.value.copy(
                manifest = ScreenState.Content(manifest),
                manifestJson = json
            )
        }
    }

    fun updateManifest(manifest: CwManifest) {
        _state.value = _state.value.copy(
            manifest = ScreenState.Content(manifest),
            manifestJson = buildManifestJson(manifest)
        )
    }

    fun loadSchema(projectId: String) {
        _state.value = _state.value.copy(schemaNodes = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            val nodes = mockSchemaNodes()
            val json = buildSchemaJson(nodes)
            _state.value = _state.value.copy(
                schemaNodes = ScreenState.Content(nodes),
                schemaJson = json
            )
        }
    }

    fun updateSchemaJson(json: String) {
        _state.value = _state.value.copy(schemaJson = json)
    }

    fun loadPermissions(projectId: String) {
        _state.value = _state.value.copy(permissions = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            _state.value = _state.value.copy(
                permissions = ScreenState.Content(mockPermissions())
            )
        }
    }

    fun togglePermission(permissionId: String) {
        val current = _state.value.permissions
        if (current is ScreenState.Content) {
            val updated = current.data.map {
                if (it.id == permissionId) it.copy(granted = !it.granted) else it
            }
            _state.value = _state.value.copy(permissions = ScreenState.Content(updated))
        }
    }

    fun loadBuildConfig(projectId: String) {
        _state.value = _state.value.copy(buildConfig = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            _state.value = _state.value.copy(
                buildConfig = ScreenState.Content(
                    CwBuildConfig(
                        target = "android",
                        signEnabled = true,
                        outputName = "my-extension-1.0.0.amx",
                        outputPath = "/output"
                    )
                )
            )
        }
    }

    fun startBuild(projectId: String) {
        _state.value = _state.value.copy(isBuilding = true, buildResult = null)
        viewModelScope.launch {
            delay(2000)
            _state.value = _state.value.copy(
                isBuilding = false,
                buildResult = CwBuildResult(
                    success = true,
                    errors = emptyList(),
                    warnings = listOf("未配置签名证书，将使用默认签名"),
                    outputSize = 256000,
                    duration = 1820,
                    steps = listOf(
                        CwBuildStep("校验", CwBuildStepStatus.Success, null, 120),
                        CwBuildStep("编译", CwBuildStepStatus.Success, null, 800),
                        CwBuildStep("签名", CwBuildStepStatus.Success, null, 300),
                        CwBuildStep("打包", CwBuildStepStatus.Success, null, 600)
                    )
                )
            )
        }
    }

    fun loadPublishInfo(projectId: String) {
        _state.value = _state.value.copy(publishInfo = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            _state.value = _state.value.copy(
                publishInfo = ScreenState.Content(
                    CwPublishInfo(
                        packageName = "my-extension-1.0.0.amx",
                        description = "示例扩展包，包含技能和 UI 贡献",
                        exportPath = "/storage/emulated/0/Download/my-extension-1.0.0.amx",
                        fileSize = 256000,
                        checksum = "a1b2c3d4e5f6"
                    )
                )
            )
        }
    }

    fun loadTestComponents(projectId: String) {
        _state.value = _state.value.copy(testComponents = ScreenState.Loading)
        viewModelScope.launch {
            delay(300)
            _state.value = _state.value.copy(
                testComponents = ScreenState.Content(mockTestComponents())
            )
        }
    }

    fun runTests(projectId: String) {
        _state.value = _state.value.copy(isTesting = true)
        viewModelScope.launch {
            delay(2500)
            val results = mockTestComponents().map {
                it.copy(status = CwTestStatus.Passed, duration = (50..300).random().toLong())
            }
            _state.value = _state.value.copy(
                isTesting = false,
                testComponents = ScreenState.Content(results)
            )
        }
    }

    private fun buildManifestJson(m: CwManifest): String {
        return buildString {
            append("{\n")
            append("  \"name\": \"${m.name}\",\n")
            append("  \"version\": \"${m.version}\",\n")
            append("  \"description\": \"${m.description}\",\n")
            append("  \"author\": \"${m.author}\",\n")
            append("  \"entryPoint\": \"${m.entryPoint}\",\n")
            append("  \"dependencies\": [${m.dependencies.joinToString { "\"$it\"" }}],\n")
            append("  \"minRuntimeVersion\": \"${m.minRuntimeVersion}\",\n")
            append("  \"license\": \"${m.license}\"\n")
            append("}")
        }
    }

    private fun buildSchemaJson(nodes: List<CwSchemaNode>): String {
        return buildString {
            append("{\n  \"nodes\": [\n")
            nodes.forEachIndexed { index, node ->
                append("    { \"id\": \"${node.id}\", \"type\": \"${node.type.name}\", \"label\": \"${node.label}\" }")
                if (index < nodes.lastIndex) append(",")
                append("\n")
            }
            append("  ]\n}")
        }
    }

    private fun mockProjects(): List<CwProject> = listOf(
        CwProject("1", "天气助手", CwProjectType.Skill, "1.2.0", "提供实时天气查询能力", CwProjectStatus.Built, System.currentTimeMillis() - 3600000),
        CwProject("2", "消息卡片", CwProjectType.UiContribution, "0.8.0", "自定义消息展示卡片", CwProjectStatus.Editing, System.currentTimeMillis() - 86400000),
        CwProject("3", "日程工具集", CwProjectType.Comprehensive, "2.0.0", "日程管理和提醒扩展", CwProjectStatus.Published, System.currentTimeMillis() - 172800000),
        CwProject("4", "MCP 搜索配置", CwProjectType.McpConfig, "1.0.0", "搜索引擎 MCP 配置包", CwProjectStatus.Draft, System.currentTimeMillis() - 259200000)
    )

    private fun mockTemplates(): List<CwTemplate> = listOf(
        CwTemplate("t1", "空白技能", CwProjectType.Skill, "从零开始创建一个技能"),
        CwTemplate("t2", "基础插件", CwProjectType.Plugin, "包含标准结构的插件模板"),
        CwTemplate("t3", "MCP 模板", CwProjectType.McpConfig, "预置常用 MCP 配置"),
        CwTemplate("t4", "UI 卡片模板", CwProjectType.UiContribution, "消息卡片贡献模板"),
        CwTemplate("t5", "综合扩展模板", CwProjectType.Comprehensive, "包含技能和 UI 的完整模板")
    )

    private fun mockFileTree(): List<CwFileNode> = listOf(
        CwFileNode("src", "src", true, children = listOf(
            CwFileNode("index.js", "src/index.js", false, 4096),
            CwFileNode("utils.js", "src/utils.js", false, 2048)
        )),
        CwFileNode("manifest.json", "manifest.json", false, 512),
        CwFileNode("schema.json", "schema.json", false, 1024),
        CwFileNode("README.md", "README.md", false, 256)
    )

    private fun mockSchemaNodes(): List<CwSchemaNode> = listOf(
        CwSchemaNode("n1", CwSchemaNodeType.Form, "用户输入表单", required = true),
        CwSchemaNode("n2", CwSchemaNodeType.List, "结果列表"),
        CwSchemaNode("n3", CwSchemaNodeType.Detail, "详情面板"),
        CwSchemaNode("n4", CwSchemaNodeType.ButtonGroup, "操作按钮"),
        CwSchemaNode("n5", CwSchemaNodeType.Permission, "权限说明")
    )

    private fun mockPermissions(): List<CwPermission> = listOf(
        CwPermission("p1", "网络访问", "允许扩展发起网络请求", CwPermissionRisk.Medium, true, true, "用于获取天气数据"),
        CwPermission("p2", "存储读取", "允许读取本地文件", CwPermissionRisk.Low, true, false, "读取缓存数据"),
        CwPermission("p3", "通知发送", "允许发送系统通知", CwPermissionRisk.Medium, false, false, "天气提醒"),
        CwPermission("p4", "位置访问", "获取设备位置信息", CwPermissionRisk.High, false, false, "基于位置的天气")
    )

    private fun mockTestComponents(): List<CwTestComponent> = listOf(
        CwTestComponent("tc1", "工具调用测试", "Tool", CwTestStatus.Pending, 0, null),
        CwTestComponent("tc2", "事件触发测试", "Event", CwTestStatus.Pending, 0, null),
        CwTestComponent("tc3", "Hook 执行测试", "Hook", CwTestStatus.Pending, 0, null),
        CwTestComponent("tc4", "调度任务测试", "Schedule", CwTestStatus.Pending, 0, null),
        CwTestComponent("tc5", "UI 贡献渲染测试", "UIContribution", CwTestStatus.Pending, 0, null)
    )
}
