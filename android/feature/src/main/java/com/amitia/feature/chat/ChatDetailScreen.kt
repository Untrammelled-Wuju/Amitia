package com.amitia.feature.chat

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
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaActionMenu
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.AmitiaChatDockShape
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaMenuItem
import com.amitia.core.designsystem.component.ChatInputDock
import com.amitia.core.designsystem.component.MessageGroup
import com.amitia.core.designsystem.component.MessageSender
import com.amitia.core.designsystem.component.MessageStatus
import com.amitia.core.designsystem.component.MessageStatusIndicator
import com.amitia.core.designsystem.component.ToolExecutionCard
import com.amitia.core.model.MessageDto

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatDetailScreen(
    characterId: String,
    onBack: () -> Unit,
    onOpenCharacter: (String) -> Unit,
    onOpenSettings: () -> Unit,
    onOpenMedia: () -> Unit,
    onOpenExport: () -> Unit,
    onOpenSearch: () -> Unit,
    onOpenContext: () -> Unit,
    onOpenPromptTrace: () -> Unit,
    onOpenVoiceCall: () -> Unit,
    viewModel: ChatViewModel = hiltViewModel(),
    extendViewModel: ChatExtendViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val mergeHint by extendViewModel.mergeHint.collectAsStateWithLifecycle()
    val listState = rememberLazyListState()
    var showAttachmentSheet by remember { mutableStateOf(false) }
    var showMenu by remember { mutableStateOf(false) }
    var inputText by remember { mutableStateOf(state.draft) }

    LaunchedEffect(characterId) {
        viewModel.loadConversation(characterId)
    }

    LaunchedEffect(state.messages.size) {
        if (state.messages.isNotEmpty()) {
            listState.animateScrollToItem(state.messages.size - 1)
        }
    }

    val menuItems = remember(onOpenSettings, onOpenMedia, onOpenExport, onOpenContext, onOpenPromptTrace) {
        listOf(
            AmitiaMenuItem("搜索消息", AmitiaIcons.Search, onClick = onOpenSearch),
            AmitiaMenuItem("媒体库", AmitiaIcons.Image, onClick = onOpenMedia),
            AmitiaMenuItem("上下文管理", AmitiaIcons.Layers, onClick = onOpenContext),
            AmitiaMenuItem("导出对话", AmitiaIcons.Download, onClick = onOpenExport),
            AmitiaMenuItem("对话设置", AmitiaIcons.Settings, onClick = onOpenSettings),
            AmitiaMenuItem("Prompt Trace", AmitiaIcons.Code, onClick = onOpenPromptTrace)
        )
    }

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        ChatDetailTopBar(
            title = state.conversation?.title ?: "对话",
            generating = state.generating,
            onBack = onBack,
            onOpenCharacter = { onOpenCharacter(characterId) },
            onVoiceCall = onOpenVoiceCall,
            menuItems = menuItems,
            showMenu = showMenu,
            onToggleMenu = { showMenu = it }
        )
        Box(modifier = Modifier.weight(1f)) {
            if (state.loading && state.messages.isEmpty()) {
                ChatDetailLoading()
            } else if (state.messages.isEmpty()) {
                ChatDetailEmpty()
            } else {
                val grouped = remember(state.messages) { groupMessages(state.messages) }
                LazyColumn(
                    state = listState,
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(vertical = AmitiaSpacing.Sm),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
                ) {
                    grouped.forEach { group ->
                        item(key = "group_${group.first().id}") {
                            MessageGroup(
                                sender = if (group.first().role == "user") MessageSender.User else MessageSender.Character,
                                senderName = if (group.first().role != "user") "艾米" else null,
                                avatarContent = { GroupAvatar() },
                                messages = group.map { msg ->
                                    { ChatMessageItem(message = msg, generating = state.generating) }
                                }
                            )
                        }
                    }
                }
            }
            if (mergeHint.active) {
                MergeHintBanner(
                    remainingSeconds = mergeHint.remainingSeconds,
                    onCancel = extendViewModel::cancelMerge
                )
            }
        }
        ChatInputDock(
            text = inputText,
            onTextChange = {
                inputText = it
                viewModel.saveDraft(it)
                if (it.isNotBlank() && !mergeHint.active) extendViewModel.startMergeTimer()
            },
            onSend = {
                viewModel.sendMessage(inputText)
                inputText = ""
            },
            onStop = { viewModel.consumeError() },
            onAttach = { showAttachmentSheet = true },
            onVoice = {},
            isGenerating = state.generating
        )
    }

    if (showAttachmentSheet) {
        AttachmentSheet(
            onDismiss = { showAttachmentSheet = false },
            onPickImage = { showAttachmentSheet = false },
            onTakePhoto = { showAttachmentSheet = false },
            onPickFile = { showAttachmentSheet = false },
            onPickMemory = { showAttachmentSheet = false }
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ChatDetailTopBar(
    title: String,
    generating: Boolean,
    onBack: () -> Unit,
    onOpenCharacter: () -> Unit,
    onVoiceCall: () -> Unit,
    menuItems: List<AmitiaMenuItem>,
    showMenu: Boolean,
    onToggleMenu: (Boolean) -> Unit
) {
    TopAppBar(
        title = {
            Column {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                    Text(
                        text = title,
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Box(
                        modifier = Modifier.size(6.dp).clip(CircleShape)
                            .background(MaterialTheme.colorScheme.tertiary)
                    )
                }
                Text(
                    text = if (generating) "正在生成…" else "在线",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (generating) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        },
        navigationIcon = {
            AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
        },
        actions = {
            AmitiaIconButton(icon = AmitiaIcons.Phone, contentDescription = "语音通话", onClick = onVoiceCall)
            Box {
                AmitiaIconButton(icon = AmitiaIcons.MoreVert, contentDescription = "更多", onClick = { onToggleMenu(true) })
                AmitiaActionMenu(
                    expanded = showMenu,
                    onDismiss = { onToggleMenu(false) },
                    items = menuItems
                )
            }
        },
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background
        )
    )
}

@Composable
private fun GroupAvatar() {
    Box(
        modifier = Modifier.size(28.dp).clip(CircleShape)
            .background(MaterialTheme.colorScheme.tertiaryContainer),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = "艾",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onTertiaryContainer
        )
    }
}

@Composable
private fun ChatMessageItem(message: MessageDto, generating: Boolean) {
    val status = mapStatus(message.status, generating, message.id)
    val bubbleColor = if (message.role == "user") MaterialTheme.colorScheme.surfaceVariant
    else MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    val bubbleShape = if (message.role == "user") {
        RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp, bottomStart = 16.dp, bottomEnd = 4.dp)
    } else {
        RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp, bottomStart = 4.dp, bottomEnd = 16.dp)
    }
    when (message.contentType) {
        "system" -> SystemMessageItem(message)
        "tool" -> ToolExecutionCard(toolName = message.content, status = status ?: MessageStatus.ToolExecuting)
        else -> {
            Box(
                modifier = Modifier
                    .fillMaxWidth(if (message.role == "user") 0.8f else 0.85f)
                    .clip(bubbleShape)
                    .background(bubbleColor)
                    .padding(horizontal = 14.dp, vertical = 10.dp)
            ) {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)) {
                    Text(
                        text = if (message.content.isBlank() && status == MessageStatus.Streaming) "正在生成…"
                        else message.content,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    if (status != null && message.role != "system") {
                        MessageStatusIndicator(status = status)
                    }
                }
            }
        }
    }
}

