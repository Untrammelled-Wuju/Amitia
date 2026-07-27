package com.amitia.feature.creativeworkshop

enum class CwProjectType(val label: String, val description: String) {
    Skill("技能", "提供可被角色调用的能力"),
    Plugin("插件", "扩展系统功能模块"),
    McpConfig("MCP 配置包", "模型上下文协议配置"),
    UiContribution("UI 贡献", "贡献自定义界面组件"),
    Comprehensive("综合扩展", "包含多种能力的扩展包")
}

enum class CwProjectStatus(val label: String) {
    Draft("草稿"),
    Editing("编辑中"),
    Building("构建中"),
    Built("已构建"),
    Testing("测试中"),
    Published("已发布")
}

data class CwProject(
    val id: String,
    val name: String,
    val type: CwProjectType,
    val version: String,
    val description: String,
    val status: CwProjectStatus,
    val lastModified: Long,
    val path: String? = null,
    val template: String? = null
)

data class CwTemplate(
    val id: String,
    val name: String,
    val type: CwProjectType,
    val description: String
)

data class CwManifest(
    val name: String,
    val version: String,
    val description: String,
    val author: String,
    val entryPoint: String,
    val dependencies: List<String> = emptyList(),
    val minRuntimeVersion: String,
    val license: String = "MIT"
)

enum class CwSchemaNodeType(val label: String) {
    Form("表单"),
    List("列表"),
    Detail("详情"),
    ButtonGroup("按钮组"),
    Permission("权限说明"),
    ResourceList("资源清单"),
    Diagnostic("诊断"),
    Wizard("简单向导"),
    Confirm("确认界面")
}

data class CwSchemaNode(
    val id: String,
    val type: CwSchemaNodeType,
    val label: String,
    val required: Boolean = false,
    val children: List<CwSchemaNode> = emptyList()
)

enum class CwPermissionRisk(val label: String) {
    Low("低"),
    Medium("中"),
    High("高")
}

data class CwPermission(
    val id: String,
    val name: String,
    val description: String,
    val risk: CwPermissionRisk,
    val granted: Boolean,
    val required: Boolean,
    val usage: String
)

data class CwBuildConfig(
    val target: String,
    val signEnabled: Boolean,
    val outputName: String,
    val outputPath: String,
    val minify: Boolean = true,
    val includeSourceMap: Boolean = false
)

data class CwBuildStep(
    val name: String,
    val status: CwBuildStepStatus,
    val message: String? = null,
    val duration: Long = 0
)

enum class CwBuildStepStatus(val label: String) {
    Pending("等待中"),
    Running("执行中"),
    Success("成功"),
    Failed("失败"),
    Skipped("跳过")
}

data class CwBuildResult(
    val success: Boolean,
    val errors: List<String>,
    val warnings: List<String>,
    val outputSize: Long,
    val duration: Long,
    val steps: List<CwBuildStep>
)

data class CwPublishInfo(
    val packageName: String,
    val description: String,
    val exportPath: String,
    val fileSize: Long,
    val checksum: String
)

enum class CwTestStatus(val label: String) {
    Passed("通过"),
    Failed("失败"),
    Skipped("跳过"),
    Running("运行中"),
    Pending("等待中")
}

data class CwTestComponent(
    val id: String,
    val name: String,
    val type: String,
    val status: CwTestStatus,
    val duration: Long,
    val message: String?
)

data class CwFileNode(
    val name: String,
    val path: String,
    val isDirectory: Boolean,
    val size: Long = 0,
    val children: List<CwFileNode> = emptyList()
)
