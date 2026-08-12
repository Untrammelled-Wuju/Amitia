package com.amitia.amitia_app.nativeprovider.share

data class ShareCapabilityState(
    val supported: Boolean = false,
    val canSend: Boolean = false,
    val canReceive: Boolean = false,
    val nativeHostReady: Boolean = false,
    val maxResources: Int = 10,
    val maxSingleResourceBytes: Long = 104857600L,
    val maxTotalBytes: Long = 262144000L,
    val state: String = "unsupported",
)

data class ShareSendRequest(
    val text: String? = null,
    val subject: String? = null,
    val resources: List<ShareResourceRef> = emptyList(),
    val mimeType: String? = null,
    val chooserTitle: String? = null,
)

data class ShareResourceRef(
    val resourceUri: String,
    val mimeType: String? = null,
    val exportToken: String? = null,
)

data class ShareSendResult(
    val status: String = "unavailable",
    val resourceCount: Int = 0,
    val mimeType: String? = null,
    val userActionRequired: Boolean = true,
)

data class IncomingShare(
    val shareId: String,
    val text: String? = null,
    val subject: String? = null,
    val resources: List<SharedIncomingResource> = emptyList(),
    val receivedAt: Long = 0L,
)

data class SharedIncomingResource(
    val resourceUri: String,
    val mimeType: String = "application/octet-stream",
    val displayName: String? = null,
    val sizeBytes: Long = 0L,
)

data class ShareNativeRequest(
    val requestId: String,
    val operation: String,
    val payload: Map<String, Any?> = emptyMap(),
)

data class ShareNativeResponse(
    val requestId: String,
    val status: String,
    val result: Map<String, Any?>? = null,
    val error: ShareNativeError? = null,
)

data class ShareNativeError(
    val code: String,
    val message: String,
    val domainCode: String? = null,
)

data class ShareExportItem(
    val contentUri: android.net.Uri,
    val mimeType: String,
    val displayName: String,
)
