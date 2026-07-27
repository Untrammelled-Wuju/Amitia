package com.amitia.android.navigation

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavType
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.amitia.feature.voice.VoiceCallDetailScreen
import com.amitia.feature.voice.VoiceCallScreen
import com.amitia.feature.voice.VoiceCaptionScreen
import com.amitia.feature.voice.VoiceHistoryScreen
import com.amitia.feature.voice.VoiceIncomingScreen
import com.amitia.feature.voice.VoiceSettingsScreen

@Composable
fun AmitiaVoiceNavHost(
    onEndCall: () -> Unit,
    modifier: Modifier = Modifier,
    navController: NavHostController = rememberNavController()
) {
    NavHost(
        navController = navController,
        startDestination = AmitiaRoutes.VoiceCall.CALL,
        modifier = modifier
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
                onAccept = { navController.navigate(AmitiaRoutes.VoiceCall.CALL) },
                onReject = onEndCall,
                onTextReply = onEndCall
            )
        }

        composable(AmitiaRoutes.VoiceCall.CAPTIONS) {
            VoiceCaptionScreen(onBack = { navController.popBackStack() })
        }

        composable(AmitiaRoutes.VoiceCall.AUDIO_DEVICE) {
            VoiceSettingsScreen(onBack = { navController.popBackStack() })
        }

        composable(AmitiaRoutes.VoiceCall.CALL_HISTORY) {
            VoiceHistoryScreen(
                onBack = { navController.popBackStack() },
                onOpenDetail = { callId -> navController.navigate(AmitiaRoutes.VoiceCall.callDetail(callId)) }
            )
        }

        composable(
            route = AmitiaRoutes.VoiceCall.CALL_DETAIL,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_CALL_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val callId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CALL_ID).orEmpty()
            VoiceCallDetailScreen(
                callId = callId,
                onBack = { navController.popBackStack() }
            )
        }
    }
}
