package com.amitia.feature.settings.more

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.LocalIsDarkTheme

data class MoreEntry(
    val title: String,
    val subtitle: String,
    val icon: ImageVector,
    val route: String
)

data class MoreSection(
    val title: String,
    val entries: List<MoreEntry>
)

@Composable
fun MoreScreen(
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    onNavigate: (String) -> Unit
) {
    val parentIsDark = LocalIsDarkTheme.current
    var isDark by remember { mutableStateOf(parentIsDark) }
    AmitiaTheme(darkTheme = isDark) {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = MaterialTheme.colorScheme.background
        ) {
            MoreScreenContent(
                onBack = onBack,
                onMenu = onMenu,
                onNavigate = onNavigate,
                onToggleTheme = { isDark = !isDark }
            )
        }
    }
}

@Composable
private fun MoreScreenContent(
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    onNavigate: (String) -> Unit,
    onToggleTheme: () -> Unit
) {
    val sections = rememberMoreSections()
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = AmitiaContentPadding.Horizontal)
    ) {
        TopLineBar(onMenu = onMenu, onToggleTheme = onToggleTheme)
        sections.forEach { section ->
            SettingGroup(
                title = section.title,
                entries = section.entries,
                onNavigate = onNavigate
            )
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
    }
}

@Composable
private fun TopLineBar(onMenu: () -> Unit, onToggleTheme: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .statusBarsPadding()
            .padding(vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        AmitiaGlassSurface(
            level = GlassLevel.Chip,
            modifier = Modifier.size(44.dp),
            shape = RoundedCornerShape(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button,
                        onClick = onMenu
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Menu,
                    contentDescription = "菜单",
                    tint = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = "更多",
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onSurface,
                fontSize = 27.sp,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = "模型、渠道、能力与系统设置。",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        AmitiaGlassSurface(
            level = GlassLevel.Chip,
            modifier = Modifier.size(44.dp),
            shape = RoundedCornerShape(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button,
                        onClick = onToggleTheme
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.LightMode,
                    contentDescription = "主题切换",
                    tint = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
        }
    }
}

@Composable
private fun SettingGroup(
    title: String,
    entries: List<MoreEntry>,
    onNavigate: (String) -> Unit
) {
    val isPreferenceGroup = title == "偏好"
    Column(modifier = Modifier.padding(bottom = 17.dp)) {
        Text(
            text = title,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
            fontSize = 11.sp,
            modifier = Modifier.padding(start = 4.dp, end = 4.dp, bottom = 8.dp)
        )
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(22.dp),
            color = MaterialTheme.colorScheme.surface
        ) {
            Column {
                entries.forEachIndexed { index, entry ->
                    SettingRowItem(
                        entry = entry,
                        showSwitch = isPreferenceGroup,
                        onNavigate = onNavigate
                    )
                    if (index < entries.lastIndex) {
                        HorizontalDivider(
                            modifier = Modifier.fillMaxWidth(),
                            thickness = 1.dp,
                            color = MaterialTheme.colorScheme.outlineVariant
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SettingRowItem(
    entry: MoreEntry,
    showSwitch: Boolean,
    onNavigate: (String) -> Unit
) {
    var switchState by remember { mutableStateOf(true) }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .then(
                if (!showSwitch) {
                    Modifier.clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        role = Role.Button,
                        onClick = { onNavigate(entry.route) }
                    )
                } else Modifier
            )
            .padding(horizontal = 11.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(38.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = entry.icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(20.dp)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = entry.title,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = entry.subtitle,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontSize = 10.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        if (showSwitch) {
            Switch(
                checked = switchState,
                onCheckedChange = { switchState = it },
                colors = SwitchDefaults.colors(
                    checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                    checkedTrackColor = MaterialTheme.colorScheme.primary,
                    uncheckedThumbColor = MaterialTheme.colorScheme.surface,
                    uncheckedTrackColor = MaterialTheme.colorScheme.surfaceVariant
                )
            )
        } else {
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

@Composable
fun MoreScreenScaffold(
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    onNavigate: (String) -> Unit
) {
    MoreScreen(onBack = onBack, onMenu = onMenu, onNavigate = onNavigate)
}

private fun rememberMoreSections(): List<MoreSection> = listOf(
    MoreSection(
        title = "常用功能",
        entries = listOf(
            MoreEntry("模型中心", "文本/视觉/语音/向量", AmitiaIcons.SmartToy, "models")
        )
    ),
    MoreSection(
        title = "系统与数据",
        entries = listOf(
            MoreEntry("数据管理", "存储、缓存、备份", AmitiaIcons.Database, "data_storage"),
            MoreEntry("外观", "主题、字体、动效", AmitiaIcons.Palette, "settings_center"),
            MoreEntry("安全与权限", "权限、加密、审计", AmitiaIcons.Lock, "developer"),
            MoreEntry("高级控制台", "运行时高级控制", AmitiaIcons.Terminal, "console")
        )
    ),
    MoreSection(
        title = "偏好",
        entries = listOf(
            MoreEntry("主动消息", "角色主动发送消息", AmitiaIcons.Notifications, "proactive"),
            MoreEntry("玻璃模糊", "界面玻璃态效果", AmitiaIcons.Contrast, "blur")
        )
    )
)

@Preview(name = "更多页 - 亮色", showBackground = true)
@Composable
private fun MoreScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MoreScreenScaffold(
            onBack = {},
            onNavigate = {}
        )
    }
}

@Preview(name = "更多页 - 暗色", showBackground = true)
@Composable
private fun MoreScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MoreScreenScaffold(
            onBack = {},
            onNavigate = {}
        )
    }
}
