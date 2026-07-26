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
import androidx.compose.ui.unit.dp

@Composable
fun CharacterSetupStep(
    state: OnboardingUiState,
    onSetup: (name: String, personality: String, systemPrompt: String, greeting: String) -> Unit
) {
    var name by remember { mutableStateOf("Amitia") }
    var personality by remember { mutableStateOf("") }
    var systemPrompt by remember { mutableStateOf("") }
    var greeting by remember { mutableStateOf("") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "创建初始角色",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "默认角色将作为后续对话与记忆的主体。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text(text = "角色名称") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        OutlinedTextField(
            value = personality,
            onValueChange = { personality = it },
            label = { Text(text = "性格特征（可选）") },
            modifier = Modifier.fillMaxWidth()
        )
        OutlinedTextField(
            value = systemPrompt,
            onValueChange = { systemPrompt = it },
            label = { Text(text = "系统提示词（可选）") },
            modifier = Modifier.fillMaxWidth()
        )
        OutlinedTextField(
            value = greeting,
            onValueChange = { greeting = it },
            label = { Text(text = "开场白（可选）") },
            modifier = Modifier.fillMaxWidth()
        )
        Button(
            onClick = { onSetup(name.trim(), personality.trim(), systemPrompt.trim(), greeting.trim()) },
            modifier = Modifier.fillMaxWidth(),
            enabled = name.isNotBlank()
        ) {
            Text(text = if (state.characterId != null) "已创建，重新创建" else "创建角色")
        }
        if (state.characterId != null) {
            Text(
                text = "角色 ID: ${state.characterId}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.tertiary
            )
        }
    }
}
