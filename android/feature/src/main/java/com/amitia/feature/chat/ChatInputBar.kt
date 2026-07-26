package com.amitia.feature.chat

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CameraAlt
import androidx.compose.material.icons.outlined.GraphicEq
import androidx.compose.material.icons.outlined.Image
import androidx.compose.material.icons.outlined.Send
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors
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

    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .imePadding()
            .navigationBarsPadding(),
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = 0.dp,
        shape = RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp)
    ) {
        Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
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
                            modifier = Modifier
                                .size(56.dp),
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
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.Bottom,
                    horizontalArrangement = Arrangement.spacedBy(4.dp)
                ) {
                    IconButton(onClick = onPickImage) {
                        Icon(
                            imageVector = Icons.Outlined.Image,
                            contentDescription = "图片",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = onTakePhoto) {
                        Icon(
                            imageVector = Icons.Outlined.CameraAlt,
                            contentDescription = "相机",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    IconButton(onClick = onStartRecording) {
                        Icon(
                            imageVector = Icons.Outlined.GraphicEq,
                            contentDescription = "语音",
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    OutlinedTextField(
                        value = localDraft,
                        onValueChange = { localDraft = it },
                        modifier = Modifier
                            .weight(1f)
                            .widthIn(max = 240.dp)
                            .height(56.dp),
                        placeholder = {
                            Text(
                                text = if (generating) "生成中…" else "说点什么",
                                style = MaterialTheme.typography.bodyMedium,
                                color = AmitiaColors.OnSurfaceMuted
                            )
                        },
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Default),
                        shape = RoundedCornerShape(20.dp)
                    )
                    IconButton(
                        onClick = onSend,
                        enabled = (localDraft.isNotBlank() || pendingImages.isNotEmpty()) && !sending && !generating
                    ) {
                        Icon(
                            imageVector = Icons.Outlined.Send,
                            contentDescription = "发送",
                            tint = if (localDraft.isBlank() && pendingImages.isEmpty()) {
                                MaterialTheme.colorScheme.onSurfaceVariant
                            } else {
                                MaterialTheme.colorScheme.primary
                            }
                        )
                    }
                }
            }
        }
    }
}
