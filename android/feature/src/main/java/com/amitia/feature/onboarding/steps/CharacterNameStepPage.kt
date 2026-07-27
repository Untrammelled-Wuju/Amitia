package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.PrimaryButton

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun CharacterNameStepPage(
    state: OnboardingFlowUiState,
    onNameChange: (String) -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    val nameSuggestions = listOf("艾米", "小星", "灵儿", "阿洛", "柚子")
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
            CharacterAvatar(name = state.character.name.ifBlank { "Amitia" }, size = 96)
            Text(
                text = "你想叫我什么？",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Text(
                text = "这个名字将作为角色的称呼，之后也可以修改。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaContentPadding.Horizontal),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                AmitiaTextField(
                    value = state.character.name,
                    onValueChange = onNameChange,
                    label = "角色名字",
                    placeholder = "输入一个名字",
                    leadingIcon = AmitiaIcons.Person
                )
                FlowRow(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    nameSuggestions.forEach { suggestion ->
                        ChipFillOption(
                            label = suggestion,
                            selected = state.character.name == suggestion,
                            onSelect = { onNameChange(suggestion) }
                        )
                    }
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                PrimaryButton(
                    text = "下一步",
                    onClick = onNext,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = state.character.name.isNotBlank(),
                    leadingIcon = AmitiaIcons.ArrowForward
                )
            }
        }
    }
}

@Preview(name = "CharacterName - Light", showBackground = true)
@Composable
private fun CharacterNameStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterNameStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onNameChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "CharacterName - Dark", showBackground = true)
@Composable
private fun CharacterNameStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterNameStepPage(
            state = OnboardingFlowUiState(),
            onNameChange = {},
            onNext = {}
        )
    }
}
