package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.PrimaryButton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ExportScreen(
    onBack: () -> Unit,
    onExport: () -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val config by viewModel.exportConfig.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "导出对话", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            item(key = "format_section") {
                AmitiaSection(title = "导出格式") {
                    Column {
                        ExportFormatOption(
                            label = ExportFormat.Json.label,
                            description = "适合程序处理和数据分析",
                            icon = AmitiaIcons.Code,
                            selected = config.format == ExportFormat.Json,
                            onClick = { viewModel.updateExportConfig { it.copy(format = ExportFormat.Json) } }
                        )
                        ExportFormatOption(
                            label = ExportFormat.Markdown.label,
                            description = "适合查看和版本控制",
                            icon = AmitiaIcons.TextFields,
                            selected = config.format == ExportFormat.Markdown,
                            onClick = { viewModel.updateExportConfig { it.copy(format = ExportFormat.Markdown) } }
                        )
                        ExportFormatOption(
                            label = ExportFormat.Document.label,
                            description = "适合打印和分享",
                            icon = AmitiaIcons.MenuBook,
                            selected = config.format == ExportFormat.Document,
                            onClick = { viewModel.updateExportConfig { it.copy(format = ExportFormat.Document) } }
                        )
                    }
                }
            }
            item(key = "options_section") {
                AmitiaSection(title = "导出选项") {
                    AmitiaContentSurface {
                        Column {
                            ExportToggleRow(
                                title = "包含媒体文件",
                                subtitle = "导出对话中的图片、语音等附件",
                                checked = config.includeMedia,
                                onToggle = { viewModel.updateExportConfig { it.copy(includeMedia = !it.includeMedia) } }
                            )
                            ExportToggleRow(
                                title = "包含工具执行记录",
                                subtitle = "导出工具调用过程和结果",
                                checked = config.includeToolRecords,
                                onToggle = { viewModel.updateExportConfig { it.copy(includeToolRecords = !it.includeToolRecords) } }
                            )
                            ExportToggleRow(
                                title = "隐私脱敏",
                                subtitle = "对敏感信息进行脱敏处理",
                                checked = config.privacyMask,
                                onToggle = { viewModel.updateExportConfig { it.copy(privacyMask = !it.privacyMask) } }
                            )
                        }
                    }
                }
            }
            item(key = "privacy_warning") {
                if (config.privacyMask) {
                    AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                        Row(
                            modifier = Modifier.padding(AmitiaSpacing.Base),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Shield,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.primary,
                                modifier = Modifier.size(AmitiaIconSize.Medium)
                            )
                            Text(
                                text = "已启用隐私脱敏，导出的内容将隐藏敏感信息",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
            }
            item(key = "export_button") {
                PrimaryButton(
                    text = "开始导出",
                    onClick = onExport,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Download
                )
            }
        }
    }
}

@Composable
private fun ExportFormatOption(
    label: String,
    description: String,
    icon: ImageVector,
    selected: Boolean,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick).padding(AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier.size(AmitiaIconSize.Large).clip(CircleShape)
                .background(if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = if (selected) MaterialTheme.colorScheme.onPrimaryContainer else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = label,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = description,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        RadioButton(
            selected = selected,
            onClick = onClick,
            colors = RadioButtonDefaults.colors(selectedColor = MaterialTheme.colorScheme.primary)
        )
    }
}

@Composable
private fun ExportToggleRow(
    title: String,
    subtitle: String,
    checked: Boolean,
    onToggle: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
        Switch(
            checked = checked,
            onCheckedChange = { onToggle() },
            colors = SwitchDefaults.colors(
                checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                checkedTrackColor = MaterialTheme.colorScheme.primary
            )
        )
    }
}

@Preview(name = "Export - Light", showBackground = true)
@Composable
private fun ExportLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                ExportFormatOption(
                    label = "Markdown",
                    description = "适合查看和版本控制",
                    icon = AmitiaIcons.TextFields,
                    selected = true,
                    onClick = {}
                )
                ExportToggleRow(
                    title = "包含媒体文件",
                    subtitle = "导出对话中的图片、语音等附件",
                    checked = true,
                    onToggle = {}
                )
            }
        }
    }
}

@Preview(name = "Export - Dark", showBackground = true)
@Composable
private fun ExportDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            ExportFormatOption(
                label = "结构化 JSON",
                description = "适合程序处理和数据分析",
                icon = AmitiaIcons.Code,
                selected = false,
                onClick = {}
            )
        }
    }
}
