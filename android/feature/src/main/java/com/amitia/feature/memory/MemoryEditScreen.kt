package com.amitia.feature.memory

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.MemoryUpdateRequest

@Composable
fun MemoryEditScreen(
    memoryId: String?,
    onSaved: () -> Unit,
    onBack: () -> Unit,
    viewModel: MemoryViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val existing = state.memories.firstOrNull { it.id == memoryId }

    var content by remember(existing?.id) { mutableStateOf(existing?.content.orEmpty()) }
    var type by remember(existing?.id) { mutableStateOf(existing?.type ?: "long_term") }
    var scope by remember(existing?.id) { mutableStateOf(existing?.scope ?: "global") }
    var characterId by remember(existing?.id) { mutableStateOf(existing?.characterId ?: state.filterCharacterId ?: "") }
    var importance by remember(existing?.id) { mutableFloatStateOf((existing?.importance ?: 0.5).toFloat()) }
    var tagsText by remember(existing?.id) { mutableStateOf(existing?.tags?.joinToString(",") ?: "") }
    var typeExpanded by remember { mutableStateOf(false) }
    var characterExpanded by remember { mutableStateOf(false) }

    LaunchedEffect(memoryId) {
        if (memoryId != null && existing == null) viewModel.listMemories()
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = if (memoryId == null) "新建记忆" else "编辑记忆",
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
                value = content,
                onValueChange = { content = it },
                label = { Text(text = "内容") },
                modifier = Modifier.fillMaxWidth(),
                minLines = 4
            )
            ExposedDropdownMenuBox(
                expanded = typeExpanded,
                onExpandedChange = { typeExpanded = it }
            ) {
                OutlinedTextField(
                    value = type,
                    onValueChange = { type = it },
                    label = { Text(text = "类型") },
                    modifier = Modifier
                        .fillMaxWidth()
                        .menuAnchor(),
                    readOnly = false,
                    trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(typeExpanded) }
                )
                DropdownMenu(
                    expanded = typeExpanded,
                    onDismissRequest = { typeExpanded = false }
                ) {
                    listOf("long_term", "episodic", "initial", "world_book").forEach { t ->
                        DropdownMenuItem(text = { Text(t) }, onClick = {
                            type = t
                            typeExpanded = false
                        })
                    }
                }
            }
            OutlinedTextField(
                value = scope,
                onValueChange = { scope = it },
                label = { Text(text = "作用域") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true
            )
            if (state.characters.isNotEmpty()) {
                ExposedDropdownMenuBox(
                    expanded = characterExpanded,
                    onExpandedChange = { characterExpanded = it }
                ) {
                    OutlinedTextField(
                        value = state.characters.firstOrNull { it.id == characterId }?.name ?: "全局",
                        onValueChange = {},
                        label = { Text(text = "角色") },
                        modifier = Modifier
                            .fillMaxWidth()
                            .menuAnchor(),
                        readOnly = true,
                        trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(characterExpanded) }
                    )
                    DropdownMenu(
                        expanded = characterExpanded,
                        onDismissRequest = { characterExpanded = false }
                    ) {
                        DropdownMenuItem(
                            text = { Text("全局") },
                            onClick = {
                                characterId = ""
                                characterExpanded = false
                            }
                        )
                        state.characters.forEach { c ->
                            DropdownMenuItem(
                                text = { Text(c.name) },
                                onClick = {
                                    characterId = c.id
                                    characterExpanded = false
                                }
                            )
                        }
                    }
                }
            }
            Text(
                text = "重要度: ${"%.2f".format(importance)}",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Slider(
                value = importance,
                onValueChange = { importance = it },
                valueRange = 0f..1f
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
                    if (memoryId == null) {
                        viewModel.createMemory(
                            MemoryCreateRequest(
                                content = content.trim(),
                                type = type.trim().ifBlank { null },
                                scope = scope.trim().ifBlank { null },
                                characterId = characterId.ifBlank { null },
                                importance = importance.toDouble(),
                                tags = tags
                            ),
                            onCreated = { onSaved() }
                        )
                    } else {
                        viewModel.updateMemory(
                            memoryId,
                            MemoryUpdateRequest(
                                content = content.trim(),
                                importance = importance.toDouble(),
                                tags = tags
                            ),
                            onUpdated = onSaved
                        )
                    }
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = content.isNotBlank() && !state.loading
            ) {
                Text(text = if (state.loading) "保存中…" else "保存")
            }
        }
    }
}
