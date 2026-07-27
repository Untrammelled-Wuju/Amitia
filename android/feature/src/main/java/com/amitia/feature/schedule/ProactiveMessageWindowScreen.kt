package com.amitia.feature.schedule

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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ProactiveMessageWindowScreen(
    onBack: () -> Unit,
    viewModel: ProactiveWindowViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    ProactiveMessageWindowContent(
        state = state,
        onBack = onBack,
        onUpdate = viewModel::update,
        onRetry = viewModel::load
    )
}

@Composable
fun ProactiveMessageWindowContent(
    state: ScreenState<ProactiveWindowData>,
    onBack: () -> Unit,
    onUpdate: (((ProactiveMessageWindow) -> ProactiveMessageWindow)) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "主动消息时间窗", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.AutoAwesome,
                title = "暂无配置",
                description = "加载完成后可在此配置主动消息时间窗",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> WindowBody(data = state.data, onUpdate = onUpdate)
            is ScreenState.Partial -> WindowBody(data = state.data, onUpdate = onUpdate)
        }
    }
}

@Composable
private fun WindowBody(
    data: ProactiveWindowData,
    onUpdate: (((ProactiveMessageWindow) -> ProactiveMessageWindow)) -> Unit
) {
    val w = data.window
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Base),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        NoonWeightCard(enabled = w.noonWeightHint)
        AmitiaSectionHeader(title = "每日有效时间")
        AmitiaSwitchRow(
            title = "启用每日主动消息",
            checked = w.dailyEnabled,
            onCheckedChange = { v -> onUpdate { it.copy(dailyEnabled = v) } },
            subtitle = "在有效时间范围内允许角色主动发起消息",
            leadingIcon = AmitiaIcons.AutoAwesome
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                AmitiaTextField(
                    value = w.startTime,
                    onValueChange = { v -> onUpdate { it.copy(startTime = v) } },
                    label = "开始时间",
                    placeholder = "10:00",
                    leadingIcon = AmitiaIcons.Schedule
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                AmitiaTextField(
                    value = w.endTime,
                    onValueChange = { v -> onUpdate { it.copy(endTime = v) } },
                    label = "结束时间",
                    placeholder = "03:00",
                    leadingIcon = AmitiaIcons.Schedule
                )
            }
        }
        RangeHintCard()
        AmitiaSectionHeader(title = "频率控制")
        AmitiaSlider(
            value = w.frequencyPerDay.toFloat(),
            onValueChange = { v -> onUpdate { it.copy(frequencyPerDay = v.toInt()) } },
            label = "每日频次",
            valueFormatter = { "${it.toInt()} 次/天" },
            valueRange = 1f..20f,
            steps = 18
        )
        AmitiaNumberField(
            value = w.minIntervalMinutes.toString(),
            onValueChange = { v -> v.toIntOrNull()?.let { n -> onUpdate { it.copy(minIntervalMinutes = n) } } },
            label = "最小间隔",
            placeholder = "90",
            unit = "分钟",
            onIncrement = { onUpdate { it.copy(minIntervalMinutes = (it.minIntervalMinutes + 10).coerceAtMost(240)) } },
            onDecrement = { onUpdate { it.copy(minIntervalMinutes = (it.minIntervalMinutes - 10).coerceAtLeast(10)) } }
        )
        AmitiaSectionHeader(title = "安静时段")
        AmitiaSwitchRow(
            title = "遵循安静时段",
            checked = w.quietHoursEnabled,
            onCheckedChange = { v -> onUpdate { it.copy(quietHoursEnabled = v) } },
            subtitle = "在安静时段内暂停主动消息",
            leadingIcon = AmitiaIcons.NotificationsOff
        )
        AmitiaSectionHeader(title = "渠道策略")
        listOf(
            "优先 Web，降级微信",
            "优先微信，降级 QQ",
            "所有渠道并发",
            "仅 Web"
        ).forEach { strategy ->
            AmitiaSelectionRow(
                title = strategy,
                selected = w.channelStrategy == strategy,
                onSelect = { onUpdate { it.copy(channelStrategy = strategy) } },
                leadingIcon = AmitiaIcons.Hub
            )
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        PrimaryButton(
            text = "保存配置",
            onClick = {},
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Check
        )
    }
}

@Composable
private fun NoonWeightCard(enabled: Boolean) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = if (enabled) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
        else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Lightbulb,
                contentDescription = null,
                tint = if (enabled) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(20.dp)
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "12:00 前权重提示",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = if (enabled) "已启用：上午时段提高主动消息权重"
                    else "未启用：建议在 12:00 前增加主动消息机会",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun RangeHintCard() {
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
                imageVector = AmitiaIcons.Info,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.tertiary,
                modifier = Modifier.size(18.dp)
            )
            Text(
                text = "支持跨天配置，例如 10:00 至次日 03:00",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onTertiaryContainer
            )
        }
    }
}

@Preview(name = "ProactiveWindow - Light", showBackground = true)
@Composable
private fun ProactiveWindowLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ProactiveMessageWindowContent(
            state = ScreenState.Content(ProactiveWindowData(ScheduleMockData.proactiveWindow)),
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}

@Preview(name = "ProactiveWindow - Dark", showBackground = true)
@Composable
private fun ProactiveWindowDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ProactiveMessageWindowContent(
            state = ScreenState.Loading,
            onBack = {}, onUpdate = {}, onRetry = {}
        )
    }
}
