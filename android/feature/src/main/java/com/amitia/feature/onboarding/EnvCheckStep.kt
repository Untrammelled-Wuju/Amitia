package com.amitia.feature.onboarding

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.AmitiaColors

@Composable
fun EnvCheckStep(
    state: OnboardingUiState,
    onCheck: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "环境检查",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "连接后端 /api/health，验证延迟与版本。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        if (state.envChecking) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                AmitiaLoadingIndicator()
                Text(
                    text = "正在检查…",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        if (state.envConnected) {
            EnvCheckRow(label = "连接状态", value = "已连接", ok = true)
            EnvCheckRow(label = "服务版本", value = state.serverVersion ?: "未知", ok = true)
            EnvCheckRow(label = "延迟", value = "${state.latencyMs} ms", ok = state.latencyMs < 800)
        }
        if (state.envError != null) {
            EnvCheckRow(label = "错误", value = state.envError, ok = false)
        }
        Button(
            onClick = onCheck,
            modifier = Modifier.fillMaxWidth(),
            enabled = !state.envChecking
        ) {
            Text(text = if (state.envConnected) "重新检查" else "开始检查")
        }
    }
}

@Composable
private fun EnvCheckRow(label: String, value: String, ok: Boolean) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        AmitiaStatusDot(color = if (ok) AmitiaColors.StateRunning else AmitiaColors.StateFailed)
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}
