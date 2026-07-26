package com.amitia.feature.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors

@Composable
fun RuntimeInstallStep(
    state: OnboardingUiState,
    onStart: () -> Unit,
    onRetry: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "运行时安装",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "解压并部署 RootFS、SurrealDB、Qdrant、Go 后端，并完成健康检查。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        LinearProgressIndicator(
            progress = { state.runtimeProgress.coerceIn(0f, 1f) },
            modifier = Modifier
                .fillMaxWidth()
                .height(4.dp),
            color = MaterialTheme.colorScheme.primary,
            trackColor = MaterialTheme.colorScheme.surfaceVariant
        )

        ProgressLine(
            label = "RootFS",
            done = state.runtimeProgress >= 0.2f
        )
        ProgressLine(
            label = "SurrealDB",
            done = state.runtimeProgress >= 0.45f
        )
        ProgressLine(
            label = "Qdrant",
            done = state.runtimeProgress >= 0.7f
        )
        ProgressLine(
            label = "Go 后端",
            done = state.runtimeProgress >= 0.9f
        )
        ProgressLine(
            label = "健康检查",
            done = state.runtimeProgress >= 1f
        )

        if (state.runtimeMessage.isNotBlank()) {
            Text(
                text = state.runtimeMessage,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }

        if (state.runtimeError != null) {
            Text(
                text = state.runtimeError,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error
            )
            Button(onClick = onRetry, modifier = Modifier.fillMaxWidth()) {
                Text(text = "重试")
            }
        } else if (state.runtimeProgress == 0f) {
            Button(onClick = onStart, modifier = Modifier.fillMaxWidth()) {
                Text(text = "开始安装")
            }
        }
    }
}

@Composable
private fun ProgressLine(label: String, done: Boolean) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        Box(
            modifier = Modifier
                .size(8.dp)
                .background(
                    color = if (done) MaterialTheme.colorScheme.tertiary else AmitiaColors.Outline,
                    shape = CircleShape
                )
        )
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = if (done) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.weight(1f))
    }
}
