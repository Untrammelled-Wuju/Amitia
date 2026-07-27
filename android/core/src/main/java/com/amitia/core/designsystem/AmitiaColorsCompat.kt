package com.amitia.core.designsystem

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

object AmitiaColors {

    val Background: Color @Composable get() = MaterialTheme.colorScheme.background
    val BackgroundElevated: Color @Composable get() = MaterialTheme.colorScheme.surfaceVariant
    val Surface: Color @Composable get() = MaterialTheme.colorScheme.surface
    val SurfaceVariant: Color @Composable get() = MaterialTheme.colorScheme.surfaceVariant
    val SurfaceContainer: Color @Composable get() = MaterialTheme.colorScheme.surfaceVariant
    val SurfaceContainerHigh: Color @Composable get() = MaterialTheme.colorScheme.surfaceVariant

    val OnBackground: Color @Composable get() = MaterialTheme.colorScheme.onBackground
    val OnSurface: Color @Composable get() = MaterialTheme.colorScheme.onSurface
    val OnSurfaceVariant: Color @Composable get() = MaterialTheme.colorScheme.onSurfaceVariant
    val OnSurfaceMuted: Color @Composable get() = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)

    val Primary: Color @Composable get() = MaterialTheme.colorScheme.primary
    val OnPrimary: Color @Composable get() = MaterialTheme.colorScheme.onPrimary
    val PrimaryContainer: Color @Composable get() = MaterialTheme.colorScheme.primaryContainer
    val OnPrimaryContainer: Color @Composable get() = MaterialTheme.colorScheme.onPrimaryContainer

    val Secondary: Color @Composable get() = MaterialTheme.colorScheme.secondary
    val OnSecondary: Color @Composable get() = MaterialTheme.colorScheme.onSecondary
    val SecondaryContainer: Color @Composable get() = MaterialTheme.colorScheme.secondaryContainer
    val OnSecondaryContainer: Color @Composable get() = MaterialTheme.colorScheme.onSecondaryContainer

    val Tertiary: Color @Composable get() = MaterialTheme.colorScheme.tertiary
    val OnTertiary: Color @Composable get() = MaterialTheme.colorScheme.onTertiary
    val TertiaryContainer: Color @Composable get() = MaterialTheme.colorScheme.tertiaryContainer
    val OnTertiaryContainer: Color @Composable get() = MaterialTheme.colorScheme.onTertiaryContainer

    val Error: Color @Composable get() = MaterialTheme.colorScheme.error
    val OnError: Color @Composable get() = MaterialTheme.colorScheme.onError
    val ErrorContainer: Color @Composable get() = MaterialTheme.colorScheme.errorContainer
    val OnErrorContainer: Color @Composable get() = MaterialTheme.colorScheme.onErrorContainer

    val Outline: Color @Composable get() = MaterialTheme.colorScheme.outline
    val OutlineVariant: Color @Composable get() = MaterialTheme.colorScheme.outlineVariant
    val Divider: Color @Composable get() = MaterialTheme.colorScheme.outlineVariant
    val Border: Color @Composable get() = MaterialTheme.colorScheme.outline
    val Scrim: Color @Composable get() = MaterialTheme.colorScheme.scrim

    val GlassTint: Color @Composable get() = MaterialTheme.colorScheme.surface.copy(alpha = 0.7f)
    val GlassBorder: Color @Composable get() = Color.White.copy(alpha = 0.14f)

    val StateRunning: Color @Composable get() = AmitiaStateColors.Running
    val StateDegraded: Color @Composable get() = AmitiaStateColors.Degraded
    val StateFailed: Color @Composable get() = AmitiaStateColors.Failed
    val StateInstalling: Color @Composable get() = AmitiaStateColors.Installing
    val StateIdle: Color @Composable get() = AmitiaStateColors.Idle

    val Overlay: Color @Composable get() = MaterialTheme.colorScheme.scrim.copy(alpha = 0.6f)
}
