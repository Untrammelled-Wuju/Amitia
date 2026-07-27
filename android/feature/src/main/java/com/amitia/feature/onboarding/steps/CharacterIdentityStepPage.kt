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

private val identityOptions = listOf("朋友", "助手", "管家", "同伴", "导师")

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun CharacterIdentityStepPage(
    state: OnboardingFlowUiState,
    onIdentityChange: (String) -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isCustom = state.character.identity.isNotBlank() && state.character.identity !in identityOptions
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
                text = "你希望我以什么身份陪伴你？",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Text(
                text = "选择一个身份，或输入自定义身份。气泡只填充，不自动提交。",
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
                FlowRow(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    identityOptions.forEach { option ->
                        ChipFillOption(
                            label = option,
                            selected = state.character.identity == option,
                            onSelect = { onIdentityChange(option) }
                        )
                    }
                }
                AmitiaTextField(
                    value = state.character.identity,
                    onValueChange = onIdentityChange,
                    label = "自定义身份",
                    placeholder = "或输入你想要的身份",
                    leadingIcon = AmitiaIcons.PersonOutlined
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                PrimaryButton(
                    text = "下一步",
                    onClick = onNext,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = state.character.identity.isNotBlank(),
                    leadingIcon = AmitiaIcons.ArrowForward
                )
            }
        }
    }
}

@Preview(name = "CharacterIdentity - Light", showBackground = true)
@Composable
private fun CharacterIdentityStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterIdentityStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米", identity = "朋友")
            ),
            onIdentityChange = {},
            onNext = {}
        )
    }
}

@Preview(name = "CharacterIdentity - Dark", showBackground = true)
@Composable
private fun CharacterIdentityStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterIdentityStepPage(
            state = OnboardingFlowUiState(),
            onIdentityChange = {},
            onNext = {}
        )
    }
}
