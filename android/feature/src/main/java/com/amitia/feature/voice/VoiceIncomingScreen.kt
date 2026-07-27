package com.amitia.feature.voice

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun VoiceIncomingScreen(
    onAccept: () -> Unit,
    onReject: () -> Unit,
    onTextReply: () -> Unit = {}
) {
    VoiceIncomingContent(
        characterName = "艾米",
        characterIdentity = "温柔知性助手",
        reason = "想和你聊聊今天的心情，现在方便吗？",
        onAccept = onAccept,
        onReject = onReject,
        onTextReply = onTextReply
    )
}

@Composable
fun VoiceIncomingContent(
    characterName: String,
    characterIdentity: String,
    reason: String,
    onAccept: () -> Unit,
    onReject: () -> Unit,
    onTextReply: () -> Unit
) {
    val infiniteTransition = rememberInfiniteTransition(label = "incomingPulse")
    val pulseScale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = 1.15f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseScale"
    )

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .statusBarsPadding()
            .systemBarsPadding()
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween
        ) {
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Text(
                    text = "语音来电",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
                Box(contentAlignment = Alignment.Center) {
                    Box(
                        modifier = Modifier
                            .size(140.dp)
                            .scale(pulseScale)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.12f))
                    )
                    Box(
                        modifier = Modifier
                            .size(96.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Person,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(AmitiaIconSize.Huge)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Lg))
                Text(
                    text = characterName,
                    style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onBackground
                )
                Text(
                    text = characterIdentity,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(MaterialTheme.shapes.medium)
                    .background(MaterialTheme.colorScheme.surface)
                    .padding(AmitiaSpacing.Base),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.ChatBubble,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                    Text(
                        text = "来电原因",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = reason,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    textAlign = TextAlign.Start
                )
            }
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                ) {
                    Box(
                        modifier = Modifier
                            .size(AmitiaTouchTarget.Minimum)
                            .weight(1f)
                            .height(AmitiaTouchTarget.Minimum),
                        contentAlignment = Alignment.Center
                    ) {
                        RejectCallButton(onClick = onReject)
                    }
                    Box(
                        modifier = Modifier
                            .size(AmitiaTouchTarget.Minimum)
                            .weight(1f)
                            .height(AmitiaTouchTarget.Minimum),
                        contentAlignment = Alignment.Center
                    ) {
                        AcceptCallButton(onClick = onAccept)
                    }
                }
                TertiaryButton(
                    text = "转文字回复",
                    onClick = onTextReply,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Chat
                )
            }
        }
    }
}

@Composable
private fun AcceptCallButton(onClick: () -> Unit) {
    val interactionSource = androidx.compose.runtime.remember { androidx.compose.foundation.interaction.MutableInteractionSource() }
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        Box(
            modifier = Modifier
                .size(64.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.primary)
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    role = androidx.compose.ui.semantics.Role.Button,
                    onClick = onClick
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.Phone,
                contentDescription = "接听",
                tint = MaterialTheme.colorScheme.onPrimary,
                modifier = Modifier.size(AmitiaIconSize.Large)
            )
        }
        Text(
            text = "接听",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.primary
        )
    }
}

@Composable
private fun RejectCallButton(onClick: () -> Unit) {
    val interactionSource = androidx.compose.runtime.remember { androidx.compose.foundation.interaction.MutableInteractionSource() }
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        Box(
            modifier = Modifier
                .size(64.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.error)
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    role = androidx.compose.ui.semantics.Role.Button,
                    onClick = onClick
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = AmitiaIcons.PhoneOff,
                contentDescription = "拒绝",
                tint = MaterialTheme.colorScheme.onError,
                modifier = Modifier.size(AmitiaIconSize.Large)
            )
        }
        Text(
            text = "拒绝",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.error
        )
    }
}

@Preview(name = "Voice Incoming - Light", showBackground = true)
@Composable
private fun VoiceIncomingLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceIncomingContent(
            characterName = "艾米",
            characterIdentity = "温柔知性助手",
            reason = "想和你聊聊今天的心情，现在方便吗？",
            onAccept = {},
            onReject = {},
            onTextReply = {}
        )
    }
}

@Preview(name = "Voice Incoming - Dark", showBackground = true)
@Composable
private fun VoiceIncomingDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceIncomingContent(
            characterName = "艾米",
            characterIdentity = "温柔知性助手",
            reason = "想和你聊聊今天的心情，现在方便吗？",
            onAccept = {},
            onReject = {},
            onTextReply = {}
        )
    }
}
