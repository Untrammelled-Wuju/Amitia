package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.GlassLevel
import kotlinx.coroutines.delay

@Composable
fun ChatInputBar(
    draft: String,
    sending: Boolean,
    generating: Boolean,
    onTextChanged: (String) -> Unit,
    onSend: () -> Unit,
    onPickImage: () -> Unit,
    onTakePhoto: () -> Unit,
    onStartRecording: () -> Unit,
    pendingImages: List<String> = emptyList(),
    recording: Boolean = false,
    recordingDuration: Int = 0
) {
    var localDraft by remember(draft) { mutableStateOf(draft) }
    LaunchedEffect(localDraft) {
        if (localDraft != draft) {
            delay(200)
            onTextChanged(localDraft)
        }
    }

    val canSend = (localDraft.isNotBlank() || pendingImages.isNotEmpty()) && !sending && !generating
    val gradientBrush = Brush.linearGradient(
        colors = listOf(
            MaterialTheme.colorScheme.primary,
            Color(0xFF91A697)
        )
    )

    AmitiaGlassSurface(
        level = GlassLevel.Navigation,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 12.dp)
            .imePadding()
            .navigationBarsPadding(),
        shape = RoundedCornerShape(24.dp)
    ) {
        Column(modifier = Modifier.padding(horizontal = 8.dp, vertical = 10.dp)) {
            if (pendingImages.isNotEmpty()) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 4.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    pendingImages.take(4).forEach { uri ->
                        Box(
                            modifier = Modifier.size(56.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            coil.compose.AsyncImage(
                                model = uri,
                                contentDescription = null,
                                modifier = Modifier.size(56.dp)
                            )
                        }
                    }
                    if (pendingImages.size > 4) {
                        Text(
                            text = "+${pendingImages.size - 4}",
                            style = MaterialTheme.typography.labelMedium,
                            color = AmitiaColors.OnSurfaceMuted
                        )
                    }
                }
            }
            if (recording) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(16.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Spacer(modifier = Modifier.size(8.dp))
                    Text(
                        text = "正在录音 ${recordingDuration}s",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            } else {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .defaultMinSize(minHeight = 62.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    DockIconButton(
                        icon = AmitiaIcons.Image,
                        contentDescription = "附件",
                        onClick = onPickImage
                    )
                    TextField(
                        value = localDraft,
                        onValueChange = { localDraft = it },
                        modifier = Modifier
                            .weight(1f)
                            .widthIn(max = 280.dp),
                        placeholder = {
                            Text(
                                text = if (generating) "生成中…" else "说点什么",
                                style = MaterialTheme.typography.bodyMedium,
                                color = AmitiaColors.OnSurfaceMuted
                            )
                        },
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Default),
                        textStyle = TextStyle(
                            fontSize = 15.sp,
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
                    DockIconButton(
                        icon = AmitiaIcons.Mic,
                        contentDescription = "语音",
                        onClick = onStartRecording
                    )
                    val sendInteractionSource = remember { MutableInteractionSource() }
                    Box(
                        modifier = Modifier
                            .size(42.dp)
                            .then(
                                if (canSend) {
                                    Modifier.shadow(
                                        elevation = 4.dp,
                                        shape = RoundedCornerShape(17.dp)
                                    )
                                } else {
                                    Modifier
                                }
                            )
                            .clip(RoundedCornerShape(17.dp))
                            .then(
                                if (canSend) {
                                    Modifier.background(gradientBrush)
                                } else {
                                    Modifier.background(MaterialTheme.colorScheme.surfaceVariant)
                                }
                            )
                            .clickable(
                                interactionSource = sendInteractionSource,
                                indication = null,
                                enabled = canSend,
                                role = Role.Button,
                                onClick = onSend
                            ),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Send,
                            contentDescription = "发送",
                            tint = if (canSend) Color.White else MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DockIconButton(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    contentDescription: String?,
    onClick: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    Box(
        modifier = Modifier
            .size(40.dp)
            .clip(RoundedCornerShape(AmitiaRadius.S))
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
            tint = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
    }
}
