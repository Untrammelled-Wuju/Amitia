package com.amitia.feature.character.detail

import androidx.compose.animation.AnimatedVisibility
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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ExpandLess
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.PersonalityData
import com.amitia.feature.character.model.PersonalityGroup
import com.amitia.feature.character.model.PersonalityPreset

@Composable
fun CharacterPersonalityTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.personalityState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadPersonality() }
        )
        is ScreenState.Content -> PersonalityContent(
            groups = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun PersonalityContent(
    groups: List<PersonalityGroup>,
    modifier: Modifier = Modifier
) {
    val dimensionValues = remember {
        mutableStateMapOf<String, Float>().apply {
            PersonalityData.defaultDimensions().forEach { put(it.id, it.value) }
        }
    }
    val expandedGroups = remember { mutableStateMapOf<String, Boolean>() }
    var selectedPreset by remember { mutableStateOf<String?>(null) }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "summary") {
            PersonalitySummaryCard(groups, dimensionValues)
        }
        item(key = "presets") {
            PresetSelector(
                selectedPreset = selectedPreset,
                onSelect = { presetId ->
                    selectedPreset = presetId
                    applyPreset(presetId, dimensionValues)
                }
            )
        }
        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "恢复默认",
                    onClick = {
                        PersonalityData.defaultDimensions().forEach {
                            dimensionValues[it.id] = 0.5f
                        }
                        selectedPreset = null
                    },
                    leadingIcon = Icons.Outlined.Refresh,
                    modifier = Modifier.weight(1f)
                )
            }
        }
        items(groups, key = { it.id }) { group ->
            PersonalityGroupCard(
                group = group,
                isExpanded = expandedGroups[group.id] ?: true,
                onToggle = { expandedGroups[group.id] = !(expandedGroups[group.id] ?: true) },
                values = dimensionValues,
                onValueChange = { dimId, value -> dimensionValues[dimId] = value }
            )
        }
        item(key = "spacer") { Spacer(modifier = Modifier.height(8.dp)) }
    }
}

@Composable
private fun PersonalitySummaryCard(
    groups: List<PersonalityGroup>,
    values: Map<String, Float>
) {
    val totalDimensions = groups.flatMap { it.dimensions }.size
    val customCount = values.count { (_, v) -> v != 0.5f }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "性格摘要",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                SummaryItem(
                    label = "总维度",
                    value = "$totalDimensions"
                )
                SummaryItem(
                    label = "已自定义",
                    value = "$customCount"
                )
                SummaryItem(
                    label = "分组数",
                    value = "${groups.size}"
                )
            }
        }
    }
}

@Composable
private fun SummaryItem(label: String, value: String) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
        )
    }
}

@Composable
private fun PresetSelector(
    selectedPreset: String?,
    onSelect: (String) -> Unit
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            text = "预设模板",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        PersonalityData.presets.forEach { preset ->
            PresetItem(
                preset = preset,
                selected = selectedPreset == preset.id,
                onClick = { onSelect(preset.id) }
            )
        }
    }
}

@Composable
private fun PresetItem(
    preset: PersonalityPreset,
    selected: Boolean,
    onClick: () -> Unit
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(20.dp)
                    .clip(CircleShape)
                    .background(
                        if (selected) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                if (selected) {
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.onPrimary)
                    )
                }
            }
            Column {
                Text(
                    text = preset.name,
                    style = MaterialTheme.typography.bodyLarge,
                    color = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = preset.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (selected) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun PersonalityGroupCard(
    group: PersonalityGroup,
    isExpanded: Boolean,
    onToggle: () -> Unit,
    values: Map<String, Float>,
    onValueChange: (String, Float) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(onClick = onToggle),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = group.title,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = group.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = "${group.dimensions.size} 项",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Icon(
                    imageVector = if (isExpanded) Icons.Outlined.ExpandLess
                    else Icons.Outlined.ExpandMore,
                    contentDescription = if (isExpanded) "收起" else "展开",
                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            AnimatedVisibility(visible = isExpanded) {
                Column(
                    modifier = Modifier.padding(top = 12.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    group.dimensions.forEach { dimension ->
                        AmitiaSlider(
                            value = values[dimension.id] ?: dimension.value,
                            onValueChange = { onValueChange(dimension.id, it) },
                            label = dimension.label,
                            valueRange = 0f..1f,
                            valueFormatter = {
                                val pct = (it * 100).toInt()
                                "${dimension.leftLabel} $pct% ${dimension.rightLabel}"
                            }
                        )
                        Text(
                            text = dimension.description,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                        )
                    }
                }
            }
        }
    }
}

private fun applyPreset(
    presetId: String,
    values: androidx.compose.runtime.snapshots.SnapshotStateMap<String, Float>
) {
    val config = when (presetId) {
        "balanced" -> 0.5f
        "gentle" -> 0.7f
        "energetic" -> 0.8f
        "calm" -> 0.3f
        "playful" -> 0.75f
        else -> 0.5f
    }
    values.keys.forEach { key -> values[key] = config }
}

@Preview(name = "Personality - Light", showBackground = true)
@Composable
private fun CharacterPersonalityLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            PersonalityContent(groups = PersonalityData.groups)
        }
    }
}

@Preview(name = "Personality - Dark", showBackground = true)
@Composable
private fun CharacterPersonalityDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            PersonalityContent(groups = PersonalityData.groups)
        }
    }
}
