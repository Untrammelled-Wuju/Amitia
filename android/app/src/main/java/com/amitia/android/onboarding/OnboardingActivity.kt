package com.amitia.android.onboarding

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.amitia.android.MainActivity
import com.amitia.android.bootstrap.BootstrapActivity
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.feature.onboarding.steps.OnboardingFlowScreen
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class OnboardingActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            AmitiaTheme(darkTheme = isSystemInDarkTheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    OnboardingFlowScreen(
                        onComplete = { completeOnboarding() },
                        onEnterMain = { completeOnboarding() }
                    )
                }
            }
        }
    }

    private fun completeOnboarding() {
        val prefs = getSharedPreferences(BootstrapActivity.PREFS_NAME, MODE_PRIVATE)
        BootstrapActivity.markOnboardingCompleted(prefs)
        startActivity(Intent(this, MainActivity::class.java))
        finish()
    }
}
