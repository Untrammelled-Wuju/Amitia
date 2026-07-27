package com.amitia.feature.chat

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaBottomSheet
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

enum class RecordState { Idle, Recording, Locked, Cancelling }

@Composable
fun VoiceRecordSheet(
    onDismiss: () -> Unit,
    onSend: (Long) -> Unit,
    onCancel: () -> Unit
) {
    var recordState by remember { mutableStateOf(RecordState.Idle) }
    var durationSeconds by remember { mutableFloatStateOf(0f) }
    var dragOffset by remember { mutableFloatStateOf(0f) }

    val infiniteTransition = rememberInfiniteTransition(label = "pulse")
    val pulseScale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = 1.15f,
        animationSpec = infiniteRepeatable(
            animation = tween(800),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseScale"
    )

    AmitiaBottomSheet(onDismiss = onDismiss, title = "语音录制") {
        Column(
            modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            RecordStatusText(state = recordState, durationSeconds = durationSeconds)

            WaveformDisplay(
                isRecording = recordState == RecordState.Recording || recordState == RecordState.Locked,
                modifier = Modifier.fillMaxWidth().height(80.dp)
            )

            RecordButton(
                state = recordState,
                pulseScale = pulseScale,
                dragOffset = dragOffset,
                onStateChanged = { recordState = it },
                onDragUpdate = { offset -> dragOffset = offset },
                onSend = {
                    onSend((durationSeconds * 1000).toLong())
                    durationSeconds = 0f
                    recordState = RecordState.Idle
                    onDismiss()
                },
                onCancel = {
                    durationSeconds = 0f
                    recordState = RecordState.Idle
                    onCancel()
                    onDismiss()
                }
            )

            RecordHintText(state = recordState)

            AnimatedVisibility(
                visible = recordState == RecordState.Locked,
                enter = fadeIn() + slideInVertically(),
                exit = fadeOut() + slideOutVertically()
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    SecondaryButton(
                        text = "取消",
                        onClick = {
                            durationSeconds = 0f
                            recordState = RecordState.Idle
                            onCancel()
                            onDismiss()
                        },
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Close
                    )
                    PrimaryButton(
                        text = "发送",
                        onClick = {
                            onSend((durationSeconds * 1000).toLong())
                            durationSeconds = 0f
                            recordState = RecordState.Idle
                            onDismiss()
                        },
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Send
                    )
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        }
    }
}

@Composable
private fun RecordStatusText(state: RecordState, durationSeconds: Float) {
    val message = when (state) {
        RecordState.Idle -> "按住录音按钮开始录制"
        RecordState.Recording -> "正在录音...松开发送，上滑取消"
        RecordState.Locked -> "录音已锁定，点击发送或取消"
        RecordState.Cancelling -> "松开手指取消录音"
    }
    val minutes = (durationSeconds / 60).toInt()
    val seconds = (durationSeconds % 60).toInt()
    val timeText = if (state != RecordState.Idle) {
        String.format("%01d:%02d", minutes, seconds)
    } else null

    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        if (timeText != null) {
            Text(
                text = timeText,
                style = MaterialTheme.typography.displaySmall,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
        Text(
            text = message,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center
        )
    }
}

@Composable
private fun WaveformDisplay(
    isRecording: Boolean,
    modifier: Modifier = Modifier
) {
    val barCount = 40
    val infiniteTransition = rememberInfiniteTransition(label = "waveform")
    val bars = remember(barCount) { List(barCount) { it } }

    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically
    ) {
        bars.forEach { index ->
            val animatedHeight by infiniteTransition.animateFloat(
                initialValue = if (isRecording) 0.2f else 0.1f,
                targetValue = if (isRecording) 1f else 0.2f,
                animationSpec = infiniteRepeatable(
                    animation = tween(300 + (index % 5) * 50),
                    repeatMode = RepeatMode.Reverse,
                    initialStartOffset = androidx.compose.animation.core.StartOffset(index * 30)
                ),
                label = "bar_$index"
            )
            Box(
                modifier = Modifier
                    .padding(horizontal = 1.dp)
                    .width(3.dp)
                    .height((animatedHeight * 60).dp)
                    .clip(CircleShape)
                    .background(
                        if (isRecording) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant
                    )
            )
        }
    }
}

@Composable
private fun RecordButton(
    state: RecordState,
    pulseScale: Float,
    dragOffset: Float,
    onStateChanged: (RecordState) -> Unit,
    onDragUpdate: (Float) -> Unit,
    onSend: () -> Unit,
    onCancel: () -> Unit
) {
    val buttonColor = when (state) {
        RecordState.Idle -> MaterialTheme.colorScheme.primary
        RecordState.Recording -> AmitiaStateColors.Failed
        RecordState.Locked -> MaterialTheme.colorScheme.tertiary
        RecordState.Cancelling -> MaterialTheme.colorScheme.error
    }
    val iconSize = when (state) {
        RecordState.Idle -> AmitiaIconSize.Large
        else -> AmitiaIconSize.Medium
    }

    Box(
        modifier = Modifier
            .size(80.dp)
            .scale(if (state == RecordState.Recording) pulseScale else 1f)
            .clip(CircleShape)
            .background(buttonColor)
            .pointerInput(Unit) {
                detectDragGestures(
                    onDragStart = {
                        onStateChanged(RecordState.Recording)
                    },
                    onDrag = { change, dragAmount ->
                        change.consume()
                        val newY = dragOffset + dragAmount.y
                        onDragUpdate(newY)
                        if (newY < -100f) {
                            onStateChanged(RecordState.Cancelling)
                        } else if (state == RecordState.Cancelling && newY >= -100f) {
                            onStateChanged(RecordState.Recording)
                        }
                    },
                    onDragEnd = {
                        if (state == RecordState.Cancelling) {
                            onCancel()
                        } else {
                            onStateChanged(RecordState.Locked)
                        }
                        onDragUpdate(0f)
                    },
                    onDragCancel = {
                        onStateChanged(RecordState.Idle)
                        onDragUpdate(0f)
                    }
                )
            },
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = when (state) {
                RecordState.Idle -> AmitiaIcons.Mic
                RecordState.Cancelling -> AmitiaIcons.Close
                else -> AmitiaIcons.Stop
            },
            contentDescription = null,
            tint = Color.White,
            modifier = Modifier.size(iconSize)
        )
    }
}

@Composable
private fun RecordHintText(state: RecordState) {
    val hint = when (state) {
        RecordState.Idle -> "长按按钮录音，可上滑取消"
        RecordState.Recording -> "上滑取消，松开锁定"
        RecordState.Locked -> "录音已锁定"
        RecordState.Cancelling -> "松开取消录音"
    }
    Text(
        text = hint,
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
        textAlign = TextAlign.Center
    )
}

@Preview(name = "Voice Record - Light", showBackground = true)
@Composable
private fun VoiceRecordLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            RecordStatusText(state = RecordState.Recording, durationSeconds = 12.5f)
            WaveformDisplay(isRecording = true, modifier = Modifier.fillMaxWidth().height(80.dp))
            Box(
                modifier = Modifier.size(80.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Mic,
                    contentDescription = null,
                    tint = Color.White,
                    modifier = Modifier.size(AmitiaIconSize.Large)
                )
            }
        }
    }
}

@Preview(name = "Voice Record - Dark", showBackground = true)
@Composable
private fun VoiceRecordDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            RecordStatusText(state = RecordState.Locked, durationSeconds = 5.0f)
            WaveformDisplay(isRecording = true, modifier = Modifier.fillMaxWidth().height(80.dp))
        }
    }
}
