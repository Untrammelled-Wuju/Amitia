package com.amitia.amitia_app.nativeprovider.overlay

internal data class OverlayInfo(
    val overlayId: String,
    val kind: String,
    val visible: Boolean,
    val focusable: Boolean,
    val touchable: Boolean,
    val draggable: Boolean,
    val x: Int,
    val y: Int,
    val width: Int,
    val height: Int,
    val gravity: String,
    val displayId: Int,
    val createdAt: Long,
    val updatedAt: Long,
    val content: Map<String, Any?> = emptyMap(),
)
