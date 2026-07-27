package com.amitia.feature.bootstrap

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.UiError
import com.amitia.core.designsystem.UiWarning
import com.amitia.core.designsystem.ErrorType
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

enum class StartupPhase {
    PreparingEnvironment,
    StartingServices,
    ConnectingServices,
    RestoringSession
}

enum class StartupIssue {
    None,
    RuntimeSlow,
    ServiceFailed,
    MigratingData,
    StorageLow,
    RemoteUnreachable
}

data class StartupProgressItem(
    val phase: StartupPhase,
    val label: String,
    val done: Boolean
)

data class StartupState(
    val currentPhase: StartupPhase = StartupPhase.PreparingEnvironment,
    val progress: Float = 0f,
    val items: List<StartupProgressItem> = emptyList(),
    val issue: StartupIssue = StartupIssue.None,
    val detailExpanded: Boolean = false,
    val failed: Boolean = false
)

data class RecoveryState(
    val crashReason: String = "",
    val crashTime: String = "",
    val safeModeAvailable: Boolean = true,
    val backupAvailable: Boolean = true,
    val restoring: Boolean = false,
    val restored: Boolean = false,
    val error: String? = null
)

data class MigrationStep(
    val name: String,
    val done: Boolean,
    val inProgress: Boolean = false,
    val failed: Boolean = false
)

data class MigrationState(
    val fromVersion: String = "",
    val toVersion: String = "",
    val progress: Float = 0f,
    val steps: List<MigrationStep> = emptyList(),
    val completed: Boolean = false,
    val failed: Boolean = false,
    val rollbackAvailable: Boolean = true,
    val error: String? = null
)

@HiltViewModel
class BootstrapViewModel @Inject constructor() : ViewModel() {

    private val _startupState = MutableStateFlow(StartupState())
    val startupState: StateFlow<StartupState> = _startupState.asStateFlow()

    private val _recoveryState = MutableStateFlow(RecoveryState())
    val recoveryState: StateFlow<RecoveryState> = _recoveryState.asStateFlow()

    private val _migrationState = MutableStateFlow(MigrationState())
    val migrationState: StateFlow<MigrationState> = _migrationState.asStateFlow()

    fun startStartup() {
        viewModelScope.launch {
            _startupState.value = StartupState(
                items = StartupPhase.entries.map {
                    StartupProgressItem(it, phaseLabel(it), done = false)
                }
            )
            StartupPhase.entries.forEachIndexed { index, phase ->
                _startupState.value = _startupState.value.copy(currentPhase = phase, progress = (index + 1f) / StartupPhase.entries.size)
                delay(420)
                _startupState.value = _startupState.value.copy(
                    items = _startupState.value.items.mapIndexed { i, item ->
                        if (i == index) item.copy(done = true) else item
                    }
                )
            }
        }
    }

    fun toggleDetail() {
        _startupState.value = _startupState.value.copy(detailExpanded = !_startupState.value.detailExpanded)
    }

    fun retryStartup() {
        startStartup()
    }

    fun simulateStartupFailure() {
        _startupState.value = _startupState.value.copy(
            issue = StartupIssue.ServiceFailed,
            failed = true
        )
    }

    fun loadRecoveryInfo(reason: String, time: String) {
        _recoveryState.value = RecoveryState(
            crashReason = reason,
            crashTime = time
        )
    }

    fun safeBoot(onRecovered: () -> Unit) {
        viewModelScope.launch {
            _recoveryState.value = _recoveryState.value.copy(restoring = true)
            delay(900)
            _recoveryState.value = _recoveryState.value.copy(restoring = false, restored = true)
            onRecovered()
        }
    }

    fun normalBoot(onRecovered: () -> Unit) {
        viewModelScope.launch {
            _recoveryState.value = _recoveryState.value.copy(restoring = true)
            delay(700)
            _recoveryState.value = _recoveryState.value.copy(restoring = false, restored = true)
            onRecovered()
        }
    }

    fun restoreBackup(onDone: () -> Unit) {
        viewModelScope.launch {
            _recoveryState.value = _recoveryState.value.copy(restoring = true)
            delay(1000)
            _recoveryState.value = _recoveryState.value.copy(restoring = false)
            onDone()
        }
    }

    fun loadMigrationInfo(from: String, to: String) {
        _migrationState.value = MigrationState(
            fromVersion = from,
            toVersion = to,
            steps = listOf(
                MigrationStep("备份当前数据", done = false),
                MigrationStep("迁移数据库结构", done = false),
                MigrationStep("迁移记忆索引", done = false),
                MigrationStep("迁移角色配置", done = false),
                MigrationStep("校验数据完整性", done = false)
            )
        )
    }

    fun runMigration(onCompleted: () -> Unit) {
        viewModelScope.launch {
            val total = _migrationState.value.steps.size
            _migrationState.value.steps.indices.forEach { index ->
                _migrationState.value = _migrationState.value.copy(
                    steps = _migrationState.value.steps.mapIndexed { i, step ->
                        when {
                            i < index -> step.copy(done = true, inProgress = false)
                            i == index -> step.copy(inProgress = true)
                            else -> step
                        }
                    },
                    progress = index.toFloat() / total
                )
                delay(700)
                _migrationState.value = _migrationState.value.copy(
                    steps = _migrationState.value.steps.mapIndexed { i, step ->
                        if (i == index) step.copy(done = true, inProgress = false) else step
                    }
                )
            }
            _migrationState.value = _migrationState.value.copy(progress = 1f, completed = true)
            onCompleted()
        }
    }

    fun rollbackMigration() {
        viewModelScope.launch {
            _migrationState.value = _migrationState.value.copy(failed = false, completed = false, progress = 0f)
            loadMigrationInfo(_migrationState.value.fromVersion, _migrationState.value.toVersion)
        }
    }

    private fun phaseLabel(phase: StartupPhase): String = when (phase) {
        StartupPhase.PreparingEnvironment -> "准备本地环境"
        StartupPhase.StartingServices -> "启动服务"
        StartupPhase.ConnectingServices -> "连接服务"
        StartupPhase.RestoringSession -> "恢复会话"
    }
}
