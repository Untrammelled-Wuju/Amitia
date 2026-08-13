package com.amitia.amitia_app.runtime.install

internal data class ActiveRuntimeInfo(
    val version: String,
    val activatedAtEpochMillis: Long,
)

internal data class ActiveProgramRoot(
    val runtimeVersion: String,
    val hostDirectory: java.io.File,
    val manifestIdentity: String,
)

internal sealed interface ActiveProgramRootResult {
    data class Ready(val root: ActiveProgramRoot) : ActiveProgramRootResult
    object NoActiveRuntime : ActiveProgramRootResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : ActiveProgramRootResult
}

internal sealed interface ActiveRuntimeResult {
    data class Active(val info: ActiveRuntimeInfo) : ActiveRuntimeResult
    object NoActiveRuntime : ActiveRuntimeResult
    data class Failure(
        val code: RuntimeInstallErrorCode,
        val message: String,
    ) : ActiveRuntimeResult
}

internal interface ActiveRuntimeManager {
    fun current(): ActiveRuntimeResult
    fun activate(version: String): ActiveRuntimeResult
    fun resolveActiveProgramRoot(): ActiveProgramRootResult
}
