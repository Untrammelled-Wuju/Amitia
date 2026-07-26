package com.amitia.feature.character

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.model.CharacterCreateRequest
import com.amitia.core.model.CharacterUpdateRequest

@Composable
fun CharacterEditScreen(
    characterId: String?,
    onSaved: () -> Unit,
    onBack: () -> Unit,
    viewModel: CharacterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val existing = state.characters.firstOrNull { it.id == characterId }

    var name by remember(existing?.id) { mutableStateOf(existing?.name.orEmpty()) }
    var description by remember(existing?.id) { mutableStateOf(existing?.description.orEmpty()) }
    var personality by remember(existing?.id) { mutableStateOf(existing?.personality.orEmpty()) }
    var systemPrompt by remember(existing?.id) { mutableStateOf(existing?.systemPrompt.orEmpty()) }
    var greeting by remember(existing?.id) { mutableStateOf(existing?.greeting.orEmpty()) }
    var avatar by remember(existing?.id) { mutableStateOf(existing?.avatar.orEmpty()) }
    var scenario by remember(existing?.id) { mutableStateOf(existing?.scenario.orEmpty()) }
    var tagsText by remember(existing?.id) { mutableStateOf(existing?.tags?.joinToString(",") ?: "") }

    LaunchedEffect(characterId) {
        if (characterId != null) viewModel.loadDetail(characterId)
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = if (characterId == null) "新建角色" else "编辑角色",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .imePadding()
                .verticalScroll(rememberScrollState())
                .padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text(text = "名称") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            OutlinedTextField(
                value = avatar,
                onValueChange = { avatar = it },
                label = { Text(text = "头像 URL（可选）") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            OutlinedTextField(
                value = description,
                onValueChange = { description = it },
                label = { Text(text = "身份描述") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 2
            )
            OutlinedTextField(
                value = personality,
                onValueChange = { personality = it },
                label = { Text(text = "性格特征") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 2
            )
            OutlinedTextField(
                value = systemPrompt,
                onValueChange = { systemPrompt = it },
                label = { Text(text = "系统提示词") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 3
            )
            OutlinedTextField(
                value = greeting,
                onValueChange = { greeting = it },
                label = { Text(text = "开场白") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 2
            )
            OutlinedTextField(
                value = scenario,
                onValueChange = { scenario = it },
                label = { Text(text = "场景（可选）") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 2
            )
            OutlinedTextField(
                value = tagsText,
                onValueChange = { tagsText = it },
                label = { Text(text = "标签（逗号分隔）") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            if (state.error != null) {
                Text(
                    text = state.error.orEmpty(),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error
                )
            }
            Button(
                onClick = {
                    val tags = tagsText.split(",").map { it.trim() }.filter { it.isNotBlank() }
                    if (characterId == null) {
                        viewModel.createCharacter(
                            CharacterCreateRequest(
                                name = name.trim(),
                                avatar = avatar.trim().ifBlank { null },
                                description = description.trim().ifBlank { null },
                                personality = personality.trim().ifBlank { null },
                                systemPrompt = systemPrompt.trim().ifBlank { null },
                                greeting = greeting.trim().ifBlank { null },
                                scenario = scenario.trim().ifBlank { null },
                                tags = tags
                            ),
                            onCreated = { onSaved() }
                        )
                    } else {
                        viewModel.updateCharacter(
                            characterId,
                            CharacterUpdateRequest(
                                name = name.trim(),
                                avatar = avatar.trim().ifBlank { null },
                                description = description.trim().ifBlank { null },
                                personality = personality.trim().ifBlank { null },
                                systemPrompt = systemPrompt.trim().ifBlank { null },
                                greeting = greeting.trim().ifBlank { null },
                                scenario = scenario.trim().ifBlank { null },
                                tags = tags
                            ),
                            onUpdated = onSaved
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = name.isNotBlank() && !state.loading
            ) {
                Text(text = if (state.loading) "保存中…" else "保存")
            }
        }
    }
}
