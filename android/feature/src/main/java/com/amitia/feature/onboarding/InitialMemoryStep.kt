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
fun InitialMemoryStep(
    state: OnboardingUiState,
    onCreate: (content: String, scope: String) -> Unit
) {
    var content by remember { mutableStateOf("") }
    var scope by remember { mutableStateOf("global") }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "初始记忆",
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onBackground,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = "为角色写入第一条初始记忆，作为后续对话与检索的种子。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        OutlinedTextField(
            value = content,
            onValueChange = { content = it },
            label = { Text(text = "记忆内容") },
            modifier = Modifier.fillMaxWidth(),
            minLines = 3
        )
        OutlinedTextField(
            value = scope,
            onValueChange = { scope = it },
            label = { Text(text = "作用域") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true
        )
        Button(
            onClick = { onCreate(content.trim(), scope.trim().ifBlank { "global" }) },
            modifier = Modifier.fillMaxWidth(),
            enabled = content.isNotBlank() && state.characterId != null
        ) {
            Text(text = "写入记忆")
        }
        if (state.characterId == null) {
            Text(
                text = "需要先创建角色再写入记忆",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}
