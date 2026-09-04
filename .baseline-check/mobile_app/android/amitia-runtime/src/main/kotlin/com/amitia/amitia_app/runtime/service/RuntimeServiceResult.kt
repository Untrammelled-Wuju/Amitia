package com.amitia.amitia_app.runtime.service

sealed interface RuntimeServiceResult {
    data object Success : RuntimeServiceResult
    data class Failure(val error: RuntimeServiceError) : RuntimeServiceResult
}
