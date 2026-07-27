package com.amitia.feature.voice

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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AudioDevicePicker
import com.amitia.core.designsystem.component.CallStatusChip
import com.amitia.core.designsystem.component.MicLevelIndicator
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.VoiceCallStatus
import com.amitia.core.designsystem.component.VoiceControlDock
import com.amitia.core.designsystem.component.VoiceOrb
import com.amitia.core.designsystem.component.Waveform

@Composable
fun VoiceCallScreen(
    onOpenCaptions: () -> Unit,
    onOpenAudioDevice: () -> Unit,
    onEndCall: () -> Unit,
    viewModel: VoiceViewModel = hiltViewModel()
) {
    val state by viewModel.callState.collectAsStateWithLifecycle()
    VoiceCallContent(
        state = state,
        onOpenCaptions = onOpenCaptions,
        onOpenAudioDevice = onOpenAudioDevice,
        onMuteToggle = viewModel::toggleMute,
        onSpeakerToggle = viewModel::toggleSpeaker,
        onCaptionToggle = viewModel::toggleCaption,
        onEndCall = onEndCall,
        onRetry = viewModel::retryConnection,
        onSwitchText = viewModel::switchToTextChat,
        onShowDevicePicker = { viewModel.showDevicePicker(true) },
        onSelectDevice = { viewModel.selectDevice(it); viewModel.showDevicePicker(false) },
        onDismissDevicePicker = { viewModel.showDevicePicker(false) }
    )
}

@Composable
fun VoiceCallContent(
    state: VoiceCallUiState,
    onOpenCaptions: () -> Unit,
    onOpenAudioDevice: () -> Unit,
    onMuteToggle: () -> Unit,
    onSpeakerToggle: () -> Unit,
    onCaptionToggle: () -> Unit,
    onEndCall: () -> Unit,
    onRetry: () -> Unit,
    onSwitchText: () -> Unit,
    onShowDevicePicker: () -> Unit,
    onSelectDevice: (com.amitia.core.designsystem.component.AudioDeviceItem) -> Unit,
    onDismissDevicePicker: () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .statusBarsPadding()
            .systemBarsPadding()
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.SpaceBetween
        ) {
            VoiceCallTopBar(state = state, onOpenAudioDevice = onOpenAudioDevice)
            VoiceCallCenter(state = state)
            if (state.connectionFailed || state.status == VoiceCallStatus.Failed) {
                VoiceCallFailedActions(onRetry = onRetry, onSwitchText = onSwitchText)
            } else {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
                ) {
                    VoiceCallBottomControls(
                        state = state,
                        onOpenCaptions = onOpenCaptions,
                        onShowDevicePicker = onShowDevicePicker
                    )
                    VoiceControlDock(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = AmitiaSpacing.Base)
                            .padding(bottom = AmitiaSpacing.Lg),
                        isMuted = state.isMuted,
                        isSpeakerOn = state.isSpeakerOn,
                        isCaptionOn = state.isCaptionOn,
                        onMuteToggle = onMuteToggle,
                        onSpeakerToggle = onSpeakerToggle,
                        onCaptionToggle = onCaptionToggle,
                        onMore = onOpenAudioDevice,
                        onEndCall = onEndCall
                    )
                }
            }
        }
        AudioDevicePicker(
            expanded = state.showDevicePicker,
            onDismiss = onDismissDevicePicker,
            devices = state.audioDevices,
            selectedDeviceId = state.selectedDeviceId,
            onDeviceSelected = onSelectDevice
        )
    }
}

@Composable
private fun VoiceCallTopBar(state: VoiceCallUiState, onOpenAudioDevice: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(top = AmitiaSpacing.Lg),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        Text(
            text = state.characterName,
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground
        )
        Text(
            text = state.characterIdentity,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        CallStatusChip(
            status = state.status,
            duration = if (state.status == VoiceCallStatus.Active) state.durationText else null
        )
        if (state.secondaryStatus.text.isNotEmpty()) {
            Text(
                text = state.secondaryStatus.text,
                style = MaterialTheme.typography.labelLarge,
                color = MaterialTheme.colorScheme.primary
            )
        }
        if (state.signalQuality != VoiceSignalQuality.Good) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                androidx.compose.material3.Icon(
                    imageVector = AmitiaIcons.WarningAmber,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.tertiary,
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
                Text(
                    text = state.signalQuality.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.tertiary
                )
            }
        }
    }
}

@Composable
private fun VoiceCallCenter(state: VoiceCallUiState) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xl)
    ) {
        VoiceOrb(
            isActive = state.status == VoiceCallStatus.Active && !state.isMuted,
            level = state.micLevel,
            onClick = null
        )
        Waveform(
            progress = state.waveformProgress,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Xxl)
                .height(40.dp),
            barCount = 48,
            isPlaying = state.isPlaying
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
            MicLevelIndicator(level = state.micLevel)
        }
    }
}

@Composable
private fun VoiceCallBottomControls(
    state: VoiceCallUiState,
    onOpenCaptions: () -> Unit,
    onShowDevicePicker: () -> Unit
) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically
    ) {
        SecondaryButton(
            text = "字幕",
            onClick = onOpenCaptions,
            leadingIcon = AmitiaIcons.ChatBubble
        )
        SecondaryButton(
            text = "音频设备",
            onClick = onShowDevicePicker,
            leadingIcon = AmitiaIcons.VolumeUp
        )
    }
}

@Composable
private fun VoiceCallFailedActions(onRetry: () -> Unit, onSwitchText: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Xxl)
            .padding(bottom = AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = "连接失败，请检查网络后重试",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.error,
            textAlign = TextAlign.Center
        )
        PrimaryButton(
            text = "重试",
            onClick = onRetry,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Refresh
        )
        SecondaryButton(
            text = "切换到文字对话",
            onClick = onSwitchText,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = AmitiaIcons.Chat
        )
    }
}

@Preview(name = "Voice Call - Light", showBackground = true)
@Composable
private fun VoiceCallScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceCallContent(
            state = VoiceCallUiState(
                status = VoiceCallStatus.Active,
                durationSeconds = 155,
                isSpeakerOn = true,
                micLevel = 0.6f,
                waveformProgress = 0.5f,
                isPlaying = true,
                secondaryStatus = VoiceSecondaryStatus.Speaking
            ),
            onOpenCaptions = {},
            onOpenAudioDevice = {},
            onMuteToggle = {},
            onSpeakerToggle = {},
            onCaptionToggle = {},
            onEndCall = {},
            onRetry = {},
            onSwitchText = {},
            onShowDevicePicker = {},
            onSelectDevice = {},
            onDismissDevicePicker = {}
        )
    }
}

@Preview(name = "Voice Call - Dark", showBackground = true)
@Composable
private fun VoiceCallScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceCallContent(
            state = VoiceCallUiState(
                status = VoiceCallStatus.Failed,
                connectionFailed = true,
                isMuted = true,
                micLevel = 0.2f,
                waveformProgress = 0.3f,
                isPlaying = false
            ),
            onOpenCaptions = {},
            onOpenAudioDevice = {},
            onMuteToggle = {},
            onSpeakerToggle = {},
            onCaptionToggle = {},
            onEndCall = {},
            onRetry = {},
            onSwitchText = {},
            onShowDevicePicker = {},
            onSelectDevice = {},
            onDismissDevicePicker = {}
        )
    }
}
