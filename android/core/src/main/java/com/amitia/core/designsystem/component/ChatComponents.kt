package com.amitia.core.designsystem.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
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
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaChatDockShape
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaNavDimensions
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget
import com.amitia.core.designsystem.GlassLevel

enum class MessageSender {
    User, Character
}

enum class MessageStatus {
    Sending, Sent, Failed, AiThinking, Streaming, ToolExecuting, Cancelled
}

@Composable
fun ChatBubble(
    text: String,
    sender: MessageSender,
    modifier: Modifier = Modifier,
    timestamp: String? = null,
    status: MessageStatus? = null
) {
    val isUser = sender == MessageSender.User
    val bubbleColor = if (isUser) MaterialTheme.colorScheme.surfaceVariant
    else MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    val textColor = if (isUser) MaterialTheme.colorScheme.onSurface
    else MaterialTheme.colorScheme.onSurface
    val bubbleShape = if (isUser) {
        RoundedCornerShape(
            topStart = AmitiaRadius.L,
            topEnd = AmitiaRadius.L,
            bottomStart = AmitiaRadius.L,
            bottomEnd = AmitiaRadius.Xs
        )
    } else {
        RoundedCornerShape(
            topStart = AmitiaRadius.L,
            topEnd = AmitiaRadius.L,
            bottomStart = AmitiaRadius.Xs,
            bottomEnd = AmitiaRadius.L
        )
    }
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
        verticalAlignment = Alignment.Bottom
    ) {
        Box(
            modifier = Modifier
                .widthIn(max = 320.dp)
                .clip(bubbleShape)
                .background(bubbleColor)
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                Text(
                    text = text,
                    style = MaterialTheme.typography.bodyMedium,
                    color = textColor
                )
                if (timestamp != null || status != null) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        if (timestamp != null) {
                            Text(
                                text = timestamp,
                                style = MaterialTheme.typography.labelSmall,
                                color = textColor.copy(alpha = 0.5f)
                            )
                        }
                        if (status != null) {
                            MessageStatusIndicator(
                                status = status,
                                tint = textColor.copy(alpha = 0.5f)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun MessageGroup(
    sender: MessageSender,
    modifier: Modifier = Modifier,
    senderName: String? = null,
    avatarContent: @Composable (() -> Unit)? = null,
    messages: List<@Composable () -> Unit>
) {
    val isUser = sender == MessageSender.User
    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        if (senderName != null) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
                verticalAlignment = Alignment.CenterVertically
            ) {
                if (!isUser && avatarContent != null) {
                    avatarContent()
                    Spacer(modifier = Modifier.width(AmitiaSpacing.Xs))
                }
                Text(
                    text = senderName,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (isUser && avatarContent != null) {
                    Spacer(modifier = Modifier.width(AmitiaSpacing.Xs))
                    avatarContent()
                }
            }
        }
        messages.forEach { it() }
    }
}

@Composable
fun MessageStatusIndicator(
    status: MessageStatus,
    modifier: Modifier = Modifier,
    tint: Color = MaterialTheme.colorScheme.onSurfaceVariant
) {
    when (status) {
        MessageStatus.Sending -> {
            CircularProgressIndicator(
                modifier = modifier.size(AmitiaIconSize.Small),
                strokeWidth = 1.5.dp,
                color = tint
            )
        }
        MessageStatus.Sent -> {
            Icon(
                imageVector = AmitiaIcons.Check,
                contentDescription = "已发送",
                tint = tint,
                modifier = modifier.size(AmitiaIconSize.Small)
            )
        }
        MessageStatus.Failed -> {
            Icon(
                imageVector = AmitiaIcons.Error,
                contentDescription = "发送失败",
                tint = MaterialTheme.colorScheme.error,
                modifier = modifier.size(AmitiaIconSize.Small)
            )
        }
        MessageStatus.AiThinking -> {
            TypingIndicator(
                modifier = modifier,
                tint = tint
            )
        }
        MessageStatus.Streaming -> {
            val infiniteTransition = rememberInfiniteTransition(label = "streaming")
            val alpha by infiniteTransition.animateFloat(
                initialValue = 0.3f,
                targetValue = 1f,
                animationSpec = infiniteRepeatable(
                    animation = tween(500),
                    repeatMode = RepeatMode.Reverse
                ),
                label = "streamingAlpha"
            )
            Box(
                modifier = modifier
                    .size(AmitiaIconSize.Small)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary.copy(alpha = alpha))
            )
        }
        MessageStatus.ToolExecuting -> {
            Row(
                modifier = modifier,
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Build,
                    contentDescription = "工具执行中",
                    tint = tint,
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
                Text(
                    text = "执行中",
                    style = MaterialTheme.typography.labelSmall,
                    color = tint
                )
            }
        }
        MessageStatus.Cancelled -> {
            Icon(
                imageVector = AmitiaIcons.Stop,
                contentDescription = "已取消",
                tint = tint,
                modifier = modifier.size(AmitiaIconSize.Small)
            )
        }
    }
}

@Composable
fun ToolExecutionCard(
    toolName: String,
    status: MessageStatus,
    modifier: Modifier = Modifier,
    input: String? = null,
    output: String? = null,
    duration: String? = null
) {
    val statusColor = when (status) {
        MessageStatus.Sent -> AmitiaStateColors.Running
        MessageStatus.Failed -> MaterialTheme.colorScheme.error
        MessageStatus.ToolExecuting -> MaterialTheme.colorScheme.primary
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(AmitiaRadius.M),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Terminal,
                    contentDescription = null,
                    tint = statusColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
                Text(
                    text = toolName,
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (duration != null) {
                    Text(
                        text = duration,
                        style = MaterialTheme.typography.labelSmall,
                        color = statusColor
                    )
                }
                MessageStatusIndicator(status = status, tint = statusColor)
            }
            if (input != null) {
                Text(
                    text = input,
                    style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis
                )
            }
            if (output != null) {
                Text(
                    text = output,
                    style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 5,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
fun MemoryReferenceCard(
    title: String,
    preview: String,
    modifier: Modifier = Modifier,
    relevance: Float? = null,
    onClick: (() -> Unit)? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .then(
                if (onClick != null) {
                    Modifier.clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = onClick
                    )
                } else Modifier
            ),
        shape = RoundedCornerShape(AmitiaRadius.M),
        color = MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            verticalAlignment = Alignment.Top,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Memory,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSecondaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = preview,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
            if (relevance != null) {
                Text(
                    text = "${(relevance * 100).toInt()}%",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSecondaryContainer
                )
            }
        }
    }
}

@Composable
fun ReasoningSummaryCard(
    summary: String,
    modifier: Modifier = Modifier,
    isExpanded: Boolean = false,
    onToggle: (() -> Unit)? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .fillMaxWidth()
            .then(
                if (onToggle != null) {
                    Modifier.clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = onToggle
                    )
                } else Modifier
            ),
        shape = RoundedCornerShape(AmitiaRadius.M),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.4f)
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Psychology,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
                Text(
                    text = "推理过程",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.weight(1f))
                if (onToggle != null) {
                    Icon(
                        imageVector = if (isExpanded) AmitiaIcons.ExpandLess else AmitiaIcons.ExpandMore,
                        contentDescription = if (isExpanded) "收起" else "展开",
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
            AnimatedVisibility(visible = isExpanded || onToggle == null) {
                Text(
                    text = summary,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = if (isExpanded) Int.MAX_VALUE else 3,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
fun AttachmentTile(
    fileName: String,
    modifier: Modifier = Modifier,
    fileSize: String? = null,
    icon: ImageVector = AmitiaIcons.AttachFile,
    onClick: (() -> Unit)? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = modifier
            .widthIn(max = 240.dp)
            .then(
                if (onClick != null) {
                    Modifier.clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = onClick
                    )
                } else Modifier
            ),
        shape = RoundedCornerShape(AmitiaRadius.M),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(AmitiaIconSize.Large)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = fileName,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (fileSize != null) {
                    Text(
                        text = fileSize,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                }
            }
        }
    }
}

@Composable
fun VoiceMessageBubble(
    duration: String,
    sender: MessageSender,
    modifier: Modifier = Modifier,
    isPlaying: Boolean = false,
    onPlayToggle: (() -> Unit)? = null,
    progress: Float = 0f
) {
    val isUser = sender == MessageSender.User
    val bubbleColor = if (isUser) MaterialTheme.colorScheme.surfaceVariant
    else MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.4f)
    val bubbleShape = if (isUser) {
        RoundedCornerShape(
            topStart = AmitiaRadius.L,
            topEnd = AmitiaRadius.L,
            bottomStart = AmitiaRadius.L,
            bottomEnd = AmitiaRadius.Xs
        )
    } else {
        RoundedCornerShape(
            topStart = AmitiaRadius.L,
            topEnd = AmitiaRadius.L,
            bottomStart = AmitiaRadius.Xs,
            bottomEnd = AmitiaRadius.L
        )
    }
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start
    ) {
        Row(
            modifier = Modifier
                .widthIn(max = 260.dp)
                .clip(bubbleShape)
                .background(bubbleColor)
                .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            if (onPlayToggle != null) {
                AmitiaIconButton(
                    icon = if (isPlaying) AmitiaIcons.Pause else AmitiaIcons.PlayArrow,
                    contentDescription = if (isPlaying) "暂停" else "播放",
                    onClick = onPlayToggle
                )
            }
            Waveform(
                progress = progress,
                modifier = Modifier.weight(1f),
                barCount = 24
            )
            Text(
                text = duration,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
fun ChatInputDock(
    text: String,
    onTextChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    onSend: (() -> Unit)? = null,
    onStop: (() -> Unit)? = null,
    onAttach: (() -> Unit)? = null,
    onVoice: (() -> Unit)? = null,
    isGenerating: Boolean = false,
    placeholder: String = "输入消息..."
) {
    AmitiaGlassSurface(
        level = GlassLevel.Navigation,
        modifier = modifier.fillMaxWidth(),
        shape = AmitiaChatDockShape
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.Bottom,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            if (onAttach != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.AttachFile,
                    contentDescription = "附件",
                    onClick = onAttach
                )
            }
            TextField(
                value = text,
                onValueChange = onTextChange,
                modifier = Modifier
                    .weight(1f)
                    .heightIn(min = 48.dp, max = 120.dp),
                placeholder = {
                    Text(
                        text = placeholder,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                },
                textStyle = MaterialTheme.typography.bodyMedium.copy(
                    color = MaterialTheme.colorScheme.onSurface
                ),
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
            if (isGenerating && onStop != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Stop,
                    contentDescription = "停止",
                    onClick = onStop,
                    tint = MaterialTheme.colorScheme.error
                )
            } else if (text.isNotBlank() && onSend != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Send,
                    contentDescription = "发送",
                    onClick = onSend,
                    tint = MaterialTheme.colorScheme.primary
                )
            } else if (onVoice != null) {
                AmitiaIconButton(
                    icon = AmitiaIcons.Mic,
                    contentDescription = "语音输入",
                    onClick = onVoice
                )
            }
        }
    }
}

@Composable
fun TypingIndicator(
    modifier: Modifier = Modifier,
    tint: Color = MaterialTheme.colorScheme.onSurfaceVariant
) {
    val infiniteTransition = rememberInfiniteTransition(label = "typing")
    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs),
        verticalAlignment = Alignment.CenterVertically
    ) {
        repeat(3) { index ->
            val delay = index * 150
            val alpha by infiniteTransition.animateFloat(
                initialValue = 0.3f,
                targetValue = 1f,
                animationSpec = infiniteRepeatable(
                    animation = tween(600, delayMillis = delay),
                    repeatMode = RepeatMode.Reverse
                ),
                label = "dot_$index"
            )
            Box(
                modifier = Modifier
                    .size(4.dp)
                    .clip(CircleShape)
                    .background(tint.copy(alpha = alpha))
            )
        }
    }
}

