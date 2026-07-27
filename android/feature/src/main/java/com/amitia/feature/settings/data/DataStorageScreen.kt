package com.amitia.feature.settings.data

import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.SettingsRow
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.feature.settings.StorageInfo
import com.amitia.feature.settings.StorageItem
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun DataStorageScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val storage = state.storage
    var showClearCacheDialog by remember { mutableStateOf(false) }
    var showDeleteDataDialog by remember { mutableStateOf(false) }

    DataStorageScreenContent(
        storage = storage,
        onBack = onBack,
        onClearCache = { showClearCacheDialog = true },
        onDeleteData = { showDeleteDataDialog = true }
    )

    if (showClearCacheDialog) {
        AmitiaConfirmDialog(
            onDismiss = { showClearCacheDialog = false },
            onConfirm = {
                viewModel.clearCache()
                showClearCacheDialog = false
            },
            title = "清理缓存",
            message = "清理本地缓存目录中的临时文件，不会影响对话历史、记忆与角色。",
            confirmText = "清理"
        )
    }
    if (showDeleteDataDialog) {
        AmitiaConfirmDialog(
            onDismiss = { showDeleteDataDialog = false },
            onConfirm = { showDeleteDataDialog = false },
            title = "删除数据",
            message = "此操作将删除所有本地数据，包括对话、记忆和设置。此操作不可恢复。",
            confirmText = "永久删除",
            destructive = true
        )
    }
}

@Composable
private fun DataStorageScreenContent(
    storage: StorageInfo,
    onBack: () -> Unit,
    onClearCache: () -> Unit,
    onDeleteData: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "数据与存储", onBack = onBack) }
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
            AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Text(
                        text = "总用量",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = storage.totalUsed,
                        style = MaterialTheme.typography.headlineMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    LinearProgressIndicator(
                        progress = { 0.4f },
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(8.dp)
                            .clip(RoundedCornerShape(4.dp)),
                        color = MaterialTheme.colorScheme.primary,
                        trackColor = MaterialTheme.colorScheme.surfaceVariant
                    )
                    Text(
                        text = "总容量 4.0 GB",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            AmitiaSection(title = "存储详情") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        storage.items.forEachIndexed { index, item ->
                            StorageItemRow(item = item)
                            if (index < storage.items.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            AmitiaSection(title = "缓存管理") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        SettingsRow(
                            title = "清理缓存",
                            subtitle = "清理临时文件，不影响数据",
                            leadingIcon = AmitiaIcons.CleaningServices,
                            onClick = onClearCache
                        )
                    }
                }
            }
            AmitiaSection(title = "危险操作") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Text(
                            text = "删除数据将清除所有本地存储，包括对话、记忆、角色和设置。此操作不可恢复。",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        DangerButton(
                            text = "删除所有数据",
                            onClick = onDeleteData,
                            modifier = Modifier.fillMaxWidth(),
                            leadingIcon = AmitiaIcons.DeleteForever
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun StorageItemRow(item: StorageItem) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Box(
            modifier = Modifier
                .size(32.dp)
                .clip(RoundedCornerShape(8.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = item.name.first().toString(),
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = item.name,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = item.category,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Text(
            text = item.size,
            style = MaterialTheme.typography.labelLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Preview(name = "数据与存储页 - 亮色", showBackground = true)
@Composable
private fun DataStorageScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        DataStorageScreenContent(
            storage = StorageInfo(),
            onBack = {},
            onClearCache = {},
            onDeleteData = {}
        )
    }
}

@Preview(name = "数据与存储页 - 暗色", showBackground = true)
@Composable
private fun DataStorageScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        DataStorageScreenContent(
            storage = StorageInfo(),
            onBack = {},
            onClearCache = {},
            onDeleteData = {}
        )
    }
}
