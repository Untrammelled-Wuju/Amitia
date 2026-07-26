package com.amitia.feature.onboarding

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp

@Composable
fun RemoteConfigStep(
    state: OnboardingUiState,
    onConfigure: (baseUrl: String, token: String) -> Unit
) {
    var baseUrl by remember(state.remoteBaseUrl) { mutableStateOf(state.remoteBaseUrl) }
    var token by remember(state.remoteToken) { mutableStateOf(state.remoteToken) }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "远程服务配置",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "填写远程 Amitia 后端地址与访问令牌（如无令牌可留空）。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        OutlinedTextField(
            value = baseUrl,
            onValueChange = { baseUrl = it },
            label = { Text(text = "后端地址") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        OutlinedTextField(
            value = token,
            onValueChange = { token = it },
            label = { Text(text = "访问令牌") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            visualTransformation = PasswordVisualTransformation()
        )
        Button(
            onClick = { onConfigure(baseUrl.trim(), token.trim()) },
            modifier = Modifier.fillMaxWidth(),
            enabled = baseUrl.isNotBlank() && baseUrl.startsWith("http")
        ) {
            Text(text = if (state.remoteConfigured) "已保存，重新保存" else "保存并测试")
        }
        if (state.remoteConfigured) {
            Text(
                text = "远程配置已生效，端点已切换。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.tertiary
            )
        }
    }
}
