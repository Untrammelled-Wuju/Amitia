package com.amitia.core.designsystem

sealed interface ScreenState<out T> {
    data object Loading : ScreenState<Nothing>
    data class Content<T>(val data: T) : ScreenState<T>
    data class Empty(val reason: EmptyReason = EmptyReason.NoData) : ScreenState<Nothing>
    data class Error(val error: UiError) : ScreenState<Nothing>
    data class Partial<T>(val data: T, val warnings: List<UiWarning> = emptyList()) : ScreenState<T>
}

enum class EmptyReason {
    NoData,
    NoResults,
    NotConnected,
    PermissionDenied,
    RuntimeUnavailable,
    ComingSoon
}

data class UiError(
    val title: String,
    val message: String,
    val type: ErrorType = ErrorType.Unknown,
    val retryable: Boolean = true,
    val actionLabel: String? = null,
    val cause: Throwable? = null
)

enum class ErrorType {
    Network,
    Authentication,
    Permission,
    RuntimeUnavailable,
    ServicePartialFailure,
    DataExpired,
    OperationFailed,
    Unknown
}

data class UiWarning(
    val message: String,
    val type: WarningType = WarningType.Info,
    val dismissible: Boolean = true
)

enum class WarningType {
    Info,
    Warning,
    Degraded,
    Offline
}

enum class DangerLevel {
    One,
    Two,
    Three
}

data class AmitiaPageState(
    val isLoading: Boolean = false,
    val isRefreshing: Boolean = false,
    val isOffline: Boolean = false,
    val isRuntimeUnavailable: Boolean = false,
    val partialFailures: List<UiWarning> = emptyList()
)
