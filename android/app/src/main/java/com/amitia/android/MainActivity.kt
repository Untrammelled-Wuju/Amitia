package com.amitia.android

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import androidx.navigation.compose.rememberNavController
import com.amitia.android.navigation.AmitiaAdaptiveNavigationContainer
import com.amitia.android.navigation.AmitiaMainNavHost
import com.amitia.android.navigation.NavEvent
import com.amitia.android.navigation.NavEventBus
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.platform.audio.AudioPlayerImpl
import com.amitia.platform.bridge.ActivityResultBridge
import com.amitia.platform.notification.NotificationManagerImpl
import com.amitia.platform.notification.UnreadRecovery
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import javax.inject.Inject

@AndroidEntryPoint
class MainActivity : ComponentActivity() {

    @Inject
    lateinit var activityResultBridge: ActivityResultBridge

    @Inject
    lateinit var navEventBus: NavEventBus

    @Inject
    lateinit var audioPlayerImpl: AudioPlayerImpl

    @Inject
    lateinit var unreadRecovery: UnreadRecovery

    private val mainScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        activityResultBridge.attachActivity(this)
        enableEdgeToEdge()
        handleIntent(intent, isFirstLaunch = true)
        unreadRecovery.recoverOnStartup()
        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    val navController = rememberNavController()
                    AmitiaAdaptiveNavigationContainer(navController = navController) { padding ->
                        AmitiaMainNavHost(
                            navController = navController,
                            navEventBus = navEventBus,
                            modifier = Modifier
                                .fillMaxSize()
                                .padding(padding)
                        )
                    }
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleIntent(intent, isFirstLaunch = false)
    }

    override fun onDestroy() {
        super.onDestroy()
        mainScope.cancel()
        runCatching { audioPlayerImpl.release() }
        runCatching { activityResultBridge.detachActivity(this) }
    }

    private fun handleIntent(intent: Intent?, isFirstLaunch: Boolean) {
        if (intent == null) return
        val action = intent.action ?: return
        when (action) {
            NotificationManagerImpl.ACTION_OPEN_CONVERSATION -> {
                val characterId = intent.getStringExtra(NotificationManagerImpl.EXTRA_CHARACTER_ID)
                    ?: return
                val conversationId = intent.getStringExtra(NotificationManagerImpl.EXTRA_CONVERSATION_ID)
                val messageId = intent.getStringExtra(NotificationManagerImpl.EXTRA_MESSAGE_ID)
                mainScope.launch {
                    navEventBus.emit(
                        NavEvent.OpenChat(
                            characterId = characterId,
                            conversationId = conversationId,
                            messageId = messageId
                        )
                    )
                }
            }
            com.amitia.platform.notification.NotificationIntentBuilder.ACTION_CLEAR_NOTIFICATIONS -> {
                mainScope.launch { navEventBus.emit(NavEvent.ClearNotifications) }
            }
            Intent.ACTION_MAIN -> {
                if (isFirstLaunch) {
                    mainScope.launch { navEventBus.emit(NavEvent.OpenHome) }
                }
            }
            else -> Unit
        }
    }
}
