package com.amitia.amitia_app.nativeprovider.display

data class DisplayInfo(
    val displayId: Int = 0,
    val name: String = "内置屏幕",
    val width: Int = 0,
    val height: Int = 0,
    val densityDpi: Int = 0,
    val rotation: Int = 0,
    val refreshRate: Float = 60f,
    val isPrimary: Boolean = true,
    val state: String = "unknown",
)

data class DisplayListResult(
    val displays: List<DisplayInfo> = emptyList(),
    val primaryDisplayId: Int = 0,
    val generation: Long = 0L,
)
