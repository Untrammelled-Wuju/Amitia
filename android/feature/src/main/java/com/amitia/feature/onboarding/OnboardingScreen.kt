package com.amitia.feature.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.component.AmitiaErrorBanner

@Composable
fun OnboardingScreen(
    onComplete: () -> Unit,
    viewModel: OnboardingViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val step by viewModel.currentStep.collectAsStateWithLifecycle()
    val total = 9
    val progress = (step.index() + 1).toFloat() / total.toFloat()

    LaunchedEffect(state.error, state.runtimeError) {
        if (state.error != null || state.runtimeError != null) {
            kotlinx.coroutines.delay(4000)
            viewModel.consumeError()
        }
    }

    Scaffold(
        modifier = Modifier.fillMaxSize(),
        containerColor = MaterialTheme.colorScheme.background
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .imePadding()
                .navigationBarsPadding()
        ) {
            OnboardingProgressBar(progress = progress, step = step)
            Box(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .verticalScroll(rememberScrollState())
                    .padding(20.dp)
            ) {
                when (step) {
                    OnboardingStep.Welcome -> WelcomeStep()
                    OnboardingStep.ModeSelection -> ModeSelectionStep(
                        state = state,
                        onSelectMode = viewModel::selectMode
                    )
                    OnboardingStep.RuntimeInstall -> RuntimeInstallStep(
                        state = state,
                        onStart = viewModel::startRuntimeInstall,
                        onRetry = viewModel::startRuntimeInstall
                    )
                    OnboardingStep.RemoteConfig -> RemoteConfigStep(
                        state = state,
                        onConfigure = viewModel::configureRemote
                    )
                    OnboardingStep.EnvCheck -> EnvCheckStep(
                        state = state,
                        onCheck = viewModel::checkEnv
                    )
                    OnboardingStep.AuthInit -> AuthInitStep(
                        state = state,
                        onInit = viewModel::authInit
                    )
                    OnboardingStep.ModelConfig -> ModelConfigStep(
                        state = state,
                        onConfigure = viewModel::configureModel
                    )
                    OnboardingStep.CharacterSetup -> CharacterSetupStep(
                        state = state,
                        onSetup = viewModel::setupCharacter
                    )
                    OnboardingStep.InitialMemory -> InitialMemoryStep(
                        state = state,
                        onCreate = viewModel::createInitialMemory
                    )
                    OnboardingStep.Complete -> CompleteStep(onFinish = { viewModel.complete(onComplete) })
                }
            }
            if (state.error != null) {
                AmitiaErrorBanner(message = state.error.orEmpty())
            }
            OnboardingNavButtons(
                step = step,
                hasError = state.runtimeError != null,
                onNext = viewModel::next,
                onPrevious = viewModel::previous,
                onSkip = viewModel::skipCurrent
            )
        }
    }
}

@Composable
private fun OnboardingProgressBar(progress: Float, step: OnboardingStep) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(20.dp)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text = "Amitia 初始化",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onBackground,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = "${step.index() + 1} / 9",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        Spacer(modifier = Modifier.height(8.dp))
        LinearProgressIndicator(
            progress = { progress },
            modifier = Modifier
                .fillMaxWidth()
                .height(4.dp),
            color = MaterialTheme.colorScheme.primary,
            trackColor = MaterialTheme.colorScheme.surfaceVariant
        )
    }
}

@Composable
private fun OnboardingNavButtons(
    step: OnboardingStep,
    hasError: Boolean,
    onNext: () -> Unit,
    onPrevious: () -> Unit,
    onSkip: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(20.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        if (step != OnboardingStep.Welcome && step != OnboardingStep.Complete) {
            OutlinedButton(
                onClick = onPrevious,
                modifier = Modifier.weight(1f)
            ) {
                Text(text = "上一步")
            }
            Button(
                onClick = onNext,
                modifier = Modifier.weight(1f),
                enabled = !hasError || step == OnboardingStep.RuntimeInstall
            ) {
                Text(text = if (hasError && step == OnboardingStep.RuntimeInstall) "重试" else "下一步")
            }
            TextButton(onClick = onSkip) {
                Text(text = "跳过")
            }
        } else if (step == OnboardingStep.Welcome) {
            Spacer(modifier = Modifier.weight(1f))
            Button(onClick = onNext) { Text(text = "开始") }
        }
    }
}
