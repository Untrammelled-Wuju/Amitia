package com.amitia.android.bootstrap

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.amitia.android.MainActivity
import com.amitia.android.onboarding.OnboardingActivity
import com.amitia.android.navigation.AmitiaRoutes
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaSpacing
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.delay

@AndroidEntryPoint
class BootstrapActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        val splashScreen = installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        val prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE)
        val onboardingCompleted = prefs.getBoolean(KEY_ONBOARDING_COMPLETED, false)
        val needsRecovery = prefs.getBoolean(KEY_NEEDS_RECOVERY, false)
        val needsMigration = prefs.getBoolean(KEY_NEEDS_MIGRATION, false)

        splashScreen.setKeepOnScreenCondition { false }

        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    BootstrapNavHost(
                        onboardingCompleted = onboardingCompleted,
                        needsRecovery = needsRecovery,
                        needsMigration = needsMigration,
                        onEnterOnboarding = { launchOnboarding() },
                        onEnterMain = { launchMain() }
                    )
                }
            }
        }
    }

    private fun launchOnboarding() {
        startActivity(Intent(this, OnboardingActivity::class.java))
        finish()
    }

    private fun launchMain() {
        startActivity(Intent(this, MainActivity::class.java))
        finish()
    }

    companion object {
        const val PREFS_NAME = "amitia_bootstrap"
        const val KEY_ONBOARDING_COMPLETED = "onboarding_completed"
        const val KEY_NEEDS_RECOVERY = "needs_recovery"
        const val KEY_NEEDS_MIGRATION = "needs_migration"

        fun markOnboardingCompleted(prefs: android.content.SharedPreferences) {
            prefs.edit().putBoolean(KEY_ONBOARDING_COMPLETED, true).apply()
        }
    }
}

@Composable
private fun BootstrapNavHost(
    onboardingCompleted: Boolean,
    needsRecovery: Boolean,
    needsMigration: Boolean,
    onEnterOnboarding: () -> Unit,
    onEnterMain: () -> Unit
) {
    val navController = rememberNavController()
    NavHost(
        navController = navController,
        startDestination = AmitiaRoutes.Bootstrap.SPLASH
    ) {
        composable(AmitiaRoutes.Bootstrap.SPLASH) {
            BootstrapSplashScreen()
            LaunchedEffect(Unit) {
                delay(SPLASH_DURATION_MS)
                navController.navigate(AmitiaRoutes.Bootstrap.STARTUP) {
                    popUpTo(AmitiaRoutes.Bootstrap.SPLASH) { inclusive = true }
                }
            }
        }

        composable(AmitiaRoutes.Bootstrap.STARTUP) {
            BootstrapStartupScreen()
            LaunchedEffect(Unit) {
                delay(STARTUP_CHECK_DELAY_MS)
                when {
                    needsMigration -> navController.navigate(AmitiaRoutes.Bootstrap.MIGRATION) {
                        launchSingleTop = true
                    }
                    needsRecovery -> navController.navigate(AmitiaRoutes.Bootstrap.RECOVERY) {
                        launchSingleTop = true
                    }
                    !onboardingCompleted -> onEnterOnboarding()
                    else -> onEnterMain()
                }
            }
        }

        composable(AmitiaRoutes.Bootstrap.RECOVERY) {
            BootstrapRecoveryScreen(
                onRecovered = onEnterMain,
                onAbort = { onEnterOnboarding() }
            )
        }

        composable(AmitiaRoutes.Bootstrap.MIGRATION) {
            BootstrapMigrationScreen(
                onCompleted = onEnterMain
            )
        }
    }
}

@Composable
private fun BootstrapSplashScreen() {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Lg)
        ) {
            Text(
                text = "Amitia",
                style = MaterialTheme.typography.displayLarge,
                color = MaterialTheme.colorScheme.primary
            )
            CircularProgressIndicator(
                color = MaterialTheme.colorScheme.primary,
                strokeWidth = 2.dp
            )
        }
    }
}

@Composable
private fun BootstrapStartupScreen() {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            CircularProgressIndicator(
                color = MaterialTheme.colorScheme.primary,
                strokeWidth = 2.dp
            )
            Text(
                text = "正在准备运行环境",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Composable
private fun BootstrapRecoveryScreen(
    onRecovered: () -> Unit,
    onAbort: () -> Unit
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
                text = "状态恢复",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "检测到上次会话异常退出，正在尝试恢复运行时状态与未完成的任务。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Button(onClick = onRecovered) {
                Text("继续进入主界面")
            }
            OutlinedButton(onClick = onAbort) {
                Text("重新引导")
            }
        }
    }
}

@Composable
private fun BootstrapMigrationScreen(
    onCompleted: () -> Unit
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
                text = "数据迁移",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "正在迁移本地数据结构到新版本，请勿关闭应用。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
            Button(onClick = onCompleted) {
                Text("完成迁移")
            }
        }
    }
}

private const val SPLASH_DURATION_MS = 800L
private const val STARTUP_CHECK_DELAY_MS = 400L
