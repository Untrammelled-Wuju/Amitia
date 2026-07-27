package com.amitia.feature.chat

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import com.amitia.core.designsystem.AmitiaCardShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaConfirmDialog
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.ConversationRow
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConversationListScreen(
    onOpenConversation: (String) -> Unit,
    onNewConversation: () -> Unit,
    onBack: () -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val state by viewModel.conversationListState.collectAsStateWithLifecycle()
    val query by viewModel.conversationQuery.collectAsStateWithLifecycle()
    var pendingDelete by remember { mutableStateOf<ConversationListItem?>(null) }
    var filterIndex by remember { mutableStateOf(0) }

    val filters = listOf("全部", "未读", "置顶", "已归档")

    Column(modifier = Modifier.fillMaxSize()) {
        ChatListTopBar(
            onBack = onBack,
            onNew = onNewConversation
        )
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaSearchField(
                value = query,
                onValueChange = viewModel::updateConversationQuery,
                placeholder = "搜索会话",
                onClear = { viewModel.updateConversationQuery("") }
            )
        }
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaChipSelector(
                items = filters.mapIndexed { index, label -> AmitiaChipItem(label, index == filterIndex) },
                onToggle = { filterIndex = it },
                multiSelect = false
            )
        }
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
                val filtered = filterConversations(all, filterIndex, query)
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
private fun ChatListTopBar(
    onBack: () -> Unit,
    onNew: () -> Unit
) {
    androidx.compose.material3.TopAppBar(
        title = {
            androidx.compose.material3.Text(
                text = "对话",
                style = MaterialTheme.typography.titleLarge
            )
        },
        navigationIcon = {
            AmitiaIconButton(
                icon = AmitiaIcons.ArrowBack,
                contentDescription = "返回",
                onClick = onBack
            )
        },
        actions = {
            AmitiaIconButton(
                icon = AmitiaIcons.Add,
                contentDescription = "新建会话",
                onClick = onNew
            )
        },
        colors = androidx.compose.material3.TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background
        )
    )
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
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        if (pinned.isNotEmpty()) {
            item(key = "pinned_header") {
                AmitiaSectionHeader(title = "置顶", modifier = Modifier.padding(horizontal = AmitiaSpacing.Base))
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
                AmitiaSectionHeader(title = "最近", modifier = Modifier.padding(horizontal = AmitiaSpacing.Base))
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
        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
    ) {
        ConversationRow(
            name = item.title,
            lastMessage = item.lastMessage,
            timestamp = item.lastMessageAt,
            unreadCount = item.unreadCount,
            isPinned = item.pinned,
            onClick = onClick,
            avatarContent = {
                Icon(
                    imageVector = if (item.muted) AmitiaIcons.NotificationsOff else AmitiaIcons.Person,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Nav)
                )
            }
        )
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
            .clip(AmitiaCardShape)
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
    filterIndex: Int,
    query: String
): List<ConversationListItem> {
    var result = all
    when (filterIndex) {
        1 -> result = result.filter { it.unreadCount > 0 }
        2 -> result = result.filter { it.pinned }
        3 -> result = result.filter { it.archived }
    }
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
        ConversationRow(
            name = "与艾米的对话",
            lastMessage = "想和你确认明天会议的时间",
            timestamp = "14:20",
            unreadCount = 2,
            isPinned = true,
            onClick = {}
        )
    }
}
