package com.amitia.feature.emoji

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
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EmojiSendStrategyScreen(
    onBack: () -> Unit,
    onSave: () -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val strategy by viewModel.sendStrategyState.collectAsStateWithLifecycle()
    EmojiSendStrategyContent(
        strategy = strategy,
        onAiSendEnabledChange = { viewModel.updateSendStrategy { it.copy(aiSendEnabled = !it.aiSendEnabled) } },
        onRandomProbabilityChange = { v -> viewModel.updateSendStrategy { it.copy(randomProbability = v) } },
        onEndOnlyChange = { viewModel.updateSendStrategy { it.copy(endOnly = !it.endOnly) } },
        onAllowInterleaveChange = { viewModel.updateSendStrategy { it.copy(allowInterleave = !it.allowInterleave) } },
        onDefaultReplyInterleaveChange = { viewModel.updateSendStrategy { it.copy(defaultReplyInterleave = !it.defaultReplyInterleave) } },
        onCooldownChange = { v -> viewModel.updateSendStrategy { it.copy(cooldownSeconds = v) } },
        onMaxPerTurnChange = { v -> viewModel.updateSendStrategy { it.copy(maxPerTurn = v) } },
        onBack = onBack,
        onSave = onSave
    )
}

@Composable
fun EmojiSendStrategyContent(
    strategy: EmojiSendStrategy,
    onAiSendEnabledChange: () -> Unit,
    onRandomProbabilityChange: (Float) -> Unit,
    onEndOnlyChange: () -> Unit,
    onAllowInterleaveChange: () -> Unit,
    onDefaultReplyInterleaveChange: () -> Unit,
    onCooldownChange: (Int) -> Unit,
    onMaxPerTurnChange: (Int) -> Unit,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "发送策略", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "AI发送控制")
            AmitiaSwitchRow(
                title = "允许AI发送表情包",
                subtitle = "控制AI是否可以在回复中发送表情",
                checked = strategy.aiSendEnabled,
                onCheckedChange = { onAiSendEnabledChange() },
                leadingIcon = AmitiaIcons.SmartToy
            )

            if (strategy.aiSendEnabled) {
                AmitiaSlider(
                    value = strategy.randomProbability,
                    onValueChange = onRandomProbabilityChange,
                    label = "随机发送概率",
                    valueFormatter = { "${(it * 100).toInt()}%" },
                    valueRange = 0f..0.5f
                )
                AmitiaSwitchRow(
                    title = "仅在回复末尾发送",
                    subtitle = "优先将表情放在最后",
                    checked = strategy.endOnly,
                    onCheckedChange = { onEndOnlyChange() },
                    leadingIcon = AmitiaIcons.AlignVerticalBottom
                )
                AmitiaSwitchRow(
                    title = "允许多消息穿插",
                    subtitle = "在多条消息中随机穿插表情",
                    checked = strategy.allowInterleave,
                    onCheckedChange = { onAllowInterleaveChange() },
                    leadingIcon = AmitiaIcons.ViewStream
                )
                if (strategy.allowInterleave) {
                    AmitiaSwitchRow(
                        title = "默认回复也穿插",
                        subtitle = "普通回复也低概率穿插表情",
                        checked = strategy.defaultReplyInterleave,
                        onCheckedChange = { onDefaultReplyInterleaveChange() },
                        leadingIcon = AmitiaIcons.AutoAwesome
                    )
                }

                AmitiaSectionHeader(title = "频率控制")
                AmitiaNumberField(
                    value = strategy.cooldownSeconds.toString(),
                    onValueChange = { v -> v.toIntOrNull()?.let { onCooldownChange(it.coerceIn(0, 600)) } },
                    label = "冷却时间",
                    placeholder = "30",
                    unit = "秒",
                    onIncrement = { onCooldownChange((strategy.cooldownSeconds + 5).coerceAtMost(600)) },
                    onDecrement = { onCooldownChange((strategy.cooldownSeconds - 5).coerceAtLeast(0)) }
                )
                AmitiaNumberField(
                    value = strategy.maxPerTurn.toString(),
                    onValueChange = { v -> v.toIntOrNull()?.let { onMaxPerTurnChange(it.coerceIn(1, 10)) } },
                    label = "每轮最多数量",
                    placeholder = "1",
                    unit = "个",
                    onIncrement = { onMaxPerTurnChange((strategy.maxPerTurn + 1).coerceAtMost(10)) },
                    onDecrement = { onMaxPerTurnChange((strategy.maxPerTurn - 1).coerceAtLeast(1)) }
                )

                AmitiaSectionHeader(title = "默认策略说明")
                Text(
                    text = "不保证每条消息发送表情；一般优先放在最后；允许多消息低概率穿插；必须设置冷却避免刷屏",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
                )
            }

            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "保存策略",
                onClick = onSave,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Preview(name = "Send Strategy - Light", showBackground = true)
@Composable
private fun EmojiSendStrategyLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiSendStrategyContent(
            strategy = EmojiSendStrategy(
                aiSendEnabled = true,
                randomProbability = 0.15f,
                endOnly = true,
                allowInterleave = true,
                defaultReplyInterleave = false,
                cooldownSeconds = 30,
                maxPerTurn = 2
            ),
            onAiSendEnabledChange = {},
            onRandomProbabilityChange = {},
            onEndOnlyChange = {},
            onAllowInterleaveChange = {},
            onDefaultReplyInterleaveChange = {},
            onCooldownChange = {},
            onMaxPerTurnChange = {},
            onBack = {},
            onSave = {}
        )
    }
}

@Preview(name = "Send Strategy - Dark", showBackground = true)
@Composable
private fun EmojiSendStrategyDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiSendStrategyContent(
            strategy = EmojiSendStrategy(aiSendEnabled = false),
            onAiSendEnabledChange = {},
            onRandomProbabilityChange = {},
            onEndOnlyChange = {},
            onAllowInterleaveChange = {},
            onDefaultReplyInterleaveChange = {},
            onCooldownChange = {},
            onMaxPerTurnChange = {},
            onBack = {},
            onSave = {}
        )
    }
}
