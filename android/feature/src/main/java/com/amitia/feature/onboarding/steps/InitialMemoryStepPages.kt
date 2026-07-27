package com.amitia.feature.onboarding.steps

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.PrimaryButton

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun InitialMemory1StepPage(
    state: OnboardingFlowUiState,
    onNicknameChange: (String) -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    MemoryStepScaffold(
        characterName = state.character.name.ifBlank { "Amitia" },
        question = "我应该怎么称呼你？",
        hint = "这个称呼会用于日常对话，之后也可以修改。",
        onNext = onNext,
        nextEnabled = state.memory.userNickname.isNotBlank(),
        modifier = modifier
    ) {
        AmitiaTextField(
            value = state.memory.userNickname,
            onValueChange = onNicknameChange,
            label = "你的称呼",
            placeholder = "如：小明、老板、老师",
            leadingIcon = AmitiaIcons.Person
        )
        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            listOf("主人", "朋友", "同学", "老师").forEach { suggestion ->
                ChipFillOption(
                    label = suggestion,
                    selected = state.memory.userNickname == suggestion,
                    onSelect = { onNicknameChange(suggestion) }
                )
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun InitialMemory2StepPage(
    state: OnboardingFlowUiState,
    onRelationshipChange: (String) -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    MemoryStepScaffold(
        characterName = state.character.name.ifBlank { "Amitia" },
        question = "我们之间有什么重要的关系或背景？",
        hint = "告诉我我们的关系，这会帮助我更好地理解你。",
        onNext = onNext,
        nextEnabled = state.memory.relationship.isNotBlank(),
        modifier = modifier
    ) {
        AmitiaMultilineField(
            value = state.memory.relationship,
            onValueChange = onRelationshipChange,
            label = "关系与背景",
            placeholder = "如：我们是大学同学、你是我工作中最信赖的伙伴……",
            minLines = 3,
            maxLines = 5,
            charLimit = 300
        )
        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            listOf("同事", "同学", "家人", "网友").forEach { suggestion ->
                ChipFillOption(
                    label = suggestion,
                    selected = state.memory.relationship == suggestion,
                    onSelect = { onRelationshipChange(suggestion) }
                )
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun InitialMemory3StepPage(
    state: OnboardingFlowUiState,
    onPreferencesChange: (String) -> Unit,
    onBack: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    MemoryStepScaffold(
        characterName = state.character.name.ifBlank { "Amitia" },
        question = "有什么偏好或约定，希望我长期记住？",
        hint = "你可以随时返回修改前面的内容。这些偏好将影响我的回复方式。",
        onNext = onNext,
        nextEnabled = true,
        modifier = modifier
    ) {
        AmitiaMultilineField(
            value = state.memory.preferences,
            onValueChange = onPreferencesChange,
            label = "偏好与约束",
            placeholder = "如：回复简洁一些、不要使用表情、称呼我用您……",
            minLines = 3,
            maxLines = 6,
            charLimit = 500
        )
        FlowRow(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            listOf("回复简洁", "不用表情", "用敬称", "主动提问").forEach { suggestion ->
                ChipFillOption(
                    label = suggestion,
                    selected = state.memory.preferences.contains(suggestion),
                    onSelect = {
                        val current = state.memory.preferences
                        onPreferencesChange(
                            if (current.contains(suggestion)) current
                            else if (current.isBlank()) suggestion
                            else "$current，$suggestion"
                        )
                    }
                )
            }
        }
        PrimaryButton(
            text = "返回修改",
            onClick = onBack,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.ArrowBack
        )
    }
}

@Composable
private fun MemoryStepScaffold(
    characterName: String,
    question: String,
    hint: String,
    onNext: () -> Unit,
    nextEnabled: Boolean,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Xxl),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xl)
        ) {
            Column(
                modifier = Modifier
                    .weight(0.35f)
                    .fillMaxHeight(),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.Center
            ) {
                CharacterAvatar(name = characterName, size = 96)
            }
            AnimatedVisibility(
                visible = true,
                enter = fadeIn(),
                exit = fadeOut(),
                modifier = Modifier.weight(0.65f)
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxHeight()
                        .verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = question,
                        style = MaterialTheme.typography.headlineSmall,
                        color = MaterialTheme.colorScheme.onBackground,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = hint,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    content()
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                    PrimaryButton(
                        text = "下一步",
                        onClick = onNext,
                        modifier = Modifier.fillMaxWidth(),
                        enabled = nextEnabled,
                        leadingIcon = AmitiaIcons.ArrowForward
                    )
                }
            }
        }
    }
}

@Preview(name = "InitialMemory1 - Light", showBackground = true)
@Composable
private fun InitialMemory1StepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        InitialMemory1StepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                memory = InitialMemoryState(userNickname = "小明")
            ),
            onNicknameChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "InitialMemory1 - Dark", showBackground = true)
@Composable
private fun InitialMemory1StepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        InitialMemory1StepPage(
            state = OnboardingFlowUiState(),
            onNicknameChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "InitialMemory2 - Light", showBackground = true)
@Composable
private fun InitialMemory2StepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        InitialMemory2StepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                memory = InitialMemoryState(relationship = "大学同学")
            ),
            onRelationshipChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "InitialMemory3 - Light", showBackground = true)
@Composable
private fun InitialMemory3StepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        InitialMemory3StepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                memory = InitialMemoryState(preferences = "回复简洁")
            ),
            onPreferencesChange = {},
            onBack = {},
            onNext = {}
        )
    }
}
