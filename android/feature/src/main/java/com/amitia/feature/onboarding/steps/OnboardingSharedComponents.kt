package com.amitia.feature.onboarding.steps

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTouchTarget

@Composable
fun OnboardingProgressLine(
    currentStep: OnboardingFlowStep,
    total: Int = OnboardingFlowStep.entries.size,
    modifier: Modifier = Modifier
) {
    val progress = (currentStep.index + 1f) / total.toFloat()
    LinearProgressIndicator(
        progress = { progress },
        modifier = modifier
            .fillMaxWidth()
            .height(2.dp),
        color = MaterialTheme.colorScheme.primary,
        trackColor = MaterialTheme.colorScheme.surfaceVariant
    )
}

@Composable
fun OnboardingBackButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true
) {
    val interactionSource = remember { MutableInteractionSource() }
    val tint = if (enabled) MaterialTheme.colorScheme.onSurfaceVariant
    else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f)
    Box(
        modifier = modifier
            .size(AmitiaTouchTarget.Minimum)
            .clip(CircleShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.ArrowBack,
            contentDescription = "返回上一层",
            tint = tint,
            modifier = Modifier.size(24.dp)
        )
    }
}

@Composable
fun CharacterAvatar(
    name: String,
    modifier: Modifier = Modifier,
    size: Int = 96,
    appearance: String = "default"
) {
    val initial = name.firstOrNull()?.uppercase() ?: "A"
    Box(
        modifier = modifier
            .size(size.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.primaryContainer)
            .border(2.dp, MaterialTheme.colorScheme.primary.copy(alpha = 0.2f), CircleShape),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = initial,
            style = MaterialTheme.typography.displayMedium,
            color = MaterialTheme.colorScheme.onPrimaryContainer,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
fun OnboardingStepScaffold(
    currentStep: OnboardingFlowStep,
    transitioning: Boolean,
    onBack: (() -> Unit)?,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        OnboardingProgressLine(currentStep = currentStep)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (onBack != null) {
                OnboardingBackButton(onClick = onBack)
            } else {
                Spacer(modifier = Modifier.size(AmitiaTouchTarget.Minimum))
            }
        }
        Box(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
        ) {
            AnimatedVisibility(
                visible = !transitioning,
                enter = fadeIn(),
                exit = fadeOut()
            ) {
                content()
            }
        }
    }
}

@Composable
fun StepTitle(
    text: String,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        style = MaterialTheme.typography.headlineMedium,
        color = MaterialTheme.colorScheme.onBackground,
        fontWeight = FontWeight.Medium,
        textAlign = TextAlign.Center,
        modifier = modifier
    )
}

@Composable
fun StepDescription(
    text: String,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center,
        modifier = modifier
    )
}

@Composable
fun ChipFillOption(
    label: String,
    selected: Boolean,
    onSelect: () -> Unit,
    modifier: Modifier = Modifier
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .clip(AmitiaPillShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.RadioButton,
                onClick = onSelect
            ),
        shape = AmitiaPillShape,
        color = if (selected) MaterialTheme.colorScheme.primaryContainer
        else MaterialTheme.colorScheme.surfaceVariant
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
            else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
        )
    }
}
