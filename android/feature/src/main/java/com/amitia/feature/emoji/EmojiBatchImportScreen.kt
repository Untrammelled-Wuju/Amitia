package com.amitia.feature.emoji

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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EmojiBatchImportScreen(
    onBack: () -> Unit,
    onImport: () -> Unit
) {
    var meanings by remember { mutableStateOf(mapOf<String, String>()) }
    var targetGroup by remember { mutableStateOf("日常表情") }
    var groups by remember {
        mutableStateOf(
            listOf(
                AmitiaChipItem("日常表情", true),
                AmitiaChipItem("情绪表达", false),
                AmitiaChipItem("新建分组", false)
            )
        )
    }
    val previewItems = remember {
        listOf(
            EmojiImportItem("1", "", EmojiImportStatus.Pending),
            EmojiImportItem("2", "", EmojiImportStatus.Pending),
            EmojiImportItem("3", "", EmojiImportStatus.Duplicate, duplicateOf = "1"),
            EmojiImportItem("4", "", EmojiImportStatus.Pending),
            EmojiImportItem("5", "", EmojiImportStatus.Pending)
        )
    }

    EmojiBatchImportContent(
        previewItems = previewItems,
        meanings = meanings,
        onMeaningChange = { id, meaning ->
            meanings = meanings + (id to meaning)
        },
        groups = groups,
        onGroupToggle = { index ->
            groups = groups.mapIndexed { i, item ->
                if (i == index) item.copy(selected = true) else item.copy(selected = false)
            }
            targetGroup = groups[index].label
        },
        onBack = onBack,
        onImport = onImport
    )
}

@Composable
fun EmojiBatchImportContent(
    previewItems: List<EmojiImportItem>,
    meanings: Map<String, String>,
    onMeaningChange: (String, String) -> Unit,
    groups: List<AmitiaChipItem>,
    onGroupToggle: (Int) -> Unit,
    onBack: () -> Unit,
    onImport: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "批量导入", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Row(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                ) {
                    Box(
                        modifier = Modifier
                            .size(40.dp)
                            .clip(RoundedCornerShape(10.dp))
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.CloudUpload,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "已选 ${previewItems.size} 张图片",
                            style = MaterialTheme.typography.titleSmall,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Text(
                            text = "重复 ${previewItems.count { it.status == EmojiImportStatus.Duplicate }} 张",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.tertiary
                        )
                    }
                    val interactionSource = remember { MutableInteractionSource() }
                    Text(
                        text = "重新选择",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier
                            .clip(RoundedCornerShape(20.dp))
                            .clickable(
                                interactionSource = interactionSource,
                                indication = null,
                                role = Role.Button,
                                onClick = {}
                            )
                            .padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp)
                    )
                }
            }

            AmitiaSectionHeader(title = "目标分组")
            AmitiaChipSelector(
                items = groups,
                onToggle = onGroupToggle,
                multiSelect = false
            )

            AmitiaSectionHeader(title = "填写含义（可选）")
            Text(
                text = "可跳过含义，导入后进入待完善状态",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            previewItems.forEach { item ->
                ImportPreviewRow(
                    item = item,
                    meaning = meanings[item.id] ?: "",
                    onMeaningChange = { onMeaningChange(item.id, it) }
                )
            }

            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "导入 ${previewItems.count { it.status == EmojiImportStatus.Pending }} 张",
                onClick = onImport,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Download
            )
        }
    }
}

@Composable
private fun ImportPreviewRow(
    item: EmojiImportItem,
    meaning: String,
    onMeaningChange: (String) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(36.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(MaterialTheme.colorScheme.surfaceVariant),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Image,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Text(
                    text = "图片 ${item.id}",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f)
                )
                if (item.status == EmojiImportStatus.Duplicate) {
                    Surface(
                        shape = RoundedCornerShape(6.dp),
                        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.5f)
                    ) {
                        Text(
                            text = "重复",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer,
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                        )
                    }
                }
            }
            if (item.status != EmojiImportStatus.Duplicate) {
                AmitiaTextField(
                    value = meaning,
                    onValueChange = onMeaningChange,
                    placeholder = "输入表情含义（可选）",
                    singleLine = true
                )
            }
        }
    }
}

@Preview(name = "Batch Import - Light", showBackground = true)
@Composable
private fun EmojiBatchImportLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiBatchImportContent(
            previewItems = listOf(
                EmojiImportItem("1", "", EmojiImportStatus.Pending),
                EmojiImportItem("2", "", EmojiImportStatus.Duplicate, duplicateOf = "1"),
                EmojiImportItem("3", "", EmojiImportStatus.Pending)
            ),
            meanings = mapOf("1" to "开心"),
            onMeaningChange = { _, _ -> },
            groups = listOf(
                AmitiaChipItem("日常表情", true),
                AmitiaChipItem("情绪表达", false)
            ),
            onGroupToggle = {},
            onBack = {},
            onImport = {}
        )
    }
}

@Preview(name = "Batch Import - Dark", showBackground = true)
@Composable
private fun EmojiBatchImportDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiBatchImportContent(
            previewItems = emptyList(),
            meanings = emptyMap(),
            onMeaningChange = { _, _ -> },
            groups = listOf(AmitiaChipItem("日常表情", true)),
            onGroupToggle = {},
            onBack = {},
            onImport = {}
        )
    }
}
