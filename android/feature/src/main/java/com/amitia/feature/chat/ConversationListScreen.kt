package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConversationListScreen(
    onOpenConversation: (String) -> Unit,
    onNewConversation: () -> Unit,
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val state by viewModel.conversationListState.collectAsStateWithLifecycle()
    val query by viewModel.conversationQuery.collectAsStateWithLifecycle()
    var pendingDelete by remember { mutableStateOf<ConversationListItem?>(null) }

    Column(modifier = Modifier.fillMaxSize()) {
        ChatListTopLine(
            onMenu = onMenu,
            onNew = onNewConversation
        )
        SearchField(
            value = query,
            onValueChange = viewModel::updateConversationQuery,
            onClear = { viewModel.updateConversationQuery("") },
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Md, vertical = AmitiaSpacing.Sm)
        )
        when (val s = state) {
            is ScreenState.Loading -> {
                LoadingSkeleton(lineCount = 6, lineHeight = 64)
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.CloudOff,
                    title = s.error.title,
                    description = s.error.message,
                    onRetry = viewModel::loadConversations
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.ChatBubbleOutline,
                    title = "还没有对话",
                    description = "选择一个角色开始对话吧",
                    primaryAction = { PrimaryButton(text = "新建会话", onClick = onNewConversation) }
                )
            }
            is ScreenState.Content, is ScreenState.Partial -> {
                val all = (s as ScreenState.Content<List<ConversationListItem>>).data
                val filtered = filterConversations(all, query)
                ConversationListContent(
                    conversations = filtered,
                    onClick = onOpenConversation,
                    onPin = viewModel::pinConversation,
                    onMute = viewModel::muteConversation,
                    onArchive = viewModel::archiveConversation,
                    onDelete = { pendingDelete = it }
                )
            }
        }
    }

    pendingDelete?.let { item ->
        AmitiaConfirmDialog(
            onDismiss = { pendingDelete = null },
            onConfirm = {
                viewModel.deleteConversation(item.id)
                pendingDelete = null
            },
            title = "删除会话",
            message = "确定删除「${item.title}」吗？此操作不可恢复。",
            confirmText = "删除",
            destructive = true
        )
    }
}

