package com.amitia.core.designsystem

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

private val DarkColorScheme = darkColorScheme(
    primary = AmitiaColors.Primary,
    onPrimary = AmitiaColors.OnPrimary,
    primaryContainer = AmitiaColors.PrimaryContainer,
    onPrimaryContainer = AmitiaColors.OnPrimaryContainer,
    secondary = AmitiaColors.Secondary,
    onSecondary = AmitiaColors.OnSecondary,
    secondaryContainer = AmitiaColors.SecondaryContainer,
    onSecondaryContainer = AmitiaColors.OnSecondaryContainer,
    tertiary = AmitiaColors.Tertiary,
    onTertiary = AmitiaColors.OnTertiary,
    tertiaryContainer = AmitiaColors.TertiaryContainer,
    onTertiaryContainer = AmitiaColors.OnTertiaryContainer,
    error = AmitiaColors.Error,
    onError = AmitiaColors.OnError,
    errorContainer = AmitiaColors.ErrorContainer,
    onErrorContainer = AmitiaColors.OnErrorContainer,
    background = AmitiaColors.Background,
    onBackground = AmitiaColors.OnBackground,
    surface = AmitiaColors.Surface,
    onSurface = AmitiaColors.OnSurface,
    surfaceVariant = AmitiaColors.SurfaceVariant,
    onSurfaceVariant = AmitiaColors.OnSurfaceVariant,
    surfaceTint = AmitiaColors.Primary,
    inverseSurface = AmitiaColors.SurfaceContainerHigh,
    inverseOnSurface = AmitiaColors.OnBackground,
    outline = AmitiaColors.Outline,
    outlineVariant = AmitiaColors.OutlineVariant,
    scrim = AmitiaColors.Scrim
)

private val LightColorScheme = lightColorScheme(
    primary = Color(0xFF4A5C72),
    onPrimary = Color(0xFFFFFFFF),
    primaryContainer = Color(0xFFD2DEEE),
    onPrimaryContainer = Color(0xFF15233A),
    secondary = Color(0xFF6F6750),
    onSecondary = Color(0xFFFFFFFF),
    secondaryContainer = Color(0xFFFAEBD0),
    onSecondaryContainer = Color(0xFF25200F),
    tertiary = Color(0xFF527068),
    onTertiary = Color(0xFFFFFFFF),
    tertiaryContainer = Color(0xFFA6CFC2),
    onTertiaryContainer = Color(0xFF0E2520),
    error = Color(0xFF8C4A4A),
    onError = Color(0xFFFFFFFF),
    errorContainer = Color(0xFFE3C2C2),
    onErrorContainer = Color(0xFF2A0F0F),
    background = Color(0xFFF7F8FA),
    onBackground = Color(0xFF1A1D24),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF1A1D24),
    surfaceVariant = Color(0xFFEAECF0),
    onSurfaceVariant = Color(0xFF4D525C),
    outline = Color(0xFF7F8590),
    outlineVariant = Color(0xFFC8CCD4)
)

@Composable
fun AmitiaTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme
    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            WindowCompat.getInsetsController(window, view).isAppearanceLightStatusBars = !darkTheme
            WindowCompat.getInsetsController(window, view).isAppearanceLightNavigationBars = !darkTheme
        }
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = AmitiaTypography,
        content = content
    )
}
