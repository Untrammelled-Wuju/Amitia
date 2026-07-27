package com.amitia.feature.character.list

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.MoreVert
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Sort
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import coil.compose.AsyncImage
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.component.AmitiaStatusType
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.model.CharacterDto
import com.amitia.feature.character.CharacterViewModel
import com.amitia.feature.character.CharacterDeleteDialog

enum class CharacterSortMode(val label: String) {
    Recent("最近互动"), Name("名称"), Created("创建时间")
}

@Composable
fun CharacterListScreen(
    onOpenDetail: (String) -> Unit,
    onCreate: () -> Unit,
    viewModel: CharacterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    var searchQuery by remember { mutableStateOf("") }
    var sortMode by remember { mutableStateOf(CharacterSortMode.Recent) }
    var showSortMenu by remember { mutableStateOf(false) }

    val filteredCharacters = remember(state.characters, searchQuery, sortMode) {
        val filtered = if (searchQuery.isBlank()) state.characters
        else state.characters.filter {
            it.name.contains(searchQuery, ignoreCase = true) ||
                (it.description?.contains(searchQuery, ignoreCase = true) ?: false)
        }
        when (sortMode) {
            CharacterSortMode.Name -> filtered.sortedBy { it.name }
            CharacterSortMode.Created -> filtered.sortedByDescending { it.createdAt }
            CharacterSortMode.Recent -> filtered.sortedByDescending { it.updatedAt }
        }
    }

    val currentCharacter = filteredCharacters.firstOrNull { it.id == state.currentCharacterId }
    val otherCharacters = filteredCharacters.filterNot { it.id == state.currentCharacterId }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "角色",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                actions = {
                    IconButton(onClick = { showSortMenu = true }) {
                        Icon(Icons.Outlined.Sort, contentDescription = "排序")
                    }
                    DropdownMenu(
                        expanded = showSortMenu,
                        onDismissRequest = { showSortMenu = false }
                    ) {
                        CharacterSortMode.entries.forEach { mode ->
                            DropdownMenuItem(
                                text = {
                                    Text(
                                        text = mode.label,
                                        color = if (sortMode == mode)
                                            MaterialTheme.colorScheme.primary
                                        else MaterialTheme.colorScheme.onSurface
                                    )
                                },
                                onClick = {
                                    sortMode = mode
                                    showSortMenu = false
                                }
                            )
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        floatingActionButton = {
            FloatingActionButton(
                onClick = onCreate,
                containerColor = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.onPrimaryContainer
            ) {
                Icon(Icons.Outlined.Add, contentDescription = "新建角色")
            }
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            AmitiaSearchField(
                value = searchQuery,
                onValueChange = { searchQuery = it },
                onClear = { searchQuery = "" },
                placeholder = "搜索角色",
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
            )
            when {
                state.loading && state.characters.isEmpty() -> {
                    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                        AmitiaLoadingIndicator()
                    }
                }
                state.error != null && state.characters.isEmpty() -> {
                    AmitiaErrorState(
                        icon = Icons.Outlined.Search,
                        title = "加载失败",
                        description = state.error,
                        onRetry = { viewModel.listCharacters() },
                        modifier = Modifier.fillMaxSize()
                    )
                }
                state.characters.isEmpty() -> {
                    AmitiaEmptyState(
                        icon = Icons.Outlined.Add,
                        title = "还没有角色",
                        description = "点击右下角创建第一个角色",
                        modifier = Modifier.fillMaxSize()
                    )
                }
                filteredCharacters.isEmpty() -> {
                    AmitiaEmptyState(
                        icon = Icons.Outlined.Search,
                        title = "未找到匹配角色",
                        description = "试试其他关键词",
                        modifier = Modifier.fillMaxSize()
                    )
                }
                else -> LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = 16.dp, vertical = 4.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    if (currentCharacter != null) {
                        item(key = "current_${currentCharacter.id}") {
                            CurrentCharacterCard(
                                character = currentCharacter,
                                onClick = { onOpenDetail(currentCharacter.id) },
                                onSwitch = { viewModel.switchCharacter(currentCharacter.id) }
                            )
                        }
                        item(key = "divider") {
                            Text(
                                text = "其他角色",
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(top = 4.dp)
                            )
                        }
                    }
                    items(otherCharacters, key = { it.id }) { character ->
                        CharacterListCard(
                            character = character,
                            onClick = { onOpenDetail(character.id) },
                            onSwitch = { viewModel.switchCharacter(character.id) }
                        )
                    }
                }
            }
        }
        if (state.pendingDeleteId != null) {
            CharacterDeleteDialog(
                characterId = state.pendingDeleteId!!,
                onDismiss = viewModel::dismissDelete,
                onConfirm = { id -> viewModel.deleteCharacter(id) {} }
            )
        }
    }
}

