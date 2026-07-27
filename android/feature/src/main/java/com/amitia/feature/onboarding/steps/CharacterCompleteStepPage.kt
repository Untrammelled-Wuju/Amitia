package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
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
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun CharacterCompleteStepPage(
    state: OnboardingFlowUiState,
    onEditName: () -> Unit,
    onEditIdentity: () -> Unit,
    onEditPersonality: () -> Unit,
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
            CharacterAvatar(name = character.name.ifBlank { "Amitia" }, size = 96)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            Text(
                text = "以后，你可以叫我「${character.name.ifBlank { "Amitia" }}」。",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "我是你的「${character.identity.ifBlank { "伙伴" }}」。",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "我会以你设定的性格，陪在你身边。",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            CurrentSettingsTable(
                name = character.name.ifBlank { "未设置" },
                identity = character.identity.ifBlank { "未设置" },
                personality = personalityText.ifBlank { "未设置" },
                onEditName = onEditName,
                onEditIdentity = onEditIdentity,
                onEditPersonality = onEditPersonality
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
private fun CurrentSettingsTable(
    name: String,
    identity: String,
    personality: String,
    onEditName: () -> Unit,
    onEditIdentity: () -> Unit,
    onEditPersonality: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Text(
                text = "当前设定",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(bottom = AmitiaSpacing.Sm)
            )
            SettingRow(label = "名字", value = name, onEdit = onEditName)
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            SettingRow(label = "身份", value = identity, onEdit = onEditIdentity)
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            SettingRow(label = "性格", value = personality, onEdit = onEditPersonality)
        }
    }
}

@Composable
private fun SettingRow(
    label: String,
    value: String,
    onEdit: () -> Unit
) {
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
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(0.3f)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.weight(0.5f)
        )
        SecondaryButton(
            text = "修改",
            onClick = onEdit,
            leadingIcon = AmitiaIcons.Edit
        )
    }
}

@Preview(name = "CharacterComplete - Light", showBackground = true)
@Composable
private fun CharacterCompleteStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterCompleteStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(
                    name = "艾米",
                    identity = "朋友",
                    personality = "温柔体贴"
                )
            ),
            onEditName = {},
            onEditIdentity = {},
            onEditPersonality = {},
            onContinue = {}
        )
    }
}

@Preview(name = "CharacterComplete - Dark", showBackground = true)
@Composable
private fun CharacterCompleteStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterCompleteStepPage(
            state = OnboardingFlowUiState(),
            onEditName = {},
            onEditIdentity = {},
            onEditPersonality = {},
            onContinue = {}
        )
    }
}
