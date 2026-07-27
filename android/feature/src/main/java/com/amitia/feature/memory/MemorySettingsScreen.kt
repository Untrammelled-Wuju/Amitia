package com.amitia.feature.memory

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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.DangerButton

@Composable
fun MemorySettingsScreen(
    onBack: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val config by viewModel.settingsState.collectAsStateWithLifecycle()
    MemorySettingsContent(
        config = config,
        onAutoWriteChange = { viewModel.updateSettings { it.copy(autoWrite = !it.autoWrite) } },
        onRequireConfirmChange = { viewModel.updateSettings { it.copy(requireConfirm = !it.requireConfirm) } },
        onMergeStrategyChange = { strategy -> viewModel.updateSettings { it.copy(mergeStrategy = strategy) } },
        onImportanceThresholdChange = { v -> viewModel.updateSettings { it.copy(importanceThreshold = v) } },
        onVectorRetrieveCountChange = { v -> viewModel.updateSettings { it.copy(vectorRetrieveCount = v) } },
        onTimeDecayChange = { viewModel.updateSettings { it.copy(timeDecay = !it.timeDecay) } },
        onGraphSyncChange = { viewModel.updateSettings { it.copy(graphSync = !it.graphSync) } },
        onBack = onBack
    )
}

@Composable
fun MemorySettingsContent(
    config: MemorySettingsConfig,
    onAutoWriteChange: () -> Unit,
    onRequireConfirmChange: () -> Unit,
    onMergeStrategyChange: (String) -> Unit,
    onImportanceThresholdChange: (Float) -> Unit,
    onVectorRetrieveCountChange: (Int) -> Unit,
    onTimeDecayChange: () -> Unit,
    onGraphSyncChange: () -> Unit,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "记忆设置", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "写入策略")
            AmitiaSwitchRow(
                title = "自动写入",
                subtitle = "对话中自动提取并保存记忆",
                checked = config.autoWrite,
                onCheckedChange = { onAutoWriteChange() },
                leadingIcon = AmitiaIcons.Sync
            )
            AmitiaSwitchRow(
                title = "需要用户确认",
                subtitle = "AI建议的记忆需用户确认后才写入",
                checked = config.requireConfirm,
                onCheckedChange = { onRequireConfirmChange() },
                leadingIcon = AmitiaIcons.Help
            )

            AmitiaSectionHeader(title = "合并策略")
            AmitiaSelectionRow(
                title = "智能合并",
                subtitle = "自动判断并合并相似记忆",
                selected = config.mergeStrategy == "智能合并",
                onSelect = { onMergeStrategyChange("智能合并") },
                leadingIcon = AmitiaIcons.Psychology
            )
            AmitiaSelectionRow(
                title = "新值优先",
                subtitle = "冲突时使用最新的值",
                selected = config.mergeStrategy == "新值优先",
                onSelect = { onMergeStrategyChange("新值优先") },
                leadingIcon = AmitiaIcons.Schedule
            )
            AmitiaSelectionRow(
                title = "保留旧值",
                subtitle = "冲突时保留已有值",
                selected = config.mergeStrategy == "保留旧值",
                onSelect = { onMergeStrategyChange("保留旧值") },
                leadingIcon = AmitiaIcons.History
            )

            AmitiaSectionHeader(title = "检索配置")
            AmitiaSlider(
                value = config.importanceThreshold,
                onValueChange = onImportanceThresholdChange,
                label = "重要度阈值",
                valueFormatter = { String.format("%.1f", it) },
                valueRange = 0f..1f
            )
            AmitiaNumberField(
                value = config.vectorRetrieveCount.toString(),
                onValueChange = { v -> v.toIntOrNull()?.let { onVectorRetrieveCountChange(it) } },
                label = "向量检索数量",
                placeholder = "5",
                onIncrement = { onVectorRetrieveCountChange((config.vectorRetrieveCount + 1).coerceAtMost(20)) },
                onDecrement = { onVectorRetrieveCountChange((config.vectorRetrieveCount - 1).coerceAtLeast(1)) }
            )

            AmitiaSectionHeader(title = "高级设置")
            AmitiaSwitchRow(
                title = "时间衰减",
                subtitle = "旧记忆的重要度随时间降低",
                checked = config.timeDecay,
                onCheckedChange = { onTimeDecayChange() },
                leadingIcon = AmitiaIcons.Schedule
            )
            AmitiaSwitchRow(
                title = "图谱同步",
                subtitle = "记忆变化时自动更新记忆图谱",
                checked = config.graphSync,
                onCheckedChange = { onGraphSyncChange() },
                leadingIcon = AmitiaIcons.Hub
            )

            AmitiaSectionHeader(title = "数据管理")
            DangerButton(
                text = "清理无效记忆",
                onClick = {},
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Delete
            )
            DangerButton(
                text = "清空所有记忆",
                onClick = {},
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.DeleteForever
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        }
    }
}

@Preview(name = "Settings - Light", showBackground = true)
@Composable
private fun MemorySettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemorySettingsContent(
            config = MemorySettingsConfig(),
            onAutoWriteChange = {},
            onRequireConfirmChange = {},
            onMergeStrategyChange = {},
            onImportanceThresholdChange = {},
            onVectorRetrieveCountChange = {},
            onTimeDecayChange = {},
            onGraphSyncChange = {},
            onBack = {}
        )
    }
}

@Preview(name = "Settings - Dark", showBackground = true)
@Composable
private fun MemorySettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemorySettingsContent(
            config = MemorySettingsConfig(
                autoWrite = false,
                requireConfirm = true,
                mergeStrategy = "新值优先",
                importanceThreshold = 0.5f,
                vectorRetrieveCount = 10,
                timeDecay = false,
                graphSync = false
            ),
            onAutoWriteChange = {},
            onRequireConfirmChange = {},
            onMergeStrategyChange = {},
            onImportanceThresholdChange = {},
            onVectorRetrieveCountChange = {},
            onTimeDecayChange = {},
            onGraphSyncChange = {},
            onBack = {}
        )
    }
}
