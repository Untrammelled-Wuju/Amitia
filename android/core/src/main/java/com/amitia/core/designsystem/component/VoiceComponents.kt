package com.amitia.core.designsystem.component

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaChatDockShape
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaMotionDuration
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget
import com.amitia.core.designsystem.GlassLevel
import com.amitia.core.designsystem.StandardEasing
import kotlin.random.Random

enum class VoiceCallStatus {
    Connecting, Active, Muted, Ended, Failed
}

enum class AudioDeviceType {
    Earpiece, Speakerphone, Bluetooth, WiredHeadset
}

data class AudioDeviceItem(
    val id: String,
    val name: String,
    val type: AudioDeviceType,
    val isConnected: Boolean = true
)

@Composable
fun VoiceOrb(
    modifier: Modifier = Modifier,
    isActive: Boolean = true,
    level: Float = 0f,
    onClick: (() -> Unit)? = null
) {
    val infiniteTransition = rememberInfiniteTransition(label = "voiceOrb")
    val breathScale by infiniteTransition.animateFloat(
        initialValue = 0.96f,
        targetValue = 1.04f,
        animationSpec = infiniteRepeatable(
            animation = tween(AmitiaMotionDuration.Character, easing = StandardEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "breathScale"
    )
    val glowAlpha by infiniteTransition.animateFloat(
        initialValue = 0.2f,
        targetValue = 0.5f,
        animationSpec = infiniteRepeatable(
            animation = tween(AmitiaMotionDuration.Immersive, easing = StandardEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "glowAlpha"
    )

    val orbSize = 120.dp
    val interactionSource = remember { MutableInteractionSource() }
    val levelScale = 1f + (level.coerceIn(0f, 1f) * 0.08f)
    val effectiveScale = if (isActive) breathScale * levelScale else 1f

    Box(
        modifier = modifier
            .size(orbSize)
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
        contentAlignment = Alignment.Center
    ) {
        if (isActive) {
            Box(
                modifier = Modifier
                    .size(orbSize)
                    .scale(effectiveScale * 1.35f)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary.copy(alpha = glowAlpha * 0.4f))
            )
        }
        if (isActive) {
            Box(
                modifier = Modifier
                    .size(orbSize)
                    .scale(effectiveScale * 1.18f)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary.copy(alpha = glowAlpha * 0.25f))
            )
        }
        Box(
            modifier = Modifier
                .size(orbSize * 0.8f)
                .scale(if (isActive) effectiveScale else 1f)
                .clip(CircleShape)
                .background(
                    if (isActive) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.surfaceVariant
                )
        )
        Icon(
            imageVector = if (isActive) AmitiaIcons.Mic else AmitiaIcons.MicOff,
            contentDescription = if (isActive) "录音中" else "麦克风关闭",
            tint = if (isActive) MaterialTheme.colorScheme.onPrimary
            else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(AmitiaIconSize.Huge)
        )
    }
}

@Composable
fun Waveform(
    progress: Float,
    modifier: Modifier = Modifier,
    barCount: Int = 32,
    isPlaying: Boolean = false,
    barWidth: Dp = 3.dp,
    barSpacing: Dp = 2.dp,
    maxHeight: Dp = 24.dp
) {
    val barHeights = remember(barCount) {
        List(barCount) { Random.nextFloat() * 0.7f + 0.3f }
    }

    val infiniteTransition = rememberInfiniteTransition(label = "waveform")
    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue = 0.65f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(400, easing = StandardEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseAlpha"
    )
    val waveOffset by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = barCount.toFloat(),
        animationSpec = infiniteRepeatable(
            animation = tween(2000, easing = StandardEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "waveOffset"
    )

    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(barSpacing),
        verticalAlignment = Alignment.CenterVertically
    ) {
        repeat(barCount) { index ->
            val baseHeight = barHeights[index]
            val dynamicHeight = if (isPlaying) {
                val wavePos = ((index + waveOffset) % barCount) / barCount
                val waveFactor = kotlin.math.sin(wavePos * Math.PI.toFloat() * 2).coerceIn(-1f, 1f)
                baseHeight * (1f + waveFactor * 0.2f)
            } else {
                baseHeight
            }
            val playedThreshold = progress * barCount
            val isPlayed = index < playedThreshold
            val alpha = if (isPlaying) pulseAlpha else 1f
            val barColor = if (isPlayed) MaterialTheme.colorScheme.primary.copy(alpha = alpha)
            else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.3f * alpha)

            Box(
                modifier = Modifier
                    .weight(1f)
                    .height(maxHeight * dynamicHeight)
                    .clip(RoundedCornerShape(barWidth / 2))
                    .background(barColor)
            )
        }
    }
}

@Composable
fun CallStatusChip(
    status: VoiceCallStatus,
    modifier: Modifier = Modifier,
    duration: String? = null
) {
    val icon = when (status) {
        VoiceCallStatus.Connecting -> AmitiaIcons.Sensors
        VoiceCallStatus.Active -> AmitiaIcons.Phone
        VoiceCallStatus.Muted -> AmitiaIcons.MicOff
        VoiceCallStatus.Ended -> AmitiaIcons.PhoneOff
        VoiceCallStatus.Failed -> AmitiaIcons.Error
    }
    val text = when (status) {
        VoiceCallStatus.Connecting -> "连接中..."
        VoiceCallStatus.Active -> if (duration != null) "通话中 · $duration" else "通话中"
        VoiceCallStatus.Muted -> "已静音"
        VoiceCallStatus.Ended -> "已结束"
        VoiceCallStatus.Failed -> "连接失败"
    }
    val color = when (status) {
        VoiceCallStatus.Connecting -> MaterialTheme.colorScheme.primary
        VoiceCallStatus.Active -> MaterialTheme.colorScheme.tertiary
        VoiceCallStatus.Muted -> MaterialTheme.colorScheme.onSurfaceVariant
        VoiceCallStatus.Ended -> MaterialTheme.colorScheme.onSurfaceVariant
        VoiceCallStatus.Failed -> MaterialTheme.colorScheme.error
    }

    AmitiaGlassSurface(
        level = GlassLevel.Chip,
        modifier = modifier
    ) {
        Row(
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            if (status == VoiceCallStatus.Connecting) {
                CircularProgressIndicator(
                    modifier = Modifier.size(AmitiaIconSize.Small),
                    strokeWidth = 1.5.dp,
                    color = color
                )
            } else {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = color,
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
            }
            Text(
                text = text,
                style = MaterialTheme.typography.labelMedium,
                color = color
            )
        }
    }
}

@Composable
fun MicLevelIndicator(
    level: Float,
    modifier: Modifier = Modifier,
    segmentCount: Int = 20
) {
    val clampedLevel = level.coerceIn(0f, 1f)
    val activeSegments = (clampedLevel * segmentCount).toInt()
    val isHigh = clampedLevel > 0.85f

    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs),
        verticalAlignment = Alignment.CenterVertically
    ) {
        repeat(segmentCount) { index ->
            val isActive = index < activeSegments
            val warningZone = index > segmentCount * 0.8
            val color = when {
                !isActive -> MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.15f)
                isHigh && warningZone -> MaterialTheme.colorScheme.error
                else -> MaterialTheme.colorScheme.primary
            }
            val barHeight = if (isActive) {
                val minH = 6.dp
                val maxH = 18.dp
                val ratio = (index + 1f) / segmentCount
                minH + (maxH - minH) * ratio
            } else {
                6.dp
            }
            Box(
                modifier = Modifier
                    .width(2.dp)
                    .height(barHeight)
                    .clip(RoundedCornerShape(1.dp))
                    .background(color)
            )
        }
    }
}

