package com.amitia.feature.computeruse

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun ComputerUseHomeScreen(
    onOpenPermissionMode: () -> Unit,
    onOpenSystemPermission: () -> Unit,
    onOpenSession: () -> Unit,
    onOpenApprovalQueue: () -> Unit,
    onOpenHistory: () -> Unit,
    onOpenSafetyRules: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val overview by viewModel.overview.collectAsStateWithLifecycle()
    val devices by viewModel.devices.collectAsStateWithLifecycle()
    val sessions by viewModel.sessions.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    ComputerUseHomeContent(
        overview = overview,
        devices = devices,
        recentSessions = sessions,
        loading = loading,
        onToggle = viewModel::toggleComputerUse,
        onOpenPermissionMode = onOpenPermissionMode,
        onOpenSystemPermission = onOpenSystemPermission,
        onOpenSession = onOpenSession,
        onOpenApprovalQueue = onOpenApprovalQueue,
        onOpenHistory = onOpenHistory,
        onOpenSafetyRules = onOpenSafetyRules
    )
}

@Composable
fun ComputerUseHomeContent(
    overview: ComputerUseOverview,
    devices: List<ControllableDevice>,
    recentSessions: List<ComputerUseSession>,
    loading: Boolean,
    onToggle: (Boolean) -> Unit,
    onOpenPermissionMode: () -> Unit,
    onOpenSystemPermission: () -> Unit,
    onOpenSession: () -> Unit,
    onOpenApprovalQueue: () -> Unit,
    onOpenHistory: () -> Unit,
    onOpenSafetyRules: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "Computer Use")
        if (loading) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载 Computer Use...")
            }
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "master_switch") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        AmitiaSwitchRow(
                            title = "Computer Use 总开关",
                            subtitle = if (overview.enabled) "已启用，当前模式：${overview.currentMode.label}" else "已关闭",
                            checked = overview.enabled,
                            onCheckedChange = onToggle,
                            leadingIcon = AmitiaIcons.PowerSettingsNew
                        )
                    }
                }
                item(key = "risk") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                    ) {
                        Row(
                            modifier = Modifier.padding(AmitiaSpacing.Base),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Warning,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.tertiary,
                                modifier = Modifier.size(AmitiaIconSize.Medium)
                            )
                            Text(
                                text = overview.riskDescription,
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
                item(key = "mode_entry") {
                    AmitiaEntryCard(
                        onClick = onOpenPermissionMode,
                        leading = {
                            Icon(AmitiaIcons.Security, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium))
                        },
                        title = "权限模式",
                        subtitle = "当前：${overview.currentMode.label} · ${overview.currentMode.risk}"
                    )
                }
                item(key = "devices_header") {
                    AmitiaSectionHeader(title = "可控制设备", trailing = {
                        Text("${devices.size}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    })
                }
                if (devices.isEmpty()) {
                    item(key = "devices_empty") {
                        AmitiaEmptyState(
                            icon = AmitiaIcons.Devices,
                            title = "暂无可控制设备",
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                } else {
                    items(devices, key = { it.id }) { device ->
                        DeviceRow(device = device)
                    }
                }
                item(key = "pending_entry") {
                    AmitiaEntryCard(
                        onClick = onOpenApprovalQueue,
                        leading = {
                            Icon(AmitiaIcons.HourglassEmpty, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium))
                        },
                        title = "待审批",
                        subtitle = "${overview.pendingApprovalCount} 个待处理操作"
                    )
                }
                item(key = "recent_header") { AmitiaSectionHeader(title = "最近会话") }
                if (recentSessions.isEmpty()) {
                    item(key = "recent_empty") {
                        Text(
                            text = "暂无运行会话",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(AmitiaSpacing.Sm)
                        )
                    }
                } else {
                    items(recentSessions, key = { it.id }) { session ->
                        RecentSessionRow(session = session, onClick = onOpenSession)
                    }
                }
                item(key = "more_entries") {
                    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        AmitiaEntryCard(
                            onClick = onOpenSystemPermission,
                            leading = { Icon(AmitiaIcons.Lock, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium)) },
                            title = "系统权限",
                            subtitle = "Android 无障碍、屏幕捕获等"
                        )
                        AmitiaEntryCard(
                            onClick = onOpenHistory,
                            leading = { Icon(AmitiaIcons.History, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium)) },
                            title = "操作历史",
                            subtitle = "查看历史操作记录"
                        )
                        AmitiaEntryCard(
                            onClick = onOpenSafetyRules,
                            leading = { Icon(AmitiaIcons.Shield, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium)) },
                            title = "安全规则",
                            subtitle = "配置禁止项与保护规则"
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DeviceRow(device: ControllableDevice) {
    val statusColor = when (device.status) {
        com.amitia.core.designsystem.component.AmitiaStatusType.Connected, com.amitia.core.designsystem.component.AmitiaStatusType.Running -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(40.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Devices,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = device.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${device.type.label} · ${device.lastActive ?: "未连接"}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
        }
    }
}

@Composable
private fun RecentSessionRow(session: ComputerUseSession, onClick: () -> Unit) {
    AmitiaEntryCard(
        onClick = onClick,
        leading = {
            Icon(AmitiaIcons.PlayArrow, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(AmitiaIconSize.Medium))
        },
        title = session.target,
        subtitle = "${session.status.label} · ${session.currentStep} · ${session.startedAt}"
    )
}

@Preview(name = "Computer Use Home - Light", showBackground = true)
@Composable
private fun ComputerUseHomeLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ComputerUseHomeContent(
            overview = ComputerUseOverview(true, PermissionMode.ManualApproval, 2, 1, 2, "谨慎配置"),
            devices = listOf(ControllableDevice("d1", "本机", DeviceType.Android, com.amitia.core.designsystem.component.AmitiaStatusType.Connected, "2分钟前")),
            recentSessions = listOf(ComputerUseSession("s1", "整理桌面文件", SessionStatus.Running, "扫描文件", "桌面 12 个文件夹", emptyList(), "14:30")),
            loading = false,
            onToggle = {}, onOpenPermissionMode = {}, onOpenSystemPermission = {},
            onOpenSession = {}, onOpenApprovalQueue = {}, onOpenHistory = {}, onOpenSafetyRules = {}
        )
    }
}

@Preview(name = "Computer Use Home - Dark", showBackground = true)
@Composable
private fun ComputerUseHomeDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ComputerUseHomeContent(
            overview = ComputerUseOverview(),
            devices = emptyList(),
            recentSessions = emptyList(),
            loading = true,
            onToggle = {}, onOpenPermissionMode = {}, onOpenSystemPermission = {},
            onOpenSession = {}, onOpenApprovalQueue = {}, onOpenHistory = {}, onOpenSafetyRules = {}
        )
    }
}
