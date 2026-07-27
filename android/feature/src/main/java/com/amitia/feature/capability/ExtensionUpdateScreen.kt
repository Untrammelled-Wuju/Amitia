package com.amitia.feature.capability

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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun ExtensionUpdateScreen(
    onBack: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val updates by viewModel.updates.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    ExtensionUpdateContent(
        updates = updates,
        loading = loading,
        onBack = onBack
    )
}

@Composable
fun ExtensionUpdateContent(
    updates: List<ExtensionUpdateInfo>,
    loading: Boolean,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "扩展更新", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载更新...")
            }
            updates.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Update,
                title = "暂无可用更新",
                description = "所有扩展均为最新版本",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "summary") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
                    ) {
                        Text(
                            text = "共 ${updates.size} 个可更新，更新失败将自动回滚",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
                items(updates, key = { it.extensionId }) { update ->
                    UpdateCard(update = update)
                }
            }
        }
    }
}

@Composable
private fun UpdateCard(update: ExtensionUpdateInfo) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(
                    imageVector = AmitiaIcons.Extension,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary
                )
                Column(modifier = Modifier.weight(1f).padding(start = AmitiaSpacing.Sm)) {
                    Text(
                        text = update.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1, overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "${update.currentVersion} → ${update.newVersion}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.primary,
                        fontWeight = FontWeight.Medium
                    )
                }
                Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.surfaceVariant) {
                    Text(
                        text = update.updateMethod.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            Text(
                text = "更新日志",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = update.changelog,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            if (update.permissionChanges.isNotEmpty()) {
                AmitiaSectionHeader(title = "权限变化")
                update.permissionChanges.forEach { change ->
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(vertical = AmitiaSpacing.Xxs),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.WarningAmber,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.tertiary,
                            modifier = Modifier.size(16.dp)
                        )
                        Text(
                            text = change,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
            }
            if (update.failedRollback) {
                WarningBanner(
                    message = "上次更新失败，已回滚到稳定版本"
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (update.failedRollback) {
                    DangerButton(
                        text = "重试更新",
                        onClick = {},
                        leadingIcon = AmitiaIcons.Refresh,
                        modifier = Modifier.weight(1f)
                    )
                }
                PrimaryButton(
                    text = "更新",
                    onClick = {},
                    leadingIcon = AmitiaIcons.Download,
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Preview(name = "Extension Update - Light", showBackground = true)
@Composable
private fun ExtensionUpdateLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ExtensionUpdateContent(
            updates = listOf(
                ExtensionUpdateInfo("pub-1", "天气查询", "1.2.0", "1.3.0",
                    permissionChanges = listOf("新增位置信息访问权限"),
                    changelog = "优化查询速度，新增降雨预警", updateMethod = UpdateMethod.Manual)
            ),
            loading = false, onBack = {}
        )
    }
}

@Preview(name = "Extension Update - Dark", showBackground = true)
@Composable
private fun ExtensionUpdateDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ExtensionUpdateContent(
            updates = emptyList(), loading = false, onBack = {}
        )
    }
}
