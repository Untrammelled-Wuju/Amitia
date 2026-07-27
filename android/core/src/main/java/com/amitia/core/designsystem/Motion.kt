package com.amitia.core.designsystem

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.tween

object AmitiaMotionDuration {
    const val Instant = 90
    const val Micro = 140
    const val Standard = 220
    const val Expand = 280
    const val Navigation = 320
    const val Character = 420
    const val Immersive = 480
}

val StandardEasing = CubicBezierEasing(0.2f, 0.0f, 0.0f, 1.0f)
val EmphasizedEasing = CubicBezierEasing(0.2f, 0.0f, 0.0f, 1.0f)
val ExitEasing = CubicBezierEasing(0.3f, 0.0f, 1.0f, 1.0f)

object AmitiaMotion {
    fun <T> standardTween() = tween<T>(
        durationMillis = AmitiaMotionDuration.Standard,
        easing = StandardEasing
    )

    fun <T> navigationTween() = tween<T>(
        durationMillis = AmitiaMotionDuration.Navigation,
        easing = StandardEasing
    )

    fun <T> expandTween() = tween<T>(
        durationMillis = AmitiaMotionDuration.Expand,
        easing = EmphasizedEasing
    )

    fun <T> characterTween() = tween<T>(
        durationMillis = AmitiaMotionDuration.Character,
        easing = EmphasizedEasing
    )

    fun <T> instantTween() = tween<T>(
        durationMillis = AmitiaMotionDuration.Instant,
        easing = StandardEasing
    )
}
