package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTransformGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import coil.request.ImageRequest
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun ImagePreviewScreen(
    imageUrl: String,
    title: String,
    onBack: () -> Unit,
    onSave: () -> Unit,
    onShare: () -> Unit,
    onViewOriginal: () -> Unit
) {
    val context = LocalContext.current
    var scale by remember { mutableFloatStateOf(1f) }
    var offsetX by remember { mutableFloatStateOf(0f) }
    var offsetY by remember { mutableFloatStateOf(0f) }
    var showControls by remember { mutableStateOf(true) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .pointerInput(Unit) {
                detectTransformGestures { _, pan, zoom, _ ->
                    scale = (scale * zoom).coerceIn(1f, 5f)
                    if (scale > 1f) {
                        offsetX += pan.x
                        offsetY += pan.y
                    } else {
                        offsetX = 0f
                        offsetY = 0f
                    }
                }
            }
    ) {
        AsyncImage(
            model = ImageRequest.Builder(context).data(imageUrl).build(),
            contentDescription = title,
            modifier = Modifier
                .fillMaxSize()
                .graphicsLayer(
                    scaleX = scale,
                    scaleY = scale,
                    translationX = offsetX,
                    translationY = offsetY
                )
                .pointerInput(Unit) {
                    detectTransformGestures(
                        onGesture = { _, pan, zoom, _ ->
                            if (zoom != 1f) showControls = !showControls
                        }
                    )
                }
        )

        if (showControls) {
            ImagePreviewTopBar(
                title = title,
                onBack = onBack
            )
            ImagePreviewBottomBar(
                onSave = onSave,
                onShare = onShare,
                onViewOriginal = onViewOriginal
            )
        }
    }
}

@Composable
private fun ImagePreviewTopBar(
    title: String,
    onBack: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        color = Color.Black.copy(alpha = 0.6f)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaIconButton(
                icon = AmitiaIcons.Close,
                contentDescription = "关闭",
                onClick = onBack,
                tint = Color.White
            )
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium,
                color = Color.White,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
                fontWeight = FontWeight.Medium
            )
        }
    }
}

@Composable
private fun ImagePreviewBottomBar(
    onSave: () -> Unit,
    onShare: () -> Unit,
    onViewOriginal: () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize(),
        contentAlignment = Alignment.BottomCenter
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .padding(AmitiaSpacing.Base),
            shape = androidx.compose.foundation.shape.RoundedCornerShape(24.dp),
            color = Color.Black.copy(alpha = 0.6f)
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                horizontalArrangement = Arrangement.SpaceEvenly,
                verticalAlignment = Alignment.CenterVertically
            ) {
                BottomAction(icon = AmitiaIcons.Download, label = "保存", onClick = onSave)
                BottomAction(icon = AmitiaIcons.Share, label = "分享", onClick = onShare)
                BottomAction(icon = AmitiaIcons.ChatBubble, label = "原消息", onClick = onViewOriginal)
            }
        }
    }
}

@Composable
private fun BottomAction(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    onClick: () -> Unit
) {
    val interactionSource = remember { androidx.compose.foundation.interaction.MutableInteractionSource() }
    Column(
        modifier = Modifier
            .clip(CircleShape)
            .androidx_clickable(interactionSource, onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        Icon(
            imageVector = icon,
            contentDescription = label,
            tint = Color.White,
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = Color.White
        )
    }
}

private fun Modifier.androidx_clickable(
    source: androidx.compose.foundation.interaction.MutableInteractionSource,
    onClick: () -> Unit
): Modifier = this.then(
    this.clickable(
        interactionSource = source,
        indication = null,
        role = androidx.compose.ui.semantics.Role.Button,
        onClick = onClick
    )
)

@Preview(name = "Image Preview - Light", showBackground = true)
@Composable
private fun ImagePreviewLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.background(Color.Black).padding(AmitiaSpacing.Base)) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Icon(
                    imageVector = AmitiaIcons.Image,
                    contentDescription = null,
                    tint = Color.White,
                    modifier = Modifier.size(120.dp)
                )
                Text(text = "图片预览", color = Color.White, style = MaterialTheme.typography.titleMedium)
            }
        }
    }
}

@Preview(name = "Image Preview - Dark", showBackground = true)
@Composable
private fun ImagePreviewDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            TertiaryButton(text = "保存", onClick = {})
            TertiaryButton(text = "分享", onClick = {})
        }
    }
}
