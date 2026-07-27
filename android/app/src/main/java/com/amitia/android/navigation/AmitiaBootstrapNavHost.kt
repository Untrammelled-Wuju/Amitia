package com.amitia.android.navigation

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.feature.bootstrap.BootstrapSplashContent
import com.amitia.feature.bootstrap.BootstrapViewModel
import com.amitia.feature.bootstrap.MigrationScreen
import com.amitia.feature.bootstrap.RecoveryScreen
import com.amitia.feature.bootstrap.StartupPreparationScreen
import kotlinx.coroutines.delay

@Composable
fun AmitiaBootstrapNavHost(
    onboardingCompleted: Boolean,
    needsRecovery: Boolean,
    needsMigration: Boolean,
    onEnterOnboarding: () -> Unit,
    onEnterMain: () -> Unit,
    navController: NavHostController = rememberNavController(),
    viewModel: BootstrapViewModel = hiltViewModel()
) {
    val startupState by viewModel.startupState.collectAsStateWithLifecycle()
    val recoveryState by viewModel.recoveryState.collectAsStateWithLifecycle()
    val migrationState by viewModel.migrationState.collectAsStateWithLifecycle()

    NavHost(
        navController = navController,
        startDestination = AmitiaRoutes.Bootstrap.SPLASH
    ) {
        composable(AmitiaRoutes.Bootstrap.SPLASH) {
            Surface(
                modifier = Modifier.fillMaxSize(),
                color = MaterialTheme.colorScheme.background
            ) {
                BootstrapSplashContent()
            }
            LaunchedEffect(Unit) {
                delay(SPLASH_DURATION_MS)
                navController.navigate(AmitiaRoutes.Bootstrap.STARTUP) {
                    popUpTo(AmitiaRoutes.Bootstrap.SPLASH) { inclusive = true }
                }
            }
        }

        composable(AmitiaRoutes.Bootstrap.STARTUP) {
            LaunchedEffect(Unit) {
                viewModel.startStartup()
                delay(STARTUP_CHECK_DELAY_MS)
                when {
                    needsMigration -> navController.navigate(AmitiaRoutes.Bootstrap.MIGRATION) {
                        launchSingleTop = true
                    }
                    needsRecovery -> {
                        viewModel.loadRecoveryInfo("异常退出", "")
                        navController.navigate(AmitiaRoutes.Bootstrap.RECOVERY) {
                            launchSingleTop = true
                        }
                    }
                    !onboardingCompleted -> onEnterOnboarding()
                    else -> onEnterMain()
                }
            }
            StartupPreparationScreen(
                state = startupState,
                onRetry = viewModel::retryStartup,
                onSwitchRemote = onEnterOnboarding,
                onDiagnostics = onEnterMain,
                onExit = onEnterOnboarding,
                onToggleDetail = viewModel::toggleDetail
            )
        }

        composable(AmitiaRoutes.Bootstrap.RECOVERY) {
            RecoveryScreen(
                state = recoveryState,
                onSafeBoot = { viewModel.safeBoot(onEnterMain) },
                onNormalBoot = { viewModel.normalBoot(onEnterMain) },
                onViewLogs = {},
                onRestoreBackup = { viewModel.restoreBackup(onEnterMain) }
            )
        }

        composable(AmitiaRoutes.Bootstrap.MIGRATION) {
            LaunchedEffect(Unit) {
                viewModel.loadMigrationInfo("v1.0", "v2.0")
                viewModel.runMigration { onEnterMain() }
            }
            MigrationScreen(
                state = migrationState,
                onRollback = viewModel::rollbackMigration,
                onRetry = { viewModel.runMigration { onEnterMain() } },
                onCompleted = onEnterMain
            )
        }
    }
}

private const val SPLASH_DURATION_MS = 800L
private const val STARTUP_CHECK_DELAY_MS = 400L
