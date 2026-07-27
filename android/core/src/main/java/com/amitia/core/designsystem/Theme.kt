package com.amitia.core.designsystem

import android.app.Activity
import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

enum class AmitiaAppearance {
    Light, Dark, System
}

enum class BlurStrength {
    Standard, Reduced, Off
}

data class AmitiaThemeConfig(
    val appearance: AmitiaAppearance = AmitiaAppearance.System,
    val dynamicColor: Boolean = false,
    val blurStrength: BlurStrength = BlurStrength.Standard,
    val highContrast: Boolean = false,
    val reduceMotion: Boolean = false
)

val LocalThemeConfig = compositionLocalOf { AmitiaThemeConfig() }
val LocalIsDarkTheme = compositionLocalOf { true }
val LocalIsBlurEnabled = compositionLocalOf { true }

private val AmitiaLightColorScheme = lightColorScheme(
    primary = AmitiaLightColors.Primary,
    onPrimary = AmitiaLightColors.OnPrimary,
    primaryContainer = AmitiaLightColors.PrimaryContainer,
    onPrimaryContainer = AmitiaLightColors.OnPrimaryContainer,
    secondary = AmitiaLightColors.Secondary,
    onSecondary = AmitiaLightColors.OnPrimary,
    tertiary = AmitiaLightColors.Tertiary,
    onTertiary = AmitiaLightColors.OnPrimary,
    error = AmitiaLightColors.Error,
    onError = AmitiaLightColors.OnPrimary,
    background = AmitiaLightColors.Background,
    onBackground = AmitiaLightColors.TextPrimary,
    surface = AmitiaLightColors.Surface,
    onSurface = AmitiaLightColors.TextPrimary,
    surfaceVariant = AmitiaLightColors.SurfaceContainer,
    onSurfaceVariant = AmitiaLightColors.TextSecondary,
    surfaceTint = AmitiaLightColors.Primary,
    outline = AmitiaLightColors.Outline,
    outlineVariant = AmitiaLightColors.Divider,
    scrim = AmitiaLightColors.Scrim
)

private val AmitiaDarkColorScheme = darkColorScheme(
    primary = AmitiaDarkColors.Primary,
    onPrimary = AmitiaDarkColors.OnPrimary,
    primaryContainer = AmitiaDarkColors.PrimaryContainer,
    onPrimaryContainer = AmitiaDarkColors.OnPrimaryContainer,
    secondary = AmitiaDarkColors.Secondary,
    onSecondary = AmitiaDarkColors.OnPrimary,
    tertiary = AmitiaDarkColors.Tertiary,
    onTertiary = AmitiaDarkColors.OnPrimary,
    error = AmitiaDarkColors.Error,
    onError = AmitiaDarkColors.OnPrimary,
    background = AmitiaDarkColors.Background,
    onBackground = AmitiaDarkColors.TextPrimary,
    surface = AmitiaDarkColors.Surface,
    onSurface = AmitiaDarkColors.TextPrimary,
    surfaceVariant = AmitiaDarkColors.SurfaceContainer,
    onSurfaceVariant = AmitiaDarkColors.TextSecondary,
    surfaceTint = AmitiaDarkColors.Primary,
    outline = AmitiaDarkColors.Outline,
    outlineVariant = AmitiaDarkColors.Divider,
    scrim = AmitiaDarkColors.Scrim
)

@Composable
fun AmitiaTheme(
    config: AmitiaThemeConfig = AmitiaThemeConfig(),
    content: @Composable () -> Unit
) {
    val isDark = when (config.appearance) {
        AmitiaAppearance.Light -> false
        AmitiaAppearance.Dark -> true
        AmitiaAppearance.System -> isSystemInDarkTheme()
    }

    val isBlurEnabled = config.blurStrength != BlurStrength.Off

    val colorScheme = when {
        config.dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            val context = LocalView.current.context
            if (isDark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        }
        isDark -> AmitiaDarkColorScheme
        else -> AmitiaLightColorScheme
    }

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars = !isDark
            WindowCompat.getInsetsController(window, view).isAppearanceLightNavigationBars = !isDark
        }
    }

    CompositionLocalProvider(
        LocalThemeConfig provides config,
        LocalIsDarkTheme provides isDark,
        LocalIsBlurEnabled provides isBlurEnabled
    ) {
        MaterialTheme(
            colorScheme = colorScheme,
            typography = AmitiaTypography,
            shapes = AmitiaShapes,
            content = content
        )
    }
}

@Composable
fun AmitiaTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    AmitiaTheme(
        config = AmitiaThemeConfig(
            appearance = if (darkTheme) AmitiaAppearance.Dark else AmitiaAppearance.Light
        ),
        content = content
    )
}
