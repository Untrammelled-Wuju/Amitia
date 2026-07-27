package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun SetupSummaryStepPage(
    state: OnboardingFlowUiState,
    onContinue: () -> Unit,
    modifier: Modifier = Modifier
) {
    val character = state.character
    val personalityText = character.customPersonality.ifBlank { character.personality }
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
            verticalArrangement = Arrangement.Center
        ) {
            CharacterAvatar(name = character.name.ifBlank { "Amitia" }, size = 80)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
            Text(
                text = "信息摘要",
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
            SummaryTable(
                items = listOf(
                    "角色名字" to character.name.ifBlank { "未设置" },
                    "角色身份" to character.identity.ifBlank { "未设置" },
                    "角色性格" to personalityText.ifBlank { "未设置" },
                    "你的称呼" to state.memory.userNickname.ifBlank { "未设置" },
                    "关系背景" to state.memory.relationship.ifBlank { "未设置" },
                    "偏好约束" to state.memory.preferences.ifBlank { "未设置" }
                )
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
            PrimaryButton(
                text = "继续",
                onClick = onContinue,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaSpacing.Xl),
                leadingIcon = AmitiaIcons.ArrowForward
            )
        }
    }
}

@Composable
private fun SummaryTable(items: List<Pair<String, String>>) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            items.forEachIndexed { index, (label, value) ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = AmitiaSpacing.Sm),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(
                        text = label,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = value,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        textAlign = TextAlign.End,
                        modifier = Modifier.weight(1f).padding(start = AmitiaSpacing.Base)
                    )
                }
                if (index < items.lastIndex) {
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                }
            }
        }
    }
}

@Preview(name = "SetupSummary - Light", showBackground = true)
@Composable
private fun SetupSummaryStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SetupSummaryStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(
                    name = "艾米",
                    identity = "朋友",
                    personality = "温柔体贴"
                ),
                memory = InitialMemoryState(
                    userNickname = "小明",
                    relationship = "大学同学",
                    preferences = "回复简洁"
                )
            ),
            onContinue = {}
        )
    }
}

@Preview(name = "SetupSummary - Dark", showBackground = true)
@Composable
private fun SetupSummaryStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SetupSummaryStepPage(
            state = OnboardingFlowUiState(),
            onContinue = {}
        )
    }
}
