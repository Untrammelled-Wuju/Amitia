package com.amitia.core.designsystem

import android.os.Build
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.compositionLocalOf

enum class GlassLevel {
    Navigation,
    Sheet,
    Chip
}

data class GlassConfig(
    val fillColor: Color,
    val borderColor: Color,
    val innerBorderColor: Color,
    val blurRadius: Dp,
    val cornerRadius: Dp,
    val shadowElevation: Dp
)

object GlassPerformancePolicy {
    fun shouldUseRealBlur(
        sdkInt: Int,
        isLowPowerMode: Boolean,
        isBlurEnabled: Boolean
    ): Boolean {
        if (!isBlurEnabled) return false
        if (isLowPowerMode) return false
        return sdkInt >= Build.VERSION_CODES.S
    }

    fun maxSimultaneousBlurLayers(): Int = 2
}

val LocalGlassConfig = compositionLocalOf<GlassConfig> { error("GlassConfig not provided") }

@Composable
fun amitiaGlassConfig(level: GlassLevel, isDark: Boolean): GlassConfig {
    return remember(level, isDark) {
        when (level) {
            GlassLevel.Navigation -> {
                GlassConfig(
                    fillColor = if (isDark) AmitiaDarkColors.GlassNavigation else AmitiaLightColors.GlassNavigation,
                    borderColor = if (isDark) AmitiaDarkColors.GlassBorder else AmitiaLightColors.GlassBorder,
                    innerBorderColor = if (isDark) AmitiaDarkColors.GlassInnerBorder else AmitiaLightColors.GlassInnerBorder,
                    blurRadius = 32.dp,
                    cornerRadius = AmitiaRadius.XXL,
                    shadowElevation = AmitiaElevation.Level2Blur
                )
            }
            GlassLevel.Sheet -> {
                GlassConfig(
                    fillColor = if (isDark) AmitiaDarkColors.GlassSheet else AmitiaLightColors.GlassSheet,
                    borderColor = if (isDark) AmitiaDarkColors.GlassBorder else AmitiaLightColors.GlassBorder,
                    innerBorderColor = if (isDark) AmitiaDarkColors.GlassInnerBorder else AmitiaLightColors.GlassInnerBorder,
                    blurRadius = 24.dp,
                    cornerRadius = AmitiaRadius.XXL,
                    shadowElevation = AmitiaElevation.Level2Blur
                )
            }
            GlassLevel.Chip -> {
                GlassConfig(
                    fillColor = if (isDark) AmitiaDarkColors.GlassChip else AmitiaLightColors.GlassChip,
                    borderColor = if (isDark) AmitiaDarkColors.GlassBorder else AmitiaLightColors.GlassBorder,
                    innerBorderColor = if (isDark) AmitiaDarkColors.GlassInnerBorder else AmitiaLightColors.GlassInnerBorder,
                    blurRadius = 12.dp,
                    cornerRadius = AmitiaRadius.Pill,
                    shadowElevation = 0.dp
                )
            }
        }
    }
}

@Composable
fun AmitiaGlassSurface(
    level: GlassLevel,
    modifier: Modifier = Modifier,
    shape: Shape? = null,
    enabled: Boolean = true,
    isBlurEnabled: Boolean = true,
    content: @Composable () -> Unit
) {
    val isDark = MaterialTheme.colorScheme.background.luminance() < 0.5f
    val config = amitiaGlassConfig(level, isDark)
    val finalShape = shape ?: RoundedCornerShape(config.cornerRadius)

    Box(
        modifier = modifier
            .clip(finalShape)
            .background(config.fillColor)
            .border(1.dp, config.borderColor, finalShape)
    ) {
        CompositionLocalProvider(LocalGlassConfig provides config) {
            content()
        }
    }
}

private fun Color.luminance(): Float {
    val r = (this.red * 0.2126f)
    val g = (this.green * 0.7152f)
    val b = (this.blue * 0.0722f)
    return r + g + b
}
