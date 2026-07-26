package com.amitia.feature.chat

import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.model.MessageDto
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun ChatScreen(
    characterId: String? = null,
    onOpenCharacter: (String) -> Unit,
    onBack: () -> Unit,
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

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(
                            text = state.conversation?.title ?: "对话",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onBackground,
                            fontWeight = FontWeight.Medium,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (state.generating) {
                            Text(
                                text = "正在生成…",
                                style = MaterialTheme.typography.labelSmall,
                                color = AmitiaColors.OnSurfaceMuted
                            )
                        }
                    }
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    val cid = characterId
                    if (cid != null) {
                        IconButton(onClick = { onOpenCharacter(cid) }) {
                            Text(
                                text = "角色",
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.primary
                            )
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        },
        bottomBar = {
            ChatInputBar(
                draft = state.draft,
                sending = state.sending,
                generating = state.generating,
                onTextChanged = viewModel::saveDraft,
                onSend = {
                    viewModel.sendMessage(state.draft)
                },
                onPickImage = { /* 由 ChatScreen 包装的 ImagePicker 处理 */ },
                onTakePhoto = { /* 同上 */ },
                onStartRecording = { /* 由 ChatScreen 包装的 VoiceRecorder 处理 */ }
            )
        }
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .padding(padding)
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
                    contentPadding = PaddingValues(vertical = 12.dp),
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
                                onPlayAudio = { /* 由 TTS/AudioPlayerController 处理 */ }
                            )
                        }
                    }
                }
            }
        }
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