@Composable
fun CurrentCharacterCard(
    character: CharacterDto,
    onClick: () -> Unit,
    onSwitch: () -> Unit
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(20.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                CharacterAvatar(character, size = 56)
                Spacer(modifier = Modifier.size(16.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = character.name,
                            style = MaterialTheme.typography.titleLarge,
                            color = MaterialTheme.colorScheme.onPrimaryContainer,
                            fontWeight = FontWeight.Medium
                        )
                        Spacer(modifier = Modifier.size(8.dp))
                        Surface(
                            shape = CircleShape,
                            color = MaterialTheme.colorScheme.primary
                        ) {
                            Text(
                                text = "当前",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                            )
                        }
                    }
                    Text(
                        text = character.description ?: "未填写身份",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.8f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            Spacer(modifier = Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                CharacterStatusChip(
                    label = "运行中",
                    color = AmitiaStateColors.Running
                )
                CharacterStatusChip(
                    label = "最近互动 15分钟前",
                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.6f),
                    showDot = false
                )
                CharacterStatusChip(
                    label = "主动消息已开启",
                    icon = Icons.Outlined.NotificationsActive,
                    color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.6f),
                    showDot = false
                )
            }
        }
    }
}

@Composable
fun CharacterListCard(
    character: CharacterDto,
    onClick: () -> Unit,
    onSwitch: () -> Unit
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            CharacterAvatar(character, size = 48)
            Spacer(modifier = Modifier.size(16.dp))
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = character.name,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Spacer(modifier = Modifier.size(8.dp))
                    AmitiaStatusDot(color = AmitiaStateColors.Idle)
                }
                Text(
                    text = character.description ?: character.personality ?: "未填写",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Surface(
                modifier = Modifier.clickable(onClick = onSwitch),
                shape = CircleShape,
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = "切换",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp)
                )
            }
        }
    }
}

@Composable
fun CharacterAvatar(character: CharacterDto, size: Int = 48) {
    Box(
        modifier = Modifier
            .size(size.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.primaryContainer),
        contentAlignment = Alignment.Center
    ) {
        if (!character.avatar.isNullOrBlank()) {
            AsyncImage(
                model = character.avatar,
                contentDescription = character.name,
                modifier = Modifier.fillMaxSize()
            )
        } else {
            Text(
                text = character.name.take(1),
                style = if (size >= 56) MaterialTheme.typography.headlineMedium
                else MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer
            )
        }
    }
}

@Composable
private fun CharacterStatusChip(
    label: String,
    color: Color,
    icon: androidx.compose.ui.graphics.vector.ImageVector? = null,
    showDot: Boolean = true
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        if (icon != null) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = color,
                modifier = Modifier.size(14.dp)
            )
        } else if (showDot) {
            AmitiaStatusDot(color = color)
        }
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = color
        )
    }
}

@Preview(name = "Character List - Light", showBackground = true)
@Composable
private fun CharacterListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CurrentCharacterCard(
                    character = CharacterDto(
                        id = "1", name = "艾米",
                        description = "温柔知性的陪伴助手",
                        isCurrent = true
                    ),
                    onClick = {},
                    onSwitch = {}
                )
                CharacterListCard(
                    character = CharacterDto(
                        id = "2", name = "小凛",
                        description = "活泼开朗的学习伙伴"
                    ),
                    onClick = {},
                    onSwitch = {}
                )
            }
        }
    }
}

@Preview(name = "Character List - Dark", showBackground = true)
@Composable
private fun CharacterListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                CurrentCharacterCard(
                    character = CharacterDto(
                        id = "1", name = "艾米",
                        description = "温柔知性的陪伴助手",
                        isCurrent = true
                    ),
                    onClick = {},
                    onSwitch = {}
                )
            }
        }
    }
}
