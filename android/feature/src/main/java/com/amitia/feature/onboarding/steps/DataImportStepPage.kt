package com.amitia.feature.onboarding.steps

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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.TertiaryButton

private data class ImportOption(
    val title: String,
    val description: String,
    val icon: ImageVector,
    val format: String
)

private val importOptions = listOf(
    ImportOption(
        title = "导入 Amitia 备份",
        description = "从 .amitia 备份文件恢复完整数据，支持结构化映射预览。",
        icon = AmitiaIcons.Backup,
        format = ".amitia / .zip"
    ),
    ImportOption(
        title = "导入聊天记录",
        description = "从 JSON 或 CSV 文件导入历史对话，导入前可预览字段映射。",
        icon = AmitiaIcons.Chat,
        format = ".json / .csv"
    )
)

@Composable
fun DataImportStepPage(
    onSelectBackup: () -> Unit,
    onSelectChatHistory: () -> Unit,
    onSkip: () -> Unit,
    modifier: Modifier = Modifier
) {
    var selectedOption by remember { mutableStateOf(-1) }
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            Text(
                text = "数据导入",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Text(
                text = "首次进入主应用，你可以选择导入已有数据，或稍后处理。此步骤不阻塞主流程。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            importOptions.forEachIndexed { index, option ->
                ImportOptionCard(
                    option = option,
                    selected = selectedOption == index,
                    onSelect = { selectedOption = index }
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "导入选中项",
                onClick = {
                    when (selectedOption) {
                        0 -> onSelectBackup()
                        1 -> onSelectChatHistory()
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = selectedOption >= 0,
                leadingIcon = AmitiaIcons.Download
            )
            TertiaryButton(
                text = "稍后处理",
                onClick = onSkip,
                leadingIcon = AmitiaIcons.Schedule
            )
        }
    }
}

@Composable
private fun ImportOptionCard(
    option: ImportOption,
    selected: Boolean,
    onSelect: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val borderColor = if (selected) MaterialTheme.colorScheme.primary
    else MaterialTheme.colorScheme.outlineVariant
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clip(AmitiaCardShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.RadioButton,
                onClick = onSelect
            ),
        shape = AmitiaCardShape,
        color = if (selected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
        else MaterialTheme.colorScheme.surface,
        border = androidx.compose.foundation.BorderStroke(2.dp, borderColor)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = option.icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = option.title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = option.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = "支持格式：${option.format}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                )
            }
            Icon(
                imageVector = if (selected) AmitiaIcons.RadioButtonChecked
                else AmitiaIcons.RadioButtonUnchecked,
                contentDescription = null,
                tint = if (selected) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(24.dp)
            )
        }
    }
}

@Preview(name = "DataImport - Light", showBackground = true)
@Composable
private fun DataImportStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        DataImportStepPage(
            onSelectBackup = {},
            onSelectChatHistory = {},
            onSkip = {}
        )
    }
}

@Preview(name = "DataImport - Dark", showBackground = true)
@Composable
private fun DataImportStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        DataImportStepPage(
            onSelectBackup = {},
            onSelectChatHistory = {},
            onSkip = {}
        )
    }
}