@Composable
private fun ChatListTopLine(
    onMenu: () -> Unit,
    onNew: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Md, vertical = AmitiaSpacing.Sm),
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
                text = "对话",
                fontSize = 27.sp,
                fontWeight = FontWeight(620),
                color = MaterialTheme.colorScheme.onBackground
            )
            Text(
                text = "每段对话都有独立的关系线",
                fontSize = 13.sp,
                color = AmitiaColors.OnSurfaceMuted
            )
        }
        TopLineIconButton(
            icon = AmitiaIcons.Add,
            contentDescription = "新建会话",
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
private fun SearchField(
    value: String,
    onValueChange: (String) -> Unit,
    onClear: () -> Unit,
    modifier: Modifier = Modifier
) {
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .height(48.dp),
        shape = RoundedCornerShape(AmitiaRadius.M),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = AmitiaSpacing.Md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Search,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            TextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier.weight(1f),
                placeholder = {
                    Text(
                        text = "搜索会话",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                },
                singleLine = true,
                textStyle = MaterialTheme.typography.bodyMedium.copy(color = MaterialTheme.colorScheme.onSurface),
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = Color.Transparent,
                    unfocusedContainerColor = Color.Transparent,
                    disabledContainerColor = Color.Transparent,
                    focusedIndicatorColor = Color.Transparent,
                    unfocusedIndicatorColor = Color.Transparent,
                    disabledIndicatorColor = Color.Transparent,
                    cursorColor = MaterialTheme.colorScheme.primary
                )
            )
            if (value.isNotEmpty()) {
                val clearInteractionSource = remember { MutableInteractionSource() }
                Box(
                    modifier = Modifier
                        .size(AmitiaIconSize.Large)
                        .clip(CircleShape)
                        .clickable(
                            interactionSource = clearInteractionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onClear
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Close,
                        contentDescription = "清除",
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ConversationListContent(
    conversations: List<ConversationListItem>,
    onClick: (String) -> Unit,
    onPin: (String) -> Unit,
    onMute: (String) -> Unit,
    onArchive: (String) -> Unit,
    onDelete: (ConversationListItem) -> Unit
) {
    val pinned = conversations.filter { it.pinned && !it.archived }
    val others = conversations.filter { !it.pinned && !it.archived }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(bottom = AmitiaSpacing.Xl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        if (pinned.isNotEmpty()) {
            item(key = "pinned_header") {
                AmitiaSectionHeader(title = "置顶", modifier = Modifier.padding(horizontal = AmitiaSpacing.Md))
            }
            items(pinned, key = { "p_${it.id}" }) { item ->
                SwipeableConversationItem(
                    item = item,
                    onClick = { onClick(item.id) },
                    onPin = { onPin(item.id) },
                    onMute = { onMute(item.id) },
                    onArchive = { onArchive(item.id) },
                    onDelete = { onDelete(item) }
                )
            }
        }
        if (others.isNotEmpty()) {
            item(key = "others_header") {
                AmitiaSectionHeader(title = "最近", modifier = Modifier.padding(horizontal = AmitiaSpacing.Md))
            }
            items(others, key = { "o_${it.id}" }) { item ->
                SwipeableConversationItem(
                    item = item,
                    onClick = { onClick(item.id) },
                    onPin = { onPin(item.id) },
                    onMute = { onMute(item.id) },
                    onArchive = { onArchive(item.id) },
                    onDelete = { onDelete(item) }
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun SwipeableConversationItem(
    item: ConversationListItem,
    onClick: () -> Unit,
    onPin: () -> Unit,
    onMute: () -> Unit,
    onArchive: () -> Unit,
    onDelete: () -> Unit
) {
    val swipeState = rememberSwipeToDismissBoxState(
        confirmValueChange = { value ->
            when (value) {
                SwipeToDismissBoxValue.StartToEnd -> { onPin(); false }
                SwipeToDismissBoxValue.EndToStart -> { onArchive(); false }
                SwipeToDismissBoxValue.Settled -> false
            }
        }
    )
    SwipeToDismissBox(
        state = swipeState,
        backgroundContent = {
            SwipeBackground(swipeState.targetValue)
        },
        modifier = Modifier.padding(horizontal = AmitiaSpacing.Md)
    ) {
        ConversationItem(
            item = item,
            onClick = onClick
        )
    }
}

@Composable
private fun ConversationItem(
    item: ConversationListItem,
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
        shape = RoundedCornerShape(AmitiaRadius.L),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            Box(
                modifier = Modifier
                    .size(54.dp)
                    .clip(RoundedCornerShape(19.dp))
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (item.muted) AmitiaIcons.NotificationsOff else AmitiaIcons.Person,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Nav)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = item.lastMessage,
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaColors.OnSurfaceMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Column(
                horizontalAlignment = Alignment.End,
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                if (item.lastMessageAt.isNotEmpty()) {
                    Text(
                        text = item.lastMessageAt,
                        style = MaterialTheme.typography.labelSmall,
                        color = AmitiaColors.OnSurfaceMuted
                    )
                }
                if (item.unreadCount > 0) {
                    Box(
                        modifier = Modifier
                            .size(20.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primary),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = if (item.unreadCount > 99) "99+" else item.unreadCount.toString(),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            fontWeight = FontWeight.Medium
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SwipeBackground(target: SwipeToDismissBoxValue) {
    val (color, icon, alignment) = when (target) {
        SwipeToDismissBoxValue.StartToEnd -> Triple(
            MaterialTheme.colorScheme.primaryContainer,
            AmitiaIcons.Bookmark,
            Alignment.CenterStart
        )
        SwipeToDismissBoxValue.EndToStart -> Triple(
            MaterialTheme.colorScheme.surfaceVariant,
            AmitiaIcons.Archive,
            Alignment.CenterEnd
        )
        SwipeToDismissBoxValue.Settled -> Triple(
            Color.Transparent,
            AmitiaIcons.ArrowBack,
            Alignment.Center
        )
    }
    Row(
        modifier = Modifier
            .fillMaxSize()
            .clip(RoundedCornerShape(AmitiaRadius.L))
            .background(color)
            .padding(horizontal = AmitiaSpacing.Lg),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = if (alignment == Alignment.CenterStart) Arrangement.Start else Arrangement.End
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurface
        )
    }
}

private fun filterConversations(
    all: List<ConversationListItem>,
    query: String
): List<ConversationListItem> {
    var result = all
    if (query.isNotBlank()) {
        result = result.filter { it.title.contains(query, ignoreCase = true) || it.lastMessage.contains(query, ignoreCase = true) }
    }
    return result
}

@Preview(name = "Conversation List - Light", showBackground = true)
@Composable
private fun ConversationListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ConversationListScreenPreviewBody()
    }
}

@Preview(name = "Conversation List - Dark", showBackground = true)
@Composable
private fun ConversationListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ConversationListScreenPreviewBody()
    }
}

@Composable
private fun ConversationListScreenPreviewBody() {
    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
        ConversationItem(
            item = ConversationListItem(
                id = "1",
                title = "与艾米的对话",
                characterId = "c1",
                characterName = "艾米",
                lastMessage = "想和你确认明天会议的时间",
                lastMessageAt = "14:20",
                channel = "web",
                unreadCount = 2,
                pinned = true
            ),
            onClick = {}
        )
    }
}
