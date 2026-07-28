package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import coil.request.ImageRequest
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.model.MessageDto

@Composable
fun MessageBubble(
    message: MessageDto,
    onRetry: () -> Unit,
    onDelete: () -> Unit,
    onCopy: () -> Unit,
    onPlayAudio: (String) -> Unit
) {
    val isUser = message.role == "user"

    BoxWithConstraints(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = 6.dp)
    ) {
        val bubbleMaxWidth = maxWidth * 0.82f

        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
            verticalAlignment = Alignment.Top
        ) {
            if (!isUser) {
                Box(
                    modifier = Modifier
                        .size(54.dp)
                        .clip(RoundedCornerShape(19.dp))
                        .background(MaterialTheme.colorScheme.tertiaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.SmartToy,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onTertiaryContainer,
                        modifier = Modifier.size(AmitiaIconSize.Nav)
                    )
                }
                Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                Column(
                    modifier = Modifier.widthIn(max = bubbleMaxWidth),
                    horizontalAlignment = Alignment.Start
                ) {
                    MessageBubbleContent(
                        message = message,
                        isUser = isUser,
                        onRetry = onRetry,
                        onDelete = onDelete,
                        onCopy = onCopy,
                        onPlayAudio = onPlayAudio
                    )
                }
            } else {
                Spacer(modifier = Modifier.weight(1f))
                Column(
                    modifier = Modifier.widthIn(max = bubbleMaxWidth),
                    horizontalAlignment = Alignment.End
                ) {
                    MessageBubbleContent(
                        message = message,
                        isUser = isUser,
                        onRetry = onRetry,
                        onDelete = onDelete,
                        onCopy = onCopy,
                        onPlayAudio = onPlayAudio
                    )
                }
                Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .clip(RoundedCornerShape(AmitiaRadius.S))
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
            }
        }
    }
}

@Composable
private fun MessageBubbleContent(
    message: MessageDto,
    isUser: Boolean,
    onRetry: () -> Unit,
    onDelete: () -> Unit,
    onCopy: () -> Unit,
    onPlayAudio: (String) -> Unit
) {
    val bubbleShape = if (isUser) {
        RoundedCornerShape(
            topStart = AmitiaRadius.M,
            topEnd = AmitiaRadius.M,
            bottomStart = AmitiaRadius.M,
            bottomEnd = 7.dp
        )
    } else {
        RoundedCornerShape(
            topStart = 7.dp,
            topEnd = AmitiaRadius.M,
            bottomStart = AmitiaRadius.M,
            bottomEnd = AmitiaRadius.M
        )
    }

    when (message.contentType) {
        "image" -> ImageBubble(message = message, isUser = isUser, bubbleShape = bubbleShape)
        "audio" -> AudioBubble(
            message = message,
            onPlayAudio = onPlayAudio,
            isUser = isUser,
            bubbleShape = bubbleShape
        )
        "system" -> SystemBubble(message = message)
        else -> TextBubble(
            message = message,
            isUser = isUser,
            bubbleShape = bubbleShape
        )
    }
    if (message.status == "failed") {
        Text(
            text = "发送失败",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.error,
            modifier = Modifier.padding(top = 4.dp, end = 4.dp)
        )
    }
    MessageActionRow(
        message = message,
        onRetry = onRetry,
        onDelete = onDelete,
        onCopy = onCopy
    )
}

@Composable
private fun TextBubble(
    message: MessageDto,
    isUser: Boolean,
    bubbleShape: androidx.compose.ui.graphics.Shape
) {
    val bubbleColor = if (isUser) {
        MaterialTheme.colorScheme.primaryContainer
    } else {
        MaterialTheme.colorScheme.surface
    }
    val bubbleTextColor = if (isUser) {
        MaterialTheme.colorScheme.onPrimaryContainer
    } else {
        MaterialTheme.colorScheme.onSurface
    }
    Surface(
        color = bubbleColor,
        shape = bubbleShape,
        tonalElevation = 0.dp,
        modifier = if (!isUser) {
            Modifier.border(1.dp, MaterialTheme.colorScheme.outline, bubbleShape)
        } else {
            Modifier
        }
    ) {
        Column(modifier = Modifier.padding(horizontal = 14.dp, vertical = 12.dp)) {
            if (message.content.isBlank() && message.status == "streaming") {
                Text(
                    text = "正在生成…",
                    style = MaterialTheme.typography.bodyMedium,
                    color = AmitiaColors.OnSurfaceMuted,
                    fontWeight = FontWeight.Light
                )
            } else {
                Text(
                    text = message.content,
                    style = MaterialTheme.typography.bodyMedium,
                    color = bubbleTextColor
                )
            }
        }
    }
}

