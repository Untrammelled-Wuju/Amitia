package com.amitia.platform.bridge

sealed class NativeActionResult {

    data class Success(val data: Map<String, String> = emptyMap()) : NativeActionResult()

    data class Denied(val permission: String, val reason: String) : NativeActionResult()

    data class Failed(val error: String, val cause: Throwable? = null) : NativeActionResult()
}
