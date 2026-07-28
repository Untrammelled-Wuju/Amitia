package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.delay

@Composable
fun OnboardingFlowScreen(
    onComplete: () -> Unit,
    onEnterMain: () -> Unit,
    viewModel: OnboardingFlowViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(state.error) {
        if (state.error != null) {
            delay(4000)
            viewModel.consumeError()
        }
    }

    OnboardingScaffold(
        currentStep = state.currentStep,
        transitioning = state.transitioning,
        preparingEntry = state.preparingEntry,
        onBack = if (state.currentStep.isEntry) null else viewModel::previous
    ) {
        Box(modifier = Modifier.fillMaxSize()) {
            when (state.currentStep) {
                OnboardingFlowStep.Welcome -> WelcomeStepPage(
                    onStart = viewModel::next
                )
                OnboardingFlowStep.ModeSelection -> ModeSelectionStepPage(
                    selectedMode = state.mode,
                    onSelect = viewModel::selectMode,
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.Environment -> EnvironmentStepPage(
                    state = state,
                    onCheck = viewModel::checkEnvironment,
                    onInstall = viewModel::startRuntimeInstall,
                    onAddressChange = viewModel::updateRemoteAddress,
                    onPortChange = viewModel::updateRemotePort,
                    onTestConnection = viewModel::testRemoteConnection,
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.Account -> AccountStepPage(
                    state = state,
                    onFieldChange = viewModel::updateAccountField,
                    onToggleLogin = viewModel::toggleAccountLogin,
                    onSubmit = viewModel::submitAccount,
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.ModelConfig -> ModelConfigStepPage(
                    state = state,
                    onFieldChange = { type, field, value -> viewModel.updateModelConfig(type, field, value) },
                    onTest = { viewModel.testModel(it) },
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.CharacterSetup -> CharacterSetupStepPage(
                    state = state,
                    onFieldChange = viewModel::updateCharacter,
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.UserInfo -> UserInfoStepPage(
                    state = state,
                    onFieldChange = viewModel::updateMemory,
                    onNext = viewModel::next,
                    onBack = viewModel::previous
                )
                OnboardingFlowStep.Complete -> CompleteStepPage(
                    onEnter = {
                        viewModel.playEnterAnimation {
                            onComplete()
                            onEnterMain()
                        }
                    }
                )
            }
        }
    }
}
