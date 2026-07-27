package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

private data class AppearanceOption(
    val id: String,
    val label: String,
    val initial: String
)

private val appearanceOptions = listOf(
    AppearanceOption("default", "默认形象", "A"),
    AppearanceOption("luna", "月华", "L"),
    AppearanceOption("ember", "炽焰", "E"),
    AppearanceOption("mist", "薄雾", "M"),
    AppearanceOption("echo", "回声", "E")
)

@Composable
fun CharacterAppearanceStepPage(
    state: OnboardingFlowUiState,
    onSelect: (String) -> Unit,
    onImportImage: () -> Unit,
    onUseSelected: () -> Unit,
    modifier: Modifier = Modifier
) {
    val selectedId = state.character.appearance
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
            StepTitle(text = "角色形象")
            StepDescription(text = "选择一个角色形象，或导入你喜欢的图片。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AppearancePreview(selectedId = selectedId)
            AppearanceHorizontalList(
                selectedId = selectedId,
                onSelect = onSelect
            )
            SecondaryButton(
                text = "导入图片",
                onClick = onImportImage,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Upload
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "使用这个",
                onClick = onUseSelected,
                modifier = Modifier.fillMaxWidth(),
                enabled = selectedId.isNotBlank(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Composable
private fun AppearancePreview(selectedId: String) {
    val option = appearanceOptions.find { it.id == selectedId } ?: appearanceOptions.first()
    Box(
        modifier = Modifier
            .size(140.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.primaryContainer)
            .border(3.dp, MaterialTheme.colorScheme.primary, CircleShape),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = option.initial,
            style = MaterialTheme.typography.displayLarge,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
            fontWeight = FontWeight.Medium
        )
    }
    Text(
        text = option.label,
        style = MaterialTheme.typography.titleMedium,
        color = MaterialTheme.colorScheme.onBackground,
        fontWeight = FontWeight.Medium,
        textAlign = TextAlign.Center
    )
}

@Composable
private fun AppearanceHorizontalList(
    selectedId: String,
    onSelect: (String) -> Unit
) {
    LazyRow(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(appearanceOptions) { option ->
            val selected = option.id == selectedId
            val interactionSource = remember { MutableInteractionSource() }
            val borderColor = if (selected) MaterialTheme.colorScheme.primary
            else MaterialTheme.colorScheme.outlineVariant
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Box(
                    modifier = Modifier
                        .size(64.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.surfaceVariant)
                        .border(2.dp, borderColor, CircleShape)
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.RadioButton,
                            onClick = { onSelect(option.id) }
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = option.initial,
                        style = MaterialTheme.typography.titleLarge,
                        color = if (selected) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = option.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = if (selected) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Preview(name = "CharacterAppearance - Light", showBackground = true)
@Composable
private fun CharacterAppearanceStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterAppearanceStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(appearance = "luna")
            ),
            onSelect = {},
            onImportImage = {},
            onUseSelected = {}
        )
    }
}

@Preview(name = "CharacterAppearance - Dark", showBackground = true)
@Composable
private fun CharacterAppearanceStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterAppearanceStepPage(
            state = OnboardingFlowUiState(),
            onSelect = {},
            onImportImage = {},
            onUseSelected = {}
        )
    }
}
