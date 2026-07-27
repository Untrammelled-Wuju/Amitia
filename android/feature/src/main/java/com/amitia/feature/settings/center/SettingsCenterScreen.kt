package com.amitia.feature.settings.center

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.ScreenState

data class SettingsEntry(
    val title: String,
    val subtitle: String,
    val icon: ImageVector,
    val route: String
)

data class SettingsGroup(
    val title: String,
    val entries: List<SettingsEntry>
)

@Composable
fun SettingsCenterScreen(
    onBack: () -> Unit,
    onNavigate: (String) -> Unit,
    state: ScreenState<List<SettingsGroup>> = ScreenState.Content(rememberSettingsGroups())
) {
    when (state) {
        is ScreenState.Loading -> SettingsCenterLoading()
        is ScreenState.Content -> SettingsCenterContent(
            groups = state.data,
            onBack = onBack,
            onNavigate = onNavigate
        )
        is ScreenState.Empty -> AmitiaEmptyState(
            icon = AmitiaIcons.Settings,
            title = "暂无设置项",
            description = "设置项加载失败，请稍后重试"
        )
        is ScreenState.Error -> AmitiaEmptyState(
            icon = AmitiaIcons.Error,
            title = "加载失败",
            description = state.error.message
        )
        is ScreenState.Partial -> SettingsCenterContent(
            groups = state.data,
            onBack = onBack,
            onNavigate = onNavigate
        )
    }
}

@Composable
private fun SettingsCenterContent(
    groups: List<SettingsGroup>,
    onBack: () -> Unit,
    onNavigate: (String) -> Unit
) {
    AmitiaPageScaffold(
        topBar = {
            AmitiaTopBar(
                title = "设置中心",
                onBack = onBack
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            groups.forEach { group ->
                AmitiaSection(title = group.title) {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Column {
                            group.entries.forEachIndexed { index, entry ->
                                SettingsRow(
                                    title = entry.title,
                                    subtitle = entry.subtitle,
                                    leadingIcon = entry.icon,
                                    onClick = { onNavigate(entry.route) }
                                )
                                if (index < group.entries.lastIndex) {
                                    AmitiaInsetDivider(
                                        leadingInset = 56.dp + AmitiaSpacing.Base
                                    )
                                }
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun SettingsCenterLoading() {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        repeat(4) { LoadingSkeleton(lineCount = 3) }
    }
}

private fun rememberSettingsGroups(): List<SettingsGroup> = listOf(
    SettingsGroup(
        title = "账号",
        entries = listOf(
            SettingsEntry("账号信息", "用户名、登录方式、设备", AmitiaIcons.Person, "account"),
            SettingsEntry("外观", "主题、动态色、模糊", AmitiaIcons.Palette, "appearance")
        )
    ),
    SettingsGroup(
        title = "通知与隐私",
        entries = listOf(
            SettingsEntry("通知设置", "消息通知、免打扰", AmitiaIcons.Notifications, "notification"),
            SettingsEntry("隐私", "数据存储、日志脱敏", AmitiaIcons.Shield, "privacy"),
            SettingsEntry("安全", "应用锁、加密、认证", AmitiaIcons.Security, "security"),
            SettingsEntry("应用锁", "PIN、生物识别", AmitiaIcons.Lock, "app_lock")
        )
    ),
    SettingsGroup(
        title = "数据",
        entries = listOf(
            SettingsEntry("数据与存储", "存储用量、缓存", AmitiaIcons.Storage, "data_storage"),
            SettingsEntry("备份与恢复", "备份、恢复、加密", AmitiaIcons.Backup, "backup"),
            SettingsEntry("导入导出", "角色、对话、记忆", AmitiaIcons.ImportExport, "import_export")
        )
    ),
    SettingsGroup(
        title = "运行",
        entries = listOf(
            SettingsEntry("运行模式", "本地/远程切换", AmitiaIcons.CloudDone, "run_mode"),
            SettingsEntry("本地运行时", "服务状态、资源", AmitiaIcons.Build, "local_runtime"),
            SettingsEntry("开机自启动", "自启动、电池优化", AmitiaIcons.PowerSettingsNew, "autostart"),
            SettingsEntry("网络与代理", "代理、DNS、证书", AmitiaIcons.Router, "network")
        )
    ),
    SettingsGroup(
        title = "系统",
        entries = listOf(
            SettingsEntry("权限管理", "系统、角色、扩展权限", AmitiaIcons.VerifiedUser, "permission"),
            SettingsEntry("语言", "语言和区域", AmitiaIcons.Language, "language"),
            SettingsEntry("辅助功能", "无障碍设置", AmitiaIcons.Accessibility, "accessibility"),
            SettingsEntry("更新", "版本、更新检查", AmitiaIcons.Update, "update")
        )
    ),
    SettingsGroup(
        title = "关于",
        entries = listOf(
            SettingsEntry("关于", "版本、团队、致谢", AmitiaIcons.Info, "about"),
            SettingsEntry("开源许可", "第三方组件许可", AmitiaIcons.Code, "licenses"),
            SettingsEntry("反馈", "问题反馈、建议", AmitiaIcons.Feedback, "feedback"),
            SettingsEntry("崩溃恢复", "崩溃日志、恢复", AmitiaIcons.BugReport, "crash"),
            SettingsEntry("开发者选项", "调试、实验功能", AmitiaIcons.Terminal, "developer")
        )
    )
)

@Preview(name = "设置首页 - 亮色", showBackground = true)
@Composable
private fun SettingsCenterScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SettingsCenterScreen(
            onBack = {},
            onNavigate = {}
        )
    }
}

@Preview(name = "设置首页 - 暗色", showBackground = true)
@Composable
private fun SettingsCenterScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SettingsCenterScreen(
            onBack = {},
            onNavigate = {}
        )
    }
}
