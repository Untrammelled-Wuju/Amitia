package com.amitia.runtime.bootstrap

import com.amitia.runtime.api.RuntimeServices
import com.amitia.runtime.api.RuntimeStage

interface BootstrapSequence {

    suspend fun start(progress: (RuntimeStage) -> Unit): Result<RuntimeServices>

    suspend fun stop(progress: (RuntimeStage) -> Unit): Result<Unit>

    suspend fun restart(): Result<RuntimeServices>

    suspend fun repair(): Result<Unit>
}
