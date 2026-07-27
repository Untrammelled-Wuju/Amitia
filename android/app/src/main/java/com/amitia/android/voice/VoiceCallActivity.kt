package com.amitia.android.voice

import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CallEnd
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.MicOff
import androidx.compose.material.icons.filled.VolumeUp
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.amitia.android.navigation.AmitiaRoutes
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaSpacing
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class VoiceCallActivity : ComponentActivity() {

    private val audioManager by lazy { getSystemService(AUDIO_SERVICE) as AudioManager }
    private var audioFocusRequest: AudioFocusRequest? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        requestAudioFocus()
        hideSystemBars()
        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    VoiceCallNavHost(onEndCall = { endCall() })
                }
            }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        abandonAudioFocus()
    }

    private fun endCall() {
        abandonAudioFocus()
        finish()
    }

    private fun requestAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT)
                .setAudioAttributes(
                    AudioAttributes.Builder()
                        .setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION)
                        .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                        .build()
                )
                .build()
            audioFocusRequest = request
            audioManager.requestAudioFocus(request)
        } else {
            @Suppress("DEPRECATION")
            audioManager.requestAudioFocus(
                null,
                AudioManager.STREAM_VOICE_CALL,
                AudioManager.AUDIOFOCUS_GAIN_TRANSIENT
            )
        }
    }

    private fun abandonAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
        }
        audioFocusRequest = null
    }

    private fun hideSystemBars() {
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowInsetsControllerCompat(window, window.decorView).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }
}

@Composable
private fun VoiceCallNavHost(onEndCall: () -> Unit) {
    val navController = rememberNavController()
    NavHost(
        navController = navController,
        startDestination = AmitiaRoutes.VoiceCall.CALL
    ) {
        composable(AmitiaRoutes.VoiceCall.CALL) {
            VoiceCallScreen(
                onOpenCaptions = { navController.navigate(AmitiaRoutes.VoiceCall.CAPTIONS) },
                onOpenAudioDevice = { navController.navigate(AmitiaRoutes.VoiceCall.AUDIO_DEVICE) },
                onEndCall = onEndCall
            )
        }

        composable(AmitiaRoutes.VoiceCall.INCOMING) {
            VoiceIncomingScreen(
                onAccept = {
                    navController.navigate(AmitiaRoutes.VoiceCall.CALL) {
                        popUpTo(AmitiaRoutes.VoiceCall.INCOMING) { inclusive = true }
                    }
                },
                onReject = onEndCall
            )
        }

        composable(AmitiaRoutes.VoiceCall.CAPTIONS) {
            VoiceDetailScreen(
                title = "实时字幕",
                description = "通话语音正在实时转写为文字",
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.VoiceCall.AUDIO_DEVICE) {
            VoiceDetailScreen(
                title = "音频设备",
                description = "选择扬声器、耳机或蓝牙设备",
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.VoiceCall.CALL_HISTORY) {
            VoiceDetailScreen(
                title = "通话记录",
                description = "查看历史语音通话记录",
                onBack = { navController.popBackStack() }
            )
        }

        composable(
            route = AmitiaRoutes.VoiceCall.CALL_DETAIL,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_CALL_ID) { type = NavType.StringType })
        ) { entry ->
            val callId = entry.arguments?.getString(AmitiaRoutes.KEY_CALL_ID).orEmpty()
            VoiceDetailScreen(
                title = "通话详情",
                description = "通话 $callId 的详细信息与录音",
                onBack = { navController.popBackStack() }
            )
        }
    }
}

@Composable
private fun VoiceCallScreen(
    onOpenCaptions: () -> Unit,
    onOpenAudioDevice: () -> Unit,
    onEndCall: () -> Unit
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxl)
        ) {
            Text(
                text = "Amitia",
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = "通话中",
                style = MaterialTheme.typography.displaySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                VoiceCallIconButton(icon = Icons.Filled.Mic, label = "麦克风", onClick = {})
                VoiceCallIconButton(icon = Icons.Filled.VolumeUp, label = "设备", onClick = onOpenAudioDevice)
                VoiceCallIconButton(icon = Icons.Filled.CallEnd, label = "挂断", onClick = onEndCall)
            }
            OutlinedButton(onClick = onOpenCaptions) {
                Text("查看字幕")
            }
        }
    }
}

@Composable
private fun VoiceCallIconButton(
    icon: ImageVector,
    label: String,
    onClick: () -> Unit
) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        IconButton(onClick = onClick) {
            Icon(
                imageVector = icon,
                contentDescription = label,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(AmitiaSpacing.Sm)
            )
        }
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun VoiceIncomingScreen(
    onAccept: () -> Unit,
    onReject: () -> Unit
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxl)
        ) {
            Text(
                text = "来电",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "Amitia 正在呼叫你",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            androidx.compose.foundation.layout.Row(
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxl)
            ) {
                Button(onClick = onAccept) { Text("接听") }
                OutlinedButton(onClick = onReject) { Text("拒接") }
            }
        }
    }
}

@Composable
private fun VoiceDetailScreen(
    title: String,
    description: String,
    onBack: () -> Unit
) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            Text(
                text = title,
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = description,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            OutlinedButton(onClick = onBack) { Text("返回") }
        }
    }
}
