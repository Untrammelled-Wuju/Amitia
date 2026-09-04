package com.amitia.amitia_app.nativeprovider.accessibility

data class NativeAccessibilityRequest(
    val requestId: String,
    val operation: String,
    val payload: Map<String, Any?> = emptyMap(),
)

data class NativeAccessibilityResponse(
    val requestId: String,
    val status: String,
    val result: Map<String, Any?>? = null,
    val error: NativeAccessibilityError? = null,
)

data class NativeAccessibilityError(
    val code: String,
    val message: String,
    val domainCode: String? = null,
)

data class AccessibilityCapabilityState(
    val platformSupported: Boolean,
    val serviceDeclared: Boolean,
    val enabledInSettings: Boolean,
    val connected: Boolean,
    val canRetrieveWindowContent: Boolean,
    val canRetrieveInteractiveWindows: Boolean,
    val userActionRequired: Boolean,
    val state: String,
)
