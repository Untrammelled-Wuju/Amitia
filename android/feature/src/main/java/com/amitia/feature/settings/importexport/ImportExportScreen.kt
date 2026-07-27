package com.amitia.feature.settings.importexport

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.settings.ImportExportItem
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun ImportExportScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val items = state.importExport

    ImportExportScreenContent(
        items = items,
        onBack = onBack
    )
}

@Composable
private fun ImportExportScreenContent(
    items: List<ImportExportItem>,
    onBack: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "导入导出", onBack = onBack) }
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
            AmitiaSection(title = "导出") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        items.forEachIndexed { index, item ->
                            AmitiaSwitchRow(
                                title = item.name,
                                subtitle = item.count,
                                checked = item.enabled,
                                onCheckedChange = {},
                                leadingIcon = when (item.name) {
                                    "角色" -> AmitiaIcons.Person
                                    "对话" -> AmitiaIcons.Chat
                                    "记忆" -> AmitiaIcons.Memory
                                    "世界书" -> AmitiaIcons.Book
                                    "扩展" -> AmitiaIcons.Extension
                                    else -> AmitiaIcons.Folder
                                }
                            )
                            if (index < items.lastIndex) {
                                AmitiaInsetDivider(leadingInset = 56.dp + AmitiaSpacing.Base)
                            }
                        }
                    }
                }
                PrimaryButton(
                    text = "导出选中数据",
                    onClick = {},
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Download
                )
            }
            AmitiaSection(title = "导入") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Text(
                            text = "选择文件导入数据，支持 JSON 和 ZIP 格式",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        SecondaryButton(
                            text = "选择文件导入",
                            onClick = {},
                            modifier = Modifier.fillMaxWidth(),
                            leadingIcon = AmitiaIcons.Upload
                        )
                    }
                }
            }
            AmitiaSection(title = "完整数据包") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        Text(
                            text = "导出包含所有数据的完整备份包，可用于迁移或归档",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        PrimaryButton(
                            text = "导出完整数据包",
                            onClick = {},
                            modifier = Modifier.fillMaxWidth(),
                            leadingIcon = AmitiaIcons.Archive
                        )
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "导入导出页 - 亮色", showBackground = true)
@Composable
private fun ImportExportScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ImportExportScreenContent(
            items = listOf(
                ImportExportItem("角色", "12 个", true),
                ImportExportItem("对话", "1,248 条", true),
                ImportExportItem("记忆", "3,562 条", true)
            ),
            onBack = {}
        )
    }
}

@Preview(name = "导入导出页 - 暗色", showBackground = true)
@Composable
private fun ImportExportScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ImportExportScreenContent(
            items = listOf(
                ImportExportItem("角色", "12 个", true),
                ImportExportItem("对话", "1,248 条", false)
            ),
            onBack = {}
        )
    }
}