@Composable
private fun SystemMessageItem(message: MessageDto) {
    Box(
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = message.content,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier
                .background(MaterialTheme.colorScheme.surfaceVariant, RoundedCornerShape(8.dp))
                .padding(horizontal = 8.dp, vertical = 2.dp),
            maxLines = 2,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun ChatDetailLoading() {
    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Text(text = "加载中…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun ChatDetailEmpty() {
    Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                imageVector = AmitiaIcons.ChatBubbleOutline,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(48.dp)
            )
            Text(
                text = "开始第一条对话吧",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
        }
    }
}

private fun mapStatus(status: String?, generating: Boolean, id: String): MessageStatus? {
    return when (status) {
        "sent" -> MessageStatus.Sent
        "streaming" -> MessageStatus.Streaming
        "failed" -> MessageStatus.Failed
        "cancelled" -> MessageStatus.Cancelled
        "tool_executing" -> MessageStatus.ToolExecuting
        null -> if (generating) MessageStatus.Sending else null
        else -> null
    }
}

private fun groupMessages(messages: List<MessageDto>): List<List<MessageDto>> {
    return messages.groupBy { it.role + (it.channel ?: "") }.values.toList()
}

@Preview(name = "Chat Detail - Light", showBackground = true)
@Composable
private fun ChatDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            MessageGroup(
                sender = MessageSender.Character,
                senderName = "艾米",
                avatarContent = {},
                messages = listOf({
                    Text(text = "你好，今天有什么可以帮你的吗？", style = MaterialTheme.typography.bodyMedium)
                })
            )
        }
    }
}

@Preview(name = "Chat Detail - Dark", showBackground = true)
@Composable
private fun ChatDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChatInputDock(
            text = "",
            onTextChange = {},
            onSend = {},
            onAttach = {},
            onVoice = {}
        )
    }
}
