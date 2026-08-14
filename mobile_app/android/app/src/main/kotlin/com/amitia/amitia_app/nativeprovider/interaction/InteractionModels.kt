package com.amitia.amitia_app.nativeprovider.interaction

data class InteractionTapRequest(
    val x: Int = -1,
    val y: Int = -1,
    val durationMs: Long = 50,
)

data class InteractionSwipeRequest(
    val startX: Int = -1,
    val startY: Int = -1,
    val endX: Int = -1,
    val endY: Int = -1,
    val durationMs: Long = 300,
)

data class InteractionInputTextRequest(
    val nodeId: String? = null,
    val text: String = "",
    val clearFirst: Boolean = false,
)

data class InteractionScrollRequest(
    val direction: String = "down",
    val percent: Float = 0.8f,
)

data class InteractionActionResult(
    val performed: Boolean = false,
    val actionLabel: String? = null,
)
