package com.amitia.amitia_app.nativeprovider.clipboard

data class ClipboardCapabilityState(
    val supported: Boolean = false,
    val canWrite: Boolean = false,
    val canRead: Boolean = false,
    val appForeground: Boolean = false,
    val appHasInputFocus: Boolean = false,
    val readRequiresForeground: Boolean = true,
    val hasPrimaryClip: Boolean = false,
    val supportedMimeTypes: List<String> = listOf("text/plain", "text/html"),
    val maxTextBytes: Int = 65536,
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class ClipboardReadResult(
    val hasContent: Boolean = false,
    val text: String? = null,
    val mimeType: String = "text/plain",
    val itemCount: Int = 0,
    val truncated: Boolean = false,
    val sensitive: Boolean = false,
    val generation: Long = 0L,
)

data class ClipboardWriteRequest(
    val text: String,
    val sensitive: Boolean = false,
)

data class ClipboardNativeRequest(
    val requestId: String,
    val operation: String,
    val payload: Map<String, Any?> = emptyMap(),
)

data class ClipboardNativeResponse(
    val requestId: String,
    val status: String,
    val result: Map<String, Any?>? = null,
    val error: ClipboardNativeError? = null,
)

data class ClipboardNativeError(
    val code: String,
    val message: String,
    val domainCode: String? = null,
)