@Composable
private fun ImageBubble(
    message: MessageDto,
    isUser: Boolean,
    bubbleShape: androidx.compose.ui.graphics.Shape
) {
    val context = LocalContext.current
    val url = message.imageUrl
    if (url.isNullOrBlank()) return
    val imageShape = if (isUser) {
        RoundedCornerShape(
            topStart = AmitiaRadius.M,
            topEnd = AmitiaRadius.M,
            bottomStart = AmitiaRadius.M,
            bottomEnd = 7.dp
        )
    } else {
        RoundedCornerShape(
            topStart = 7.dp,
            topEnd = AmitiaRadius.M,
            bottomStart = AmitiaRadius.M,
            bottomEnd = AmitiaRadius.M
        )
    }
    AsyncImage(
        model = ImageRequest.Builder(context).data(url).build(),
        contentDescription = "图片消息",
        modifier = Modifier
            .size(180.dp)
            .clip(imageShape)
    )
}

@Composable
private fun AudioBubble(
    message: MessageDto,
    onPlayAudio: (String) -> Unit,
    isUser: Boolean,
    bubbleShape: androidx.compose.ui.graphics.Shape
) {
    val url = message.audioUrl ?: return
    val duration = message.duration?.toInt() ?: 0
    val bubbleColor = if (isUser) {
        MaterialTheme.colorScheme.primaryContainer
    } else {
        MaterialTheme.colorScheme.surface
    }
    val onColor = if (isUser) {
        MaterialTheme.colorScheme.onPrimaryContainer
    } else {
        MaterialTheme.colorScheme.onSurface
    }
    Surface(
        color = bubbleColor,
        shape = bubbleShape,
        tonalElevation = 0.dp,
        modifier = if (!isUser) {
            Modifier.border(1.dp, MaterialTheme.colorScheme.outline, bubbleShape)
        } else {
            Modifier
        }
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = { onPlayAudio(url) }) {
                Icon(
                    imageVector = AmitiaIcons.GraphicEq,
                    contentDescription = "播放",
                    tint = MaterialTheme.colorScheme.primary
                )
            }
            Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
            Column {
                Text(
                    text = "语音消息",
                    style = MaterialTheme.typography.bodySmall,
                    color = onColor
                )
                if (duration > 0) {
                    Text(
                        text = "${duration}s",
                        style = MaterialTheme.typography.labelSmall,
                        color = AmitiaColors.OnSurfaceMuted
                    )
                }
            }
        }
    }
}

@Composable
private fun SystemBubble(message: MessageDto) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = message.content,
            style = MaterialTheme.typography.labelSmall,
            color = AmitiaColors.OnSurfaceMuted,
            modifier = Modifier
                .background(
                    color = MaterialTheme.colorScheme.surfaceVariant,
                    shape = RoundedCornerShape(8.dp)
                )
                .padding(horizontal = 8.dp, vertical = 2.dp),
            maxLines = 2,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Composable
private fun MessageActionRow(
    message: MessageDto,
    onRetry: () -> Unit,
    onDelete: () -> Unit,
    onCopy: () -> Unit
) {
    if (message.status == "streaming" || message.role == "system") return
    Row(
        modifier = Modifier.padding(top = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(0.dp)
    ) {
        if (message.content.isNotBlank()) {
            IconButton(onClick = onCopy, modifier = Modifier.size(24.dp)) {
                Icon(
                    imageVector = AmitiaIcons.ContentCopy,
                    contentDescription = "复制",
                    tint = AmitiaColors.OnSurfaceMuted,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        if (message.status == "failed") {
            IconButton(onClick = onRetry, modifier = Modifier.size(24.dp)) {
                Icon(
                    imageVector = AmitiaIcons.Refresh,
                    contentDescription = "重试",
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        IconButton(onClick = onDelete, modifier = Modifier.size(24.dp)) {
            Icon(
                imageVector = AmitiaIcons.Delete,
                contentDescription = "删除",
                tint = AmitiaColors.OnSurfaceMuted,
                modifier = Modifier.size(14.dp)
            )
        }
    }
}