@Composable
fun VoiceControlDock(
    modifier: Modifier = Modifier,
    isMuted: Boolean = false,
    isSpeakerOn: Boolean = false,
    isCaptionOn: Boolean = false,
    onMuteToggle: () -> Unit = {},
    onSpeakerToggle: () -> Unit = {},
    onCaptionToggle: () -> Unit = {},
    onMore: () -> Unit = {},
    onEndCall: () -> Unit = {}
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
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically
        ) {
            AmitiaIconButton(
                icon = if (isMuted) AmitiaIcons.MicOff else AmitiaIcons.Mic,
                contentDescription = if (isMuted) "取消静音" else "静音",
                onClick = onMuteToggle,
                tint = if (isMuted) MaterialTheme.colorScheme.error
                else MaterialTheme.colorScheme.onSurface
            )
            AmitiaIconButton(
                icon = if (isSpeakerOn) AmitiaIcons.VolumeUp else AmitiaIcons.VolumeOff,
                contentDescription = if (isSpeakerOn) "关闭扬声器" else "开启扬声器",
                onClick = onSpeakerToggle,
                tint = if (isSpeakerOn) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant
            )
            AmitiaIconButton(
                icon = AmitiaIcons.ChatBubble,
                contentDescription = if (isCaptionOn) "关闭字幕" else "开启字幕",
                onClick = onCaptionToggle,
                tint = if (isCaptionOn) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant
            )
            AmitiaIconButton(
                icon = AmitiaIcons.MoreVert,
                contentDescription = "更多",
                onClick = onMore
            )
            val endInteractionSource = remember { MutableInteractionSource() }
            Box(
                modifier = Modifier
                    .size(AmitiaTouchTarget.Minimum)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.error)
                    .clickable(
                        interactionSource = endInteractionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = onEndCall
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.PhoneOff,
                    contentDescription = "结束通话",
                    tint = MaterialTheme.colorScheme.onError,
                    modifier = Modifier.size(AmitiaIconSize.Nav)
                )
            }
        }
    }
}

