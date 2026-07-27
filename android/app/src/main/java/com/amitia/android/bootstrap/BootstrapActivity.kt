package com.amitia.android.bootstrap

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import com.amitia.android.MainActivity
import com.amitia.android.navigation.AmitiaBootstrapNavHost
import com.amitia.android.onboarding.OnboardingActivity
import com.amitia.core.designsystem.AmitiaTheme
import dagger.hilt.android.AndroidEntryPoint

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
            AmitiaTheme(darkTheme = isSystemInDarkTheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    AmitiaBootstrapNavHost(
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
