package com.amitia.feature.memory

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
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
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ImportFieldMappingScreen(
    onBack: () -> Unit,
    onImport: () -> Unit
) {
    val availableFields = remember {
        listOf("user_name", "char_name", "timestamp", "content", "media_url", "session_id", "source_type", "role")
    }
    val targetFields = remember {
        listOf("用户", "角色", "时间", "消息内容", "媒体", "会话", "来源")
    }
    var mappings by remember {
        mutableStateOf(
            targetFields.mapIndexed { index, target ->
                FieldMappingItem(
                    targetField = target,
                    sourceField = availableFields.getOrNull(index),
                    availableSources = availableFields
                )
            }
        )
    }

    ImportFieldMappingContent(
        mappings = mappings,
        onMappingChange = { index, newSource ->
            mappings = mappings.mapIndexed { i, m ->
                if (i == index) m.copy(sourceField = newSource) else m
            }
        },
        onBack = onBack,
        onImport = onImport
    )
}

@Composable
fun ImportFieldMappingContent(
    mappings: List<FieldMappingItem>,
    onMappingChange: (Int, String?) -> Unit,
    onBack: () -> Unit,
    onImport: () -> Unit
) {
    val hasUnmapped = mappings.any { it.sourceField == null }
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "字段映射", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Text(
                text = "将源文件字段映射到目标字段，无法映射的字段将被忽略",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            if (hasUnmapped) {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                    color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                ) {
                    Row(
                        modifier = Modifier.padding(AmitiaSpacing.Base),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.WarningAmber,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.tertiary,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                        Text(
                            text = "部分字段未映射，对应数据将不会导入",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer
                        )
                    }
                }
            }
            mappings.forEachIndexed { index, mapping ->
                MappingRow(
                    mapping = mapping,
                    onChange = { newSource -> onMappingChange(index, newSource) }
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "开始导入",
                onClick = onImport,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Download
            )
        }
    }
}

@Composable
private fun MappingRow(
    mapping: FieldMappingItem,
    onChange: (String?) -> Unit
) {
    var expanded by remember { mutableStateOf(false) }
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = mapping.targetField,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = mapping.sourceField ?: "未映射",
                    style = MaterialTheme.typography.bodySmall,
                    color = if (mapping.sourceField == null) MaterialTheme.colorScheme.tertiary
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Surface(
                modifier = Modifier
                    .clip(RoundedCornerShape(8.dp))
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = { expanded = !expanded }
                    ),
                shape = RoundedCornerShape(8.dp),
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Text(
                        text = "选择",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Icon(
                        imageVector = if (expanded) AmitiaIcons.ExpandLess else AmitiaIcons.ExpandMore,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                }
            }
        }
    }
    if (expanded) {
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            color = MaterialTheme.colorScheme.surface
        ) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Sm)) {
                mapping.availableSources.forEach { source ->
                    val selected = source == mapping.sourceField
                    val rowInteraction = remember { MutableInteractionSource() }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(8.dp))
                            .clickable(
                                interactionSource = rowInteraction,
                                indication = null,
                                role = Role.RadioButton,
                                onClick = {
                                    onChange(source)
                                    expanded = false
                                }
                            )
                            .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = if (selected) AmitiaIcons.RadioButtonChecked else AmitiaIcons.RadioButtonUnchecked,
                            contentDescription = null,
                            tint = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                        Text(
                            text = source,
                            style = MaterialTheme.typography.bodySmall,
                            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
                val clearInteraction = remember { MutableInteractionSource() }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(8.dp))
                        .clickable(
                            interactionSource = clearInteraction,
                            indication = null,
                            role = Role.Button,
                            onClick = {
                                onChange(null)
                                expanded = false
                            }
                        )
                        .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Close,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                    Text(
                        text = "清除映射",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Preview(name = "Mapping - Light", showBackground = true)
@Composable
private fun ImportFieldMappingLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ImportFieldMappingContent(
            mappings = listOf(
                FieldMappingItem("用户", "user_name", listOf("user_name", "char_name", "timestamp")),
                FieldMappingItem("角色", "char_name", listOf("user_name", "char_name", "timestamp")),
                FieldMappingItem("时间", "timestamp", listOf("user_name", "char_name", "timestamp")),
                FieldMappingItem("消息内容", null, listOf("user_name", "char_name", "timestamp"))
            ),
            onMappingChange = { _, _ -> },
            onBack = {},
            onImport = {}
        )
    }
}

@Preview(name = "Mapping - Dark", showBackground = true)
@Composable
private fun ImportFieldMappingDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ImportFieldMappingContent(
            mappings = listOf(
                FieldMappingItem("用户", "user_name", listOf("user_name")),
                FieldMappingItem("角色", null, listOf("user_name"))
            ),
            onMappingChange = { _, _ -> },
            onBack = {},
            onImport = {}
        )
    }
}
