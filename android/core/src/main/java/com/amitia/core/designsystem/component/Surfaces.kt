package com.amitia.core.designsystem.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.windowInsetsTopHeight
import androidx.compose.foundation.layout.windowInsetsBottomHeight
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaContentPadding
import com.amitia.core.designsystem.AmitiaElevation
import com.amitia.core.designsystem.AmitiaRadius
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaHeroCardShape
import com.amitia.core.designsystem.AmitiaCardShape

@Composable
fun AmitiaPageScaffold(
    modifier: Modifier = Modifier,
    topBar: @Composable (() -> Unit)? = null,
    bottomBar: @Composable (() -> Unit)? = null,
    contentPadding: PaddingValues = PaddingValues(AmitiaSpacing.None),
    content: @Composable (PaddingValues) -> Unit
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Scaffold(
            topBar = { topBar?.invoke() },
            bottomBar = { bottomBar?.invoke() },
            containerColor = MaterialTheme.colorScheme.background,
            contentWindowInsets = WindowInsets.statusBars,
            content = content
        )
    }
}

@Composable
fun AmitiaContentSurface(
    modifier: Modifier = Modifier,
    shape: androidx.compose.ui.graphics.Shape = AmitiaCardShape,
    tonalElevation: androidx.compose.ui.unit.Dp = AmitiaElevation.Level0,
    content: @Composable () -> Unit
) {
    Surface(
        modifier = modifier,
        shape = shape,
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = tonalElevation,
        content = content
    )
}

@Composable
fun AmitiaElevatedSurface(
    modifier: Modifier = Modifier,
    shape: androidx.compose.ui.graphics.Shape = AmitiaCardShape,
    shadowElevation: androidx.compose.ui.unit.Dp = AmitiaElevation.Level1,
    content: @Composable () -> Unit
) {
    Surface(
        modifier = modifier,
        shape = shape,
        color = MaterialTheme.colorScheme.surfaceVariant,
        tonalElevation = AmitiaElevation.Level1,
        shadowElevation = shadowElevation,
        content = content
    )
}

@Composable
fun AmitiaHeroSurface(
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Surface(
        modifier = modifier,
        shape = AmitiaHeroCardShape,
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = AmitiaElevation.Level1,
        content = content
    )
}

@Composable
fun AmitiaSection(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    headerTrailing: @Composable (() -> Unit)? = null,
    content: @Composable () -> Unit
) {
    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (subtitle != null) {
                    Text(
                        text = subtitle,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            if (headerTrailing != null) {
                headerTrailing()
            }
        }
        content()
    }
}

@Composable
fun AmitiaInsetDivider(
    modifier: Modifier = Modifier,
    leadingInset: androidx.compose.ui.unit.Dp = AmitiaContentPadding.Horizontal,
    trailingInset: androidx.compose.ui.unit.Dp = AmitiaSpacing.None,
    thickness: androidx.compose.ui.unit.Dp = 1.dp
) {
    HorizontalDivider(
        modifier = modifier.fillMaxWidth(),
        thickness = thickness,
        color = MaterialTheme.colorScheme.outlineVariant
    )
}

@Preview(name = "Surfaces - Light", showBackground = true)
@Composable
private fun AmitiaSurfacesLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            AmitiaContentSurface(modifier = Modifier.fillMaxWidth().height(64.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Content Surface", style = MaterialTheme.typography.bodyMedium)
                }
            }
            AmitiaElevatedSurface(modifier = Modifier.fillMaxWidth().height(64.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Elevated Surface", style = MaterialTheme.typography.bodyMedium)
                }
            }
            AmitiaHeroSurface(modifier = Modifier.fillMaxWidth().height(80.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Hero Surface", style = MaterialTheme.typography.titleMedium)
                }
            }
            AmitiaSection(
                title = "Section Title",
                subtitle = "Section subtitle",
                modifier = Modifier.fillMaxWidth()
            ) {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth().height(48.dp)) {
                    Box(
                        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                        contentAlignment = Alignment.CenterStart
                    ) {
                        Text("Section content", style = MaterialTheme.typography.bodyMedium)
                    }
                }
            }
            AmitiaInsetDivider()
        }
    }
}

@Preview(name = "Surfaces - Dark", showBackground = true)
@Composable
private fun AmitiaSurfacesDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            AmitiaContentSurface(modifier = Modifier.fillMaxWidth().height(64.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Content Surface", style = MaterialTheme.typography.bodyMedium)
                }
            }
            AmitiaElevatedSurface(modifier = Modifier.fillMaxWidth().height(64.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Elevated Surface", style = MaterialTheme.typography.bodyMedium)
                }
            }
            AmitiaHeroSurface(modifier = Modifier.fillMaxWidth().height(80.dp)) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base),
                    contentAlignment = Alignment.CenterStart
                ) {
                    Text("Hero Surface", style = MaterialTheme.typography.titleMedium)
                }
            }
        }
    }
}
