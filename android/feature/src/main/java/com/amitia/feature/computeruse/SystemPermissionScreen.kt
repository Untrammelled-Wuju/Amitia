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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun SystemPermissionScreen(
    onBack: () -> Unit,
    viewModel: ComputerUseViewModel = hiltViewModel()
) {
    val permissions by viewModel.permissions.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    SystemPermissionContent(
        permissions = permissions,
        loading = loading,
        onBack = onBack
    )
}

@Composable
fun SystemPermissionContent(
    permissions: List<SystemPermissionState>,
    loading: Boolean,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "系统权限", onBack = onBack)
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item(key = "warn") {
                WarningBanner(
                    message = "以下为 Android 系统实际权限状态，不会用应用内开关伪装已授权"
                )
            }
            if (loading) {
                item(key = "loading") {
                    Box(modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Xl), contentAlignment = Alignment.Center) {
                        InlineLoading(message = "加载权限状态...")
                    }
                }
            } else if (permissions.isEmpty()) {
                item(key = "empty") {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.Lock,
                        title = "暂无系统权限",
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            } else {
                items(permissions, key = { it.name }) { permission ->
                    SystemPermissionRow(permission = permission)
                }
            }
        }
    }
}

@Composable
private fun SystemPermissionRow(permission: SystemPermissionState) {
    val accentColor = if (permission.granted) MaterialTheme.colorScheme.tertiary
    else MaterialTheme.colorScheme.error
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = if (permission.granted) MaterialTheme.colorScheme.surface
        else MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.2f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(40.dp).clip(CircleShape)
                    .background(if (permission.granted) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.errorContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (permission.granted) AmitiaIcons.CheckCircle else AmitiaIcons.Lock,
                    contentDescription = null,
                    tint = accentColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = permission.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = permission.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = if (permission.granted) "已授权" else "未授权 · 需前往 ${permission.settingsAction}",
                    style = MaterialTheme.typography.labelSmall,
                    color = accentColor
                )
            }
            if (!permission.granted) {
                PrimaryButton(
                    text = "前往",
                    onClick = {}
                )
            }
        }
    }
}

@Preview(name = "System Permission - Light", showBackground = true)
@Composable
private fun SystemPermissionLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SystemPermissionContent(
            permissions = listOf(
                SystemPermissionState("无障碍服务", "控制和读取屏幕内容", false, true, "无障碍设置"),
                SystemPermissionState("屏幕捕获", "捕获屏幕截图", true, true, "权限管理")
            ),
            loading = false, onBack = {}
        )
    }
}

@Preview(name = "System Permission - Dark", showBackground = true)
@Composable
private fun SystemPermissionDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SystemPermissionContent(
            permissions = emptyList(), loading = true, onBack = {}
        )
    }
}
