package com.amitia.feature.chat

import android.widget.Toast
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
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.model.MessageDto
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun ChatScreen(
    characterId: String? = null,
    onOpenCharacter: (String) -> Unit,
    onBack: () -> Unit,
    onMenu: () -> Unit = {},
    viewModel: ChatViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val listState = rememberLazyListState()

    LaunchedEffect(characterId) {
        characterId?.let { viewModel.loadConversation(it) }
    }

    if (characterId == null) {
        ChatListPlaceholder(
            onOpenCharacter = onOpenCharacter,
            onBack = onBack
        )
        return
    }

    LaunchedEffect(state.messages.size) {
        if (state.messages.isNotEmpty()) {
            listState.animateScrollToItem(state.messages.size - 1)
        }
    }

    val shouldLoadOlder by remember {
        derivedStateOf {
            val firstVisible = listState.firstVisibleItemIndex
            firstVisible <= 1 && state.hasMore && !state.loading
        }
    }
    LaunchedEffect(shouldLoadOlder) {
        if (shouldLoadOlder) viewModel.loadOlder()
    }

    LaunchedEffect(state.error) {
        state.error?.let {
            Toast.makeText(context, it, Toast.LENGTH_SHORT).show()
            viewModel.consumeError()
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        if (state.loading && state.messages.isEmpty()) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(
                    text = "加载中…",
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
        } else {
            val grouped = remember(state.messages) { groupByDate(state.messages) }
            LazyColumn(
                state = listState,
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(
                    top = 123.dp,
                    start = AmitiaSpacing.Base,
                    end = AmitiaSpacing.Base,
                    bottom = 105.dp
                ),
                verticalArrangement = Arrangement.spacedBy(2.dp)
            ) {
                grouped.forEach { (date, messages) ->
                    item(key = "header_$date") { DateHeader(dateText = date) }
                    items(messages, key = { it.id }) { message ->
                        MessageBubble(
                            message = message,
                            onRetry = { viewModel.retryMessage(message.id) },
                            onDelete = { viewModel.deleteMessage(message.id) },
                            onCopy = {
                                viewModel.copyMessage(message.id) { text ->
                                    val clipboard = context.getSystemService(android.content.ClipboardManager::class.java)
                                    clipboard?.setPrimaryClip(
                                        android.content.ClipData.newPlainText("Amitia", text)
                                    )
                                }
                            },
                            onPlayAudio = { }
                        )
                    }
                }
            }
        }

        ChatHeader(
            title = state.conversation?.title ?: "对话",
            generating = state.generating,
            onBack = onBack,
            onMenu = onMenu,
            onMore = { onOpenCharacter(characterId) }
        )

        Box(modifier = Modifier.align(Alignment.BottomCenter)) {
            ChatInputBar(
                draft = state.draft,
                sending = state.sending,
                generating = state.generating,
                onTextChanged = viewModel::saveDraft,
                onSend = {
                    viewModel.sendMessage(state.draft)
                },
                onPickImage = { },
                onTakePhoto = { },
                onStartRecording = { }
            )
        }
    }
}

@Composable
private fun ChatHeader(
    title: String,
    generating: Boolean,
    onBack: () -> Unit,
    onMenu: () -> Unit,
    onMore: () -> Unit
) {
    AmitiaGlassSurface(
        level = GlassLevel.Navigation,
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 12.dp, end = 12.dp, top = 44.dp)
            .height(64.dp),
        shape = RoundedCornerShape(23.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            HeaderIconButton(
                icon = AmitiaIcons.Menu,
                contentDescription = "菜单",
                onClick = onMenu
            )
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(RoundedCornerShape(14.dp))
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Person,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    fontSize = 13.sp,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = if (generating) "正在生成…" else "在线",
                    fontSize = 10.sp,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
            HeaderIconButton(
                icon = AmitiaIcons.MoreHoriz,
                contentDescription = "更多",
                onClick = onMore
            )
        }
    }
}

@Composable
private fun HeaderIconButton(
    icon: ImageVector,
    contentDescription: String?,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Box(
        modifier = Modifier
            .size(38.dp)
            .clip(RoundedCornerShape(14.dp))
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
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
    }
}

private fun groupByDate(messages: List<MessageDto>): List<Pair<String, List<MessageDto>>> {
    val inputFormat = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSSXXX", Locale.getDefault())
    val inputFormatAlt = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", Locale.getDefault())
    val outputFormat = SimpleDateFormat("yyyy 年 MM 月 dd 日", Locale.getDefault())
    return messages.groupBy { message ->
        runCatching { inputFormat.parse(message.createdAt ?: "") }
            .recoverCatching { inputFormatAlt.parse(message.createdAt ?: "") }
            .getOrNull()?.let { outputFormat.format(it) } ?: "未知时间"
    }.toList()
}

@Composable
private fun ChatListPlaceholder(
    onOpenCharacter: (String) -> Unit,
    onBack: () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Text(
                text = "选择一个角色开始对话",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground
            )
            Text(
                text = "从「角色」或「首页」点击进入聊天",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}
