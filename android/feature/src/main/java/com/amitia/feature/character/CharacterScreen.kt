package com.amitia.feature.character

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import coil.compose.AsyncImage
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.model.CharacterDto

@Composable
fun CharacterScreen(
    onOpenDetail: (String) -> Unit,
    onCreate: () -> Unit,
    onMenu: () -> Unit = {},
    viewModel: CharacterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Box(modifier = Modifier.fillMaxSize()) {
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(horizontal = AmitiaContentPadding.Horizontal)
        ) {
            item(key = "topline") {
                CharacterTopLine(onMenu = onMenu, onNew = onCreate)
            }
            when {
                state.loading && state.characters.isEmpty() -> {
                    item(key = "loading") {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 120.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            AmitiaLoadingIndicator()
                        }
                    }
                }
                state.characters.isEmpty() -> {
                    item(key = "empty") {
                        AmitiaEmptyState(
                            title = "还没有角色",
                            description = "点击右上角创建第一个角色",
                            icon = AmitiaIcons.Add,
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 80.dp)
                        )
                    }
                }
                else -> {
                    val currentCharacter = state.characters.firstOrNull { it.id == state.currentCharacterId }
                    val otherCharacters = state.characters.filter { it.id != state.currentCharacterId }
                    if (currentCharacter != null) {
                        item(key = "current_section") {
                            Column(modifier = Modifier.padding(bottom = 18.dp)) {
                                CurrentSectionTitle()
                                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                                CurrentCharacterCard(
                                    character = currentCharacter,
                                    onClick = { onOpenDetail(currentCharacter.id) }
                                )
                            }
                        }
                    }
                    item(key = "others_header") {
                        OthersSectionHeader()
                    }
                    items(otherCharacters, key = { it.id }) { character ->
                        CharacterListRow(
                            character = character,
                            onClick = { onOpenDetail(character.id) }
                        )
                    }
                    item(key = "bottom_spacer") {
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Xl))
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
private fun CharacterTopLine(
    onMenu: () -> Unit,
    onNew: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        TopLineIconButton(
            icon = AmitiaIcons.Menu,
            contentDescription = "菜单",
            onClick = onMenu
        )
        Column(
            modifier = Modifier.weight(1f),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "角色",
                fontSize = 27.sp,
                fontWeight = FontWeight(620),
                color = MaterialTheme.colorScheme.onBackground
            )
            Text(
                text = "每段关系都有独立的记忆与生活。",
                fontSize = 13.sp,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
        TopLineIconButton(
            icon = AmitiaIcons.Add,
            contentDescription = "新建角色",
            onClick = onNew
        )
    }
}

@Composable
private fun TopLineIconButton(
    icon: ImageVector,
    contentDescription: String?,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    AmitiaGlassSurface(
        level = GlassLevel.Chip,
        modifier = Modifier.size(44.dp),
        shape = RoundedCornerShape(16.dp)
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    role = Role.Button,
                    onClick = onClick
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
    }
}

@Composable
private fun CurrentSectionTitle() {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Text(
            text = "当前角色",
            fontSize = 15.sp,
            fontWeight = FontWeight.SemiBold,
            color = MaterialTheme.colorScheme.onSurface
        )
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(7.dp)
                    .clip(CircleShape)
                    .background(AmitiaStateColors.Running)
            )
            Text(
                text = "正在使用",
                fontSize = 11.sp,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
    }
}

@Composable
private fun CurrentCharacterCard(
    character: CharacterDto,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val gradient = Brush.linearGradient(
        colors = listOf(
            MaterialTheme.colorScheme.primaryContainer,
            MaterialTheme.colorScheme.surface
        )
    )
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        shape = RoundedCornerShape(22.dp),
        color = Color.Transparent
    ) {
        Box(modifier = Modifier.background(gradient)) {
            Row(
                modifier = Modifier.padding(14.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(13.dp)
            ) {
                Box(modifier = Modifier.size(70.dp)) {
                    Box(
                        modifier = Modifier
                            .size(70.dp)
                            .clip(RoundedCornerShape(23.dp))
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
                                fontSize = 26.sp,
                                fontWeight = FontWeight.Medium,
                                color = MaterialTheme.colorScheme.onPrimaryContainer
                            )
                        }
                    }
                    Box(
                        modifier = Modifier
                            .align(Alignment.BottomEnd)
                            .size(12.dp)
                            .clip(CircleShape)
                            .background(AmitiaStateColors.Running)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = character.name,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = character.description ?: character.personality ?: "未填写",
                        fontSize = 11.sp,
                        color = AmitiaColors.OnSurfaceMuted,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis
                    )
                    val meta = buildCharacterMeta(character)
                    if (meta.isNotEmpty()) {
                        Spacer(modifier = Modifier.height(2.dp))
                        Text(
                            text = meta,
                            fontSize = 11.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f)
                        )
                    }
                }
                Column(
                    horizontalAlignment = Alignment.End,
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Surface(
                        shape = RoundedCornerShape(50),
                        color = MaterialTheme.colorScheme.primary
                    ) {
                        Text(
                            text = "当前",
                            fontSize = 11.sp,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp)
                        )
                    }
                    Icon(
                        imageVector = AmitiaIcons.ChevronRight,
                        contentDescription = null,
                        tint = AmitiaColors.OnSurfaceMuted,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
        }
    }
}

@Composable
private fun OthersSectionHeader() {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = 4.dp, bottom = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = "其他角色",
            fontSize = 17.sp,
            fontWeight = FontWeight.SemiBold,
            color = MaterialTheme.colorScheme.onSurface
        )
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            Icon(
                imageVector = AmitiaIcons.Sort,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(14.dp)
            )
            Text(
                text = "最近使用",
                fontSize = 12.sp,
                color = MaterialTheme.colorScheme.primary
            )
        }
    }
}

@Composable
private fun CharacterListRow(
    character: CharacterDto,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        shape = RoundedCornerShape(21.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(13.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(64.dp)
                    .clip(RoundedCornerShape(21.dp))
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
                        fontSize = 22.sp,
                        fontWeight = FontWeight.Medium,
                        color = MaterialTheme.colorScheme.onPrimaryContainer
                    )
                }
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = character.name,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = character.description ?: character.personality ?: "未填写",
                    fontSize = 11.sp,
                    color = AmitiaColors.OnSurfaceMuted,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    lineHeight = 17.sp
                )
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = AmitiaColors.OnSurfaceMuted,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

private fun buildCharacterMeta(character: CharacterDto): String {
    val parts = mutableListOf<String>()
    character.createdAt?.takeIf { it.isNotBlank() }?.let { parts.add("创建于 $it") }
    if (character.tags.isNotEmpty()) {
        parts.add("${character.tags.size} 个标签")
    }
    return parts.joinToString(" · ")
}
