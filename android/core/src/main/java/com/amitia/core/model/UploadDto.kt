package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class UploadResponse(
    val url: String,
    val filename: String? = null,
    val size: Long? = null,
    val mimeType: String? = null,
    val width: Int? = null,
    val height: Int? = null,
    val duration: Double? = null
)

@Serializable
data class UploadResult(
    val success: Boolean,
    val url: String? = null,
    val error: String? = null
)
