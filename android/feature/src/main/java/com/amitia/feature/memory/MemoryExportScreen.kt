package com.amitia.feature.memory

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun MemoryExportScreen(
    onBack: () -> Unit,
    onExport: () -> Unit
) {
    var characterId by remember { mutableStateOf("") }
    var types by remember {
        mutableStateOf(
            listOf(
                com.amitia.core.designsystem.component.AmitiaChipItem("长期记忆", true),
                com.amitia.core.designsystem.component.AmitiaChipItem("情景记忆", true),
                com.amitia.core.designsystem.component.AmitiaChipItem("世界书", false),
                com.amitia.core.designsystem.component.AmitiaChipItem("待确认", false)
            )
        )
    }
    var startTime by remember { mutableStateOf("") }
    var endTime by remember { mutableStateOf("") }
    var desensitize by remember { mutableStateOf(true) }
    var includeMedia by remember { mutableStateOf(false) }
    var format by remember { mutableStateOf("json") }

    MemoryExportContent(
        characterId = characterId,
        onCharacterIdChange = { characterId = it },
        types = types,
        onTypesChange = { types = it },
        startTime = startTime,
        onStartTimeChange = { startTime = it },
        endTime = endTime,
        onEndTimeChange = { endTime = it },
        desensitize = desensitize,
        onDesensitizeChange = { desensitize = it },
        includeMedia = includeMedia,
        onIncludeMediaChange = { includeMedia = it },
        format = format,
        onFormatChange = { format = it },
        onBack = onBack,
        onExport = onExport
    )
}

@Composable
fun MemoryExportContent(
    characterId: String,
    onCharacterIdChange: (String) -> Unit,
    types: List<com.amitia.core.designsystem.component.AmitiaChipItem>,
    onTypesChange: (List<com.amitia.core.designsystem.component.AmitiaChipItem>) -> Unit,
    startTime: String,
    onStartTimeChange: (String) -> Unit,
    endTime: String,
    onEndTimeChange: (String) -> Unit,
    desensitize: Boolean,
    onDesensitizeChange: (Boolean) -> Unit,
    includeMedia: Boolean,
    onIncludeMediaChange: (Boolean) -> Unit,
    format: String,
    onFormatChange: (String) -> Unit,
    onBack: () -> Unit,
    onExport: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "导出记忆", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "角色范围")
            AmitiaTextField(
                value = characterId,
                onValueChange = onCharacterIdChange,
                label = "角色ID",
                placeholder = "留空表示所有角色",
                leadingIcon = AmitiaIcons.Person
            )

            AmitiaSectionHeader(title = "记忆类型")
            com.amitia.core.designsystem.component.AmitiaChipSelector(
                items = types,
                onToggle = { index ->
                    onTypesChange(types.mapIndexed { i, item ->
                        if (i == index) item.copy(selected = !item.selected) else item
                    })
                }
            )

            AmitiaSectionHeader(title = "时间范围")
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                AmitiaTextField(
                    value = startTime,
                    onValueChange = onStartTimeChange,
                    label = "开始",
                    placeholder = "YYYY-MM-DD",
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Schedule
                )
                AmitiaTextField(
                    value = endTime,
                    onValueChange = onEndTimeChange,
                    label = "结束",
                    placeholder = "YYYY-MM-DD",
                    modifier = Modifier.weight(1f),
                    leadingIcon = AmitiaIcons.Schedule
                )
            }

            AmitiaSectionHeader(title = "导出选项")
            AmitiaSwitchRow(
                title = "脱敏处理",
                subtitle = "移除敏感个人信息",
                checked = desensitize,
                onCheckedChange = onDesensitizeChange,
                leadingIcon = AmitiaIcons.Security
            )
            AmitiaSwitchRow(
                title = "包含媒体",
                subtitle = "导出关联的图片等媒体文件",
                checked = includeMedia,
                onCheckedChange = onIncludeMediaChange,
                leadingIcon = AmitiaIcons.Image
            )

            AmitiaSectionHeader(title = "导出格式")
            AmitiaSelectionRow(
                title = "JSON",
                subtitle = "结构化数据，适合程序处理",
                selected = format == "json",
                onSelect = { onFormatChange("json") },
                leadingIcon = AmitiaIcons.Code
            )
            AmitiaSelectionRow(
                title = "CSV",
                subtitle = "表格格式，适合查看和编辑",
                selected = format == "csv",
                onSelect = { onFormatChange("csv") },
                leadingIcon = AmitiaIcons.GridView
            )
            AmitiaSelectionRow(
                title = "Markdown",
                subtitle = "可读性强的文档格式",
                selected = format == "md",
                onSelect = { onFormatChange("md") },
                leadingIcon = AmitiaIcons.TextFields
            )

            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "开始导出",
                onClick = onExport,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Download
            )
        }
    }
}

@Preview(name = "Export - Light", showBackground = true)
@Composable
private fun MemoryExportLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemoryExportContent(
            characterId = "",
            onCharacterIdChange = {},
            types = listOf(
                com.amitia.core.designsystem.component.AmitiaChipItem("长期记忆", true),
                com.amitia.core.designsystem.component.AmitiaChipItem("情景记忆", true)
            ),
            onTypesChange = {},
            startTime = "",
            onStartTimeChange = {},
            endTime = "",
            onEndTimeChange = {},
            desensitize = true,
            onDesensitizeChange = {},
            includeMedia = false,
            onIncludeMediaChange = {},
            format = "json",
            onFormatChange = {},
            onBack = {},
            onExport = {}
        )
    }
}

@Preview(name = "Export - Dark", showBackground = true)
@Composable
private fun MemoryExportDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemoryExportContent(
            characterId = "",
            onCharacterIdChange = {},
            types = emptyList(),
            onTypesChange = {},
            startTime = "",
            onStartTimeChange = {},
            endTime = "",
            onEndTimeChange = {},
            desensitize = true,
            onDesensitizeChange = {},
            includeMedia = false,
            onIncludeMediaChange = {},
            format = "csv",
            onFormatChange = {},
            onBack = {},
            onExport = {}
        )
    }
}
