package com.amitia.feature.onboarding.steps

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EnterAmitiaStepPage(
    state: OnboardingFlowUiState,
    onEnter: () -> Unit,
    modifier: Modifier = Modifier
) {
    val animating = state.enterAnimationPlaying
    val scale by animateFloatAsState(
        targetValue = if (animating) 1.5f else 1f,
        animationSpec = tween(1200),
        label = "enterScale"
    )
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Box(
                modifier = Modifier
                    .size(120.dp)
                    .scale(scale)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = state.character.name.firstOrNull()?.uppercase() ?: "A",
                    style = MaterialTheme.typography.displayLarge,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    fontWeight = FontWeight.Medium
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            Text(
                text = "一切就绪",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "${state.character.name.ifBlank { "Amitia" }} 已经准备好陪伴你了。",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
            if (animating) {
                CircularProgressIndicator(
                    modifier = Modifier.size(32.dp),
                    strokeWidth = 3.dp,
                    color = MaterialTheme.colorScheme.primary
                )
            } else {
                PrimaryButton(
                    text = "开始使用 Amitia",
                    onClick = onEnter,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = AmitiaSpacing.Xl),
                    leadingIcon = AmitiaIcons.ArrowForward
                )
            }
        }
    }
}

@Preview(name = "EnterAmitia - Light", showBackground = true)
@Composable
private fun EnterAmitiaStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EnterAmitiaStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onEnter = {}
        )
    }
}

@Preview(name = "EnterAmitia - Dark", showBackground = true)
@Composable
private fun EnterAmitiaStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EnterAmitiaStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                enterAnimationPlaying = true
            ),
            onEnter = {}
        )
    }
}
