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
fun ModelConfigStep(
    state: OnboardingUiState,
    onConfigure: (provider: String, modelName: String, apiKey: String, endpoint: String) -> Unit
) {
    var provider by remember(state.modelProvider) { mutableStateOf(state.modelProvider.ifBlank { "openai" }) }
    var modelName by remember(state.modelName) { mutableStateOf(state.modelName) }
    var apiKey by remember { mutableStateOf("") }
    var endpoint by remember { mutableStateOf("") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "模型配置",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "选择默认对话模型，API Key 经后端存储，客户端仅做透传测试。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        OutlinedTextField(
            value = provider,
            onValueChange = { provider = it },
            label = { Text(text = "服务商") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        OutlinedTextField(
            value = modelName,
            onValueChange = { modelName = it },
            label = { Text(text = "模型名") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        OutlinedTextField(
            value = apiKey,
            onValueChange = { apiKey = it },
            label = { Text(text = "API Key") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            visualTransformation = PasswordVisualTransformation()
        )
        OutlinedTextField(
            value = endpoint,
            onValueChange = { endpoint = it },
            label = { Text(text = "自定义端点（可选）") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        Button(
            onClick = { onConfigure(provider.trim(), modelName.trim(), apiKey.trim(), endpoint.trim()) },
            modifier = Modifier.fillMaxWidth(),
            enabled = modelName.isNotBlank()
        ) {
            Text(text = "保存配置")
        }
        if (state.modelName.isNotBlank()) {
            Text(
                text = "当前模型：${state.modelName}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.tertiary
            )
        }
    }
}
