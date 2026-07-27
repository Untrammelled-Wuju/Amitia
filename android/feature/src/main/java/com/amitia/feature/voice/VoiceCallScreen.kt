package com.amitia.feature.voice

import androidx.compose.foundation.background
importpackage com.amitia.feature.voice

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
import androidx.compose.material3.Materialpackage com.amitia.feature.voice

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
importpackage com.amitia.feature.voice

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
importpackage com.amitia.feature.voice

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
import com.amitia.core.designpackage com.amitia.feature.voice

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
import com.amitia.corepackage com.amitia.feature.voice

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
import com.amitia.core.designsystem.component.VoiceCallStatuspackage com.amitia.feature.voice

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
import com.amitia.core.designsystem.component.Vpackage com.amitia.feature.voice

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
import com.amitia.core.designpackage com.amitia.feature.voice

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
    val state by viewModel.callState.collectAsStateWithpackage com.amitia.feature.voice

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
        onOpenCaptions = onOpenpackage com.amitia.feature.voice

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
        onMuteToggle =package com.amitia.feature.voice

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
        onCaptionTogglepackage com.amitia.feature.voice

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
        onSwitchText = viewModelpackage com.amitia.feature.voice

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
        onShowDevicepackage com.amitia.feature.voice

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
    onSwitchTextpackage com.amitia.feature.voice

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
    onSelectDevice: (com.amitia.core.designsystem.component.AudioDeviceItempackage com.amitia.feature.voice

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
    onDismissDevicePicker:package com.amitia.feature.voice

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
            verticalArrangement = Arrpackage com.amitia.feature.voice

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
            VoiceCallTopBar(state = state, onOpenAudioDevicepackage com.amitia.feature.voice

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
                    horizontalAlignmentpackage com.amitia.feature.voice

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
                    verticalArrangement