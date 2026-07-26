package com.amitia.feature.chat.audio

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Mic
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors

@Composable
fun VoiceRecorderButton(
    recording: Boolean,
    durationSec: Int,
    onRecordStart: () -> Unit,
    onRecordEnd: (released: Boolean) -> Unit,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier
) {
    var draggedOut by remember { mutableStateOf(false) }

    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .pointerInput(Unit) {
                    detectDragGestures(
                        onDragStart = {
                            draggedOut = false
                            onRecordStart()
                        },
                        onDragEnd = {
                            onRecordEnd(!draggedOut)
                            draggedOut = false
                        },
                        onDragCancel = {
                            onCancel()
                            draggedOut = false
                        },
                        onDrag = { change, dragAmount ->
                            change.consume()
                            if (dragAmount.y < -40f) {
                                draggedOut = true
                            }
                        }
                    )
                }
                .background(
                    color = if (recording) {
                        MaterialTheme.colorScheme.error
                    } else {
                        MaterialTheme.colorScheme.primaryContainer
                    },
                    shape = CircleShape
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = Icons.Outlined.Mic,
                contentDescription = "录音",
                tint = if (recording) {
                    MaterialTheme.colorScheme.onError
                } else {
                    MaterialTheme.colorScheme.onPrimaryContainer
                }
            )
        }
        Spacer(modifier = Modifier.height(4.dp))
        Text(
            text = if (recording) {
                if (draggedOut) "松开取消" else "${durationSec}s · 上滑取消"
            } else {
                "长按录音"
            },
            style = MaterialTheme.typography.labelSmall,
            color = AmitiaColors.OnSurfaceMuted
        )
    }
}
