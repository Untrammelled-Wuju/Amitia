package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.MemoryReferenceCard
import com.amitia.core.designsystem.component.MessageSender
import com.amitia.core.designsystem.component.MessageStatus
import com.amitia.core.designsystem.component.ToolExecutionCard
import com.amitia.core.designsystem.component.ChatBubble
import com.amitia.core.designsystem.component.TertiaryButton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MessageDetailScreen(
    messageId: String,
    onBack: () -> Unit,
    onViewTool: (String) -> Unit,
    onViewMemory: (String) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(messageId) { viewModel.loadMessageDetail(messageId) }
    val state by viewModel.messageDetailState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "消息详情", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            actions = {
                AmitiaIconButton(icon = AmitiaIcons.Share, contentDescription = "分享", onClick = {})
                AmitiaIconButton(icon = AmitiaIcons.ContentCopy, contentDescription = "复制", onClick = {})
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 5, lineHeight = 48)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadMessageDetail(messageId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.ChatBubbleOutline,
                title = "消息不存在",
                description = "该消息可能已被删除"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val data = (s as ScreenState.Content<MessageDetailData>).data
                MessageDetailContent(
                    data = data,
                    onViewTool = onViewTool,
                    onViewMemory = onViewMemory
                )
            }
        }
    }
}

@Composable
private fun MessageDetailContent(
    data: MessageDetailData,
    onViewTool: (String) -> Unit,
    onViewMemory: (String) -> Unit
) {
    val message = data.message
    val isUser = message.role == "user"
    val sender = if (isUser) MessageSender.User else MessageSender.Character

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        item(key = "message_card") {
            AmitiaSection(title = "消息内容") {
                ChatBubble(
                    text = message.content,
                    sender = sender,
                    timestamp = message.createdAt,
                    status = MessageStatus.Sent,
                    modifier = Modifier.padding(AmitiaSpacing.Sm)
                )
            }
        }
        item(key = "message_info") {
            AmitiaSection(title = "消息信息") {
                AmitiaContentSurface {
                    Column {
                        InfoRow(label = "角色", value = data.characterName)
                        InfoRow(label = "渠道", value = data.channel)
                        InfoRow(label = "消息ID", value = message.id)
                        InfoRow(label = "内容类型", value = message.contentType)
                    }
                }
            }
        }
        if (data.toolExecutions.isNotEmpty()) {
            item(key = "tool_header") {
                AmitiaSectionHeader(title = "工具执行 (${data.toolExecutions.size})")
            }
            items(data.toolExecutions.size, key = { data.toolExecutions[it].id }) { index ->
                val tool = data.toolExecutions[index]
                ToolExecutionCard(
                    toolName = tool.toolName,
                    status = tool.status,
                    input = tool.inputSummary,
                    output = tool.outputSummary,
                    duration = tool.duration,
                    modifier = Modifier.padding(bottom = AmitiaSpacing.Sm)
                )
                TertiaryButton(
                    text = "查看工具详情",
                    onClick = { onViewTool(tool.id) },
                    leadingIcon = AmitiaIcons.ChevronRight
                )
            }
        }
        if (data.relatedMemories.isNotEmpty()) {
            item(key = "memory_header") {
                AmitiaSectionHeader(title = "关联记忆 (${data.relatedMemories.size})")
            }
            items(data.relatedMemories.size, key = { "mem_$it" }) { index ->
                val memoryTitle = data.relatedMemories[index]
                MemoryReferenceCard(
                    title = memoryTitle,
                    preview = "点击查看记忆详情",
                    relevance = 0.9f - index * 0.1f,
                    onClick = { onViewMemory("mem_$index") }
                )
            }
        }
        if (data.referencedBy.isNotEmpty()) {
            item(key = "referenced_section") {
                AmitiaSection(title = "被以下消息引用") {
                    AmitiaContentSurface {
                        Column {
                            data.referencedBy.forEachIndexed { index, ref ->
                                Row(
                                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Box(
                                        modifier = Modifier.size(AmitiaIconSize.Medium).clip(CircleShape)
                                            .background(MaterialTheme.colorScheme.surfaceVariant),
                                        contentAlignment = Alignment.Center
                                    ) {
                                        Icon(
                                            imageVector = AmitiaIcons.ChatBubbleOutline,
                                            contentDescription = null,
                                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                            modifier = Modifier.size(AmitiaIconSize.Small)
                                        )
                                    }
                                    Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                                    Text(
                                        text = ref,
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurface,
                                        maxLines = 2,
                                        overflow = TextOverflow.Ellipsis
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.weight(1f))
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Preview(name = "Message Detail - Light", showBackground = true)
@Composable
private fun MessageDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            MessageDetailContent(
                data = MessageDetailData(
                    message = com.amitia.core.model.MessageDto(
                        id = "m1", role = "assistant",
                        content = "这是一条测试消息，用于展示消息详情功能。",
                        channel = "web", createdAt = "2026-07-27T14:30:00"
                    ),
                    channel = "Web 对话",
                    characterName = "艾米",
                    referencedBy = listOf("后续追问消息"),
                    relatedMemories = listOf("用户偏好"),
                    toolExecutions = emptyList()
                ),
                onViewTool = {},
                onViewMemory = {}
            )
        }
    }
}

@Preview(name = "Message Detail - Dark", showBackground = true)
@Composable
private fun MessageDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            InfoRow(label = "角色", value = "艾米")
        }
    }
}