@Composable
fun AudioDevicePicker(
    expanded: Boolean,
    onDismiss: () -> Unit,
    devices: List<AudioDeviceItem>,
    selectedDeviceId: String?,
    onDeviceSelected: (AudioDeviceItem) -> Unit,
    modifier: Modifier = Modifier
) {
    DropdownMenu(
        expanded = expanded,
        onDismissRequest = onDismiss,
        modifier = modifier
    ) {
        devices.forEach { device ->
            DropdownMenuItem(
                text = {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = audioDeviceIcon(device.type),
                            contentDescription = null,
                            tint = if (device.id == selectedDeviceId)
                                MaterialTheme.colorScheme.primary
                            else MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = device.name,
                                style = MaterialTheme.typography.bodyMedium,
                                color = if (device.id == selectedDeviceId)
                                    MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurface,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                            if (!device.isConnected) {
                                Text(
                                    text = "未连接",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                                )
                            }
                        }
                    }
                },
                onClick = {
                    onDeviceSelected(device)
                    onDismiss()
                },
                trailingIcon = {
                    if (device.id == selectedDeviceId) {
                        Icon(
                            imageVector = AmitiaIcons.Check,
                            contentDescription = "已选择",
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                }
            )
        }
    }
}

private fun audioDeviceIcon(type: AudioDeviceType): ImageVector {
    return when (type) {
        AudioDeviceType.Earpiece -> AmitiaIcons.PhoneAndroid
        AudioDeviceType.Speakerphone -> AmitiaIcons.VolumeUp
        AudioDeviceType.Bluetooth -> AmitiaIcons.Sensors
        AudioDeviceType.WiredHeadset -> AmitiaIcons.Mic
    }
}

@Preview(name = "Voice - Light", showBackground = true)
@Composable
private fun AmitiaVoiceLightPreview() {
    var menuExpanded by remember { mutableStateOf(false) }
    val sampleDevices = listOf(
        AudioDeviceItem("1", "手机听筒", AudioDeviceType.Earpiece),
        AudioDeviceItem("2", "扬声器", AudioDeviceType.Speakerphone, isConnected = true),
        AudioDeviceItem("3", "蓝牙耳机 AirPods Pro", AudioDeviceType.Bluetooth),
        AudioDeviceItem("4", "有线耳机", AudioDeviceType.WiredHeadset, isConnected = false)
    )
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            VoiceOrb(
                isActive = true,
                level = 0.6f,
                onClick = {}
            )
            CallStatusChip(
                status = VoiceCallStatus.Active,
                duration = "02:35"
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Text(
                    text = "麦克风电平",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                MicLevelIndicator(level = 0.7f)
            }
            Waveform(
                progress = 0.5f,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(32.dp),
                barCount = 40,
                isPlaying = true
            )
            Box {
                Surface(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(AmitiaTouchTarget.Minimum)
                        .clip(AmitiaPillShape)
                        .clickable { menuExpanded = true },
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Base),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.VolumeUp,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                        Text(
                            text = "扬声器",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Spacer(modifier = Modifier.weight(1f))
                        Icon(
                            imageVector = AmitiaIcons.ArrowDropDown,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
                AudioDevicePicker(
                    expanded = menuExpanded,
                    onDismiss = { menuExpanded = false },
                    devices = sampleDevices,
                    selectedDeviceId = "2",
                    onDeviceSelected = {}
                )
            }
            Spacer(modifier = Modifier.weight(1f))
            VoiceControlDock(
                isMuted = false,
                isSpeakerOn = true,
                isCaptionOn = true,
                onMuteToggle = {},
                onSpeakerToggle = {},
                onCaptionToggle = {},
                onMore = {},
                onEndCall = {}
            )
        }
    }
}

@Preview(name = "Voice - Dark", showBackground = true)
@Composable
private fun AmitiaVoiceDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            VoiceOrb(
                isActive = false,
                onClick = {}
            )
            CallStatusChip(
                status = VoiceCallStatus.Connecting
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Text(
                    text = "麦克风电平",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                MicLevelIndicator(level = 0.2f)
            }
            Waveform(
                progress = 0.3f,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(32.dp),
                barCount = 32,
                isPlaying = false
            )
            CallStatusChip(
                status = VoiceCallStatus.Muted
            )
            CallStatusChip(
                status = VoiceCallStatus.Failed
            )
            Spacer(modifier = Modifier.weight(1f))
            VoiceControlDock(
                isMuted = true,
                isSpeakerOn = false,
                isCaptionOn = false,
                onMuteToggle = {},
                onSpeakerToggle = {},
                onCaptionToggle = {},
                onMore = {},
                onEndCall = {}
            )
        }
    }
}
