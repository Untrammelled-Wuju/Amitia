package com.amitia.amitia_app.runtime.api

enum class RuntimeProgressStage {
    NONE,
    PREPARING,
    READING_PACKAGE,
    VERIFYING_PACKAGE,
    EXTRACTING,
    APPLYING_PERMISSIONS,
    VERIFYING_INSTALLATION,
    PREPARING_GUEST,
    STARTING_GUEST,
    STARTING_BACKEND,
    WAITING_BACKEND_READY,
    STOPPING_COMPONENTS,
    CLEANING_UP,
    REPAIRING,
    COMPLETED
}

data class RuntimeProgress(
    val stage: RuntimeProgressStage,
    val completedUnits: Long,
    val totalUnits: Long,
    val percent: Int,
    val messageKey: String?
) {
    init {
        require(completedUnits >= 0L) { "completedUnits must not be negative" }
        require(totalUnits >= 0L) { "totalUnits must not be negative" }
        if (totalUnits == 0L) {
            require(percent == 0) { "percent must be 0 when totalUnits is 0" }
        } else {
            require(percent in 0..100) { "percent must be in 0..100" }
        }
    }

    companion object {
        fun none(): RuntimeProgress = RuntimeProgress(
            stage = RuntimeProgressStage.NONE,
            completedUnits = 0,
            totalUnits = 0,
            percent = 0,
            messageKey = null
        )
    }
}