@Composable
fun StreamingMessage(
    text: String,
    sender: MessageSender,
    modifier: Modifier = Modifier,
    isComplete: Boolean = false
) {
    ChatBubble(
        text = if (isComplete) text else "$text",
        sender = sender,
        modifier = modifier,
        status = if (isComplete) MessageStatus.Sent else MessageStatus.Streaming
    )
}

@Preview(name = "Chat - Light", showBackground = true)
@Composable
private fun AmitiaChatLightPreview() {
    var inputText by remember { mutableStateOf("") }
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            ChatBubble(
                text = "你好，今天天气怎么样？",
                sender = MessageSender.User,
                timestamp = "14:30",
                status = MessageStatus.Sent
            )
            ChatBubble(
                text = "你好！让我帮你查看今天的天气情况。",
                sender = MessageSender.Character,
                timestamp = "14:30",
                status = MessageStatus.Streaming
            )
            ToolExecutionCard(
                toolName = "weather_query",
                status = MessageStatus.ToolExecuting,
                input = "city: 上海",
                duration = "0.3s"
            )
            MemoryReferenceCard(
                title = "用户位置偏好",
                preview = "用户通常关注上海的天气情况...",
                relevance = 0.92f
            )
            ReasoningSummaryCard(
                summary = "根据用户历史记录，用户关注上海天气，将查询上海今日天气数据并返回...",
                isExpanded = true,
                onToggle = {}
            )
            VoiceMessageBubble(
                duration = "0:15",
                sender = MessageSender.Character,
                progress = 0.5f,
                onPlayToggle = {}
            )
            AttachmentTile(
                fileName = "screenshot.png",
                fileSize = "1.2 MB"
            )
            Spacer(modifier = Modifier.weight(1f))
            ChatInputDock(
                text = inputText,
                onTextChange = { inputText = it },
                onSend = {},
                onAttach = {},
                onVoice = {},
                placeholder = "输入消息..."
            )
        }
    }
}

@Preview(name = "Chat - Dark", showBackground = true)
@Composable
private fun AmitiaChatDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            TypingIndicator()
            ChatBubble(
                text = "AI 正在思考...",
                sender = MessageSender.Character,
                status = MessageStatus.AiThinking
            )
        }
    }
}
