package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun WelcomeStepPage(
    onStart: () -> Unit,
    modifier: Modifier = Modifier
) {
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
            CharacterAvatar(name = "Amitia", size = 112)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
            Text(
                text = "Amitia",
                style = MaterialTheme.typography.displayMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "你的专属 AI 伙伴，已经在这里等你了。",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
            PrimaryButton(
                text = "开始设置",
                onClick = onStart,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaSpacing.Xl)
            )
        }
    }
}

@Preview(name = "Welcome - Light", showBackground = true)
@Composable
private fun WelcomeStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        WelcomeStepPage(onStart = {})
    }
}

@Preview(name = "Welcome - Dark", showBackground = true)
@Composable
private fun WelcomeStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        WelcomeStepPage(onStart = {})
    }
}
