package com.amitia.platform.bridge

data class NativeActionRequest(
    val action: String,
    val params: Map<String, String> = emptyMap(),
    val requiresPermission: String? = null
)
