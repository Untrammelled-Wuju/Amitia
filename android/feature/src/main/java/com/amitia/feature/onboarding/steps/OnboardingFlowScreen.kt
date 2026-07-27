package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
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

    Surface(
        modifier = Modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        OnboardingStepScaffold(
            currentStep = state.currentStep,
            transitioning = state.transitioning,
            onBack = if (state.currentStep.isEntry) null else viewModel::previous
        ) {
            Box(modifier = Modifier.fillMaxSize()) {
                when (state.currentStep) {
                    OnboardingFlowStep.Welcome -> WelcomeStepPage(
                        onStart = viewModel::next
                    )
                    OnboardingFlowStep.EnvCheck -> EnvCheckStepPage(
                        state = state,
                        onCheck = viewModel::checkEnvironment,
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.ModeSelection -> ModeSelectionStepPage(
                        selectedMode = state.mode,
                        onSelect = viewModel::selectMode,
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.RuntimeInstall -> RuntimeInstallStepPage(
                        state = state,
                        onStart = viewModel::startRuntimeInstall,
                        onPause = {},
                        onRetry = viewModel::startRuntimeInstall,
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.RemoteConfig -> RemoteConfigStepPage(
                        state = state,
                        onAddressChange = viewModel::updateRemoteAddress,
                        onPortChange = viewModel::updateRemotePort,
                        onTest = viewModel::testRemoteConnection,
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.AccountEntry -> AccountEntryStepPage(
                        onRegister = viewModel::next,
                        onLogin = { viewModel.goToStep(OnboardingFlowStep.Login) },
                        onUseLocal = viewModel::next
                    )
                    OnboardingFlowStep.Register -> RegisterStepPage(
                        state = state,
                        onFieldChange = viewModel::updateAccountField,
                        onSubmit = {
                            if (viewModel.submitRegister()) viewModel.next()
                        },
                        onBack = viewModel::previous
                    )
                    OnboardingFlowStep.Login -> LoginStepPage(
                        state = state,
                        onFieldChange = viewModel::updateAccountField,
                        onSubmit = viewModel::next,
                        onForgotPassword = {},
                        onBack = { viewModel.goToStep(OnboardingFlowStep.AccountEntry) },
                        remoteAddress = state.remoteAddress
                    )
                    OnboardingFlowStep.Permissions -> PermissionsStepPage(
                        state = state,
                        onToggle = viewModel::togglePermission,
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.ModelText -> TextModelStepPage(
                        state = state,
                        onFieldChange = { field, value -> viewModel.updateModelConfig("text", field, value) },
                        onTest = { viewModel.testModel("text") },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.ModelVision -> VisionModelStepPage(
                        state = state,
                        onFieldChange = { field, value -> viewModel.updateModelConfig("vision", field, value) },
                        onTest = { viewModel.testModel("vision") },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.ModelVoice -> VoiceModelStepPage(
                        state = state,
                        onTtsFieldChange = { field, value -> viewModel.updateModelConfig("tts", field, value) },
                        onSttFieldChange = { field, value -> viewModel.updateModelConfig("stt", field, value) },
                        onTtsTest = { viewModel.testModel("tts") },
                        onSttTest = { viewModel.testModel("stt") },
                        onVoiceSelect = viewModel::updateVoiceSelected,
                        onPreview = {},
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.ModelVector -> VectorModelStepPage(
                        state = state,
                        onFieldChange = { field, value -> viewModel.updateModelConfig("vector", field, value) },
                        onDimensionChange = viewModel::updateVectorDimension,
                        onTestModel = { viewModel.testModel("vector") },
                        onTestQdrant = viewModel::testVectorQdrant,
                        onTestVectorWrite = {},
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.CharacterAppearance -> CharacterAppearanceStepPage(
                        state = state,
                        onSelect = { viewModel.updateCharacter("appearance", it) },
                        onImportImage = {},
                        onUseSelected = viewModel::next
                    )
                    OnboardingFlowStep.CharacterName -> CharacterNameStepPage(
                        state = state,
                        onNameChange = { viewModel.updateCharacter("name", it) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.CharacterIdentity -> CharacterIdentityStepPage(
                        state = state,
                        onIdentityChange = { viewModel.updateCharacter("identity", it) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.CharacterPersonality -> CharacterPersonalityStepPage(
                        state = state,
                        onPersonalityChange = { viewModel.updateCharacter("personality", it) },
                        onCustomChange = { viewModel.updateCharacter("customPersonality", it) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.InitialMemory1 -> InitialMemory1StepPage(
                        state = state,
                        onNicknameChange = { viewModel.updateMemory("nickname", it) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.InitialMemory2 -> InitialMemory2StepPage(
                        state = state,
                        onRelationshipChange = { viewModel.updateMemory("relationship", it) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.InitialMemory3 -> InitialMemory3StepPage(
                        state = state,
                        onPreferencesChange = { viewModel.updateMemory("preferences", it) },
                        onBack = { viewModel.goToStep(OnboardingFlowStep.InitialMemory2) },
                        onNext = viewModel::next
                    )
                    OnboardingFlowStep.SetupSummary -> SetupSummaryStepPage(
                        state = state,
                        onContinue = viewModel::next
                    )
                    OnboardingFlowStep.CharacterComplete -> CharacterCompleteStepPage(
                        state = state,
                        onEditName = { viewModel.goToStep(OnboardingFlowStep.CharacterName) },
                        onEditIdentity = { viewModel.goToStep(OnboardingFlowStep.CharacterIdentity) },
                        onEditPersonality = { viewModel.goToStep(OnboardingFlowStep.CharacterPersonality) },
                        onContinue = viewModel::next
                    )
                    OnboardingFlowStep.EnterAmitia -> EnterAmitiaStepPage(
                        state = state,
                        onEnter = {
                            viewModel.playEnterAnimation {
                                onComplete()
                                onEnterMain()
                            }
                        }
                    )
                    OnboardingFlowStep.DataImport -> DataImportStepPage(
                        onSelectBackup = {},
                        onSelectChatHistory = {},
                        onSkip = onComplete
                    )
                }
            }
        }
    }
}
