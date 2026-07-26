package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.GraphicEq
import androidx.compose.material.icons.outlined.Refresh
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
    val alignment = if (isUser) Alignment.End else Alignment.Start
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

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 6.dp),
        horizontalAlignment = alignment
    ) {
        Row(
            verticalAlignment = Alignment.Top,
            horizontalArrangement = if (isUser) Arrangement.End else Arrangement.Start,
            modifier = Modifier.fillMaxWidth()
        ) {
            if (!isUser) {
                Box(
                    modifier = Modifier
                        .size(28.dp)
                        .clip(RoundedCornerShape(14.dp))
                        .background(MaterialTheme.colorScheme.tertiaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = Icons.Outlined.Description,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onTertiaryContainer,
                        modifier = Modifier.size(16.dp)
                    )
                }
                Spacer(modifier = Modifier.size(8.dp))
            }
            Column(
                modifier = Modifier.widthIn(max = 280.dp),
                horizontalAlignment = alignment
            ) {
                when (message.contentType) {
                    "image" -> ImageBubble(message = message)
                    "audio" -> AudioBubble(
                        message = message,
                        onPlayAudio = onPlayAudio
                    )
                    "system" -> SystemBubble(message = message)
                    else -> TextBubble(
                        message = message,
                        color = bubbleColor,
                        textColor = bubbleTextColor,
                        isUser = isUser
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
            if (isUser) {
                Spacer(modifier = Modifier.size(8.dp))
            }
        }
    }
}

@Composable
private fun TextBubble(
    message: MessageDto,
    color: androidx.compose.ui.graphics.Color,
    textColor: androidx.compose.ui.graphics.Color,
    isUser: Boolean
) {
    Surface(
        color = color,
        shape = RoundedCornerShape(
            topStart = 16.dp,
            topEnd = 16.dp,
            bottomStart = if (isUser) 16.dp else 4.dp,
            bottomEnd = if (isUser) 4.dp else 16.dp
        ),
        tonalElevation = 0.dp
    ) {
        Column(modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)) {
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
                    color = textColor
                )
            }
        }
    }
}

@Composable
private fun ImageBubble(message: MessageDto) {
    val context = LocalContext.current
    val url = message.imageUrl
    if (url.isNullOrBlank()) return
    AsyncImage(
        model = ImageRequest.Builder(context).data(url).build(),
        contentDescription = "图片消息",
        modifier = Modifier
            .size(180.dp)
            .clip(RoundedCornerShape(16.dp))
    )
}

@Composable
private fun AudioBubble(
    message: MessageDto,
    onPlayAudio: (String) -> Unit
) {
    val url = message.audioUrl ?: return
    val duration = message.duration?.toInt() ?: 0
    Surface(
        color = MaterialTheme.colorScheme.surfaceVariant,
        shape = RoundedCornerShape(16.dp),
        tonalElevation = 0.dp
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = { onPlayAudio(url) }) {
                Icon(
                    imageVector = Icons.Outlined.GraphicEq,
                    contentDescription = "播放",
                    tint = MaterialTheme.colorScheme.primary
                )
            }
            Spacer(modifier = Modifier.size(8.dp))
            Column {
                Text(
                    text = "语音消息",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface
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
                    imageVector = Icons.Outlined.ContentCopy,
                    contentDescription = "复制",
                    tint = AmitiaColors.OnSurfaceMuted,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        if (message.status == "failed") {
            IconButton(onClick = onRetry, modifier = Modifier.size(24.dp)) {
                Icon(
                    imageVector = Icons.Outlined.Refresh,
                    contentDescription = "重试",
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(14.dp)
                )
            }
        }
        IconButton(onClick = onDelete, modifier = Modifier.size(24.dp)) {
            Icon(
                imageVector = Icons.Outlined.Delete,
                contentDescription = "删除",
                tint = AmitiaColors.OnSurfaceMuted,
                modifier = Modifier.size(14.dp)
            )
        }
    }
}
