package com.amitia.core.designsystem.component

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaInputShape
import com.amitia.core.designsystem.AmitiaMotion
import com.amitia.core.designsystem.AmitiaMotionDuration
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSmallButtonShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget
import com.amitia.core.designsystem.GlassLevel

enum class AmitiaButtonState {
    Default, Loading, Success, Failure
}

@Composable
fun PrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    state: AmitiaButtonState = AmitiaButtonState.Default,
    leadingIcon: ImageVector? = null
) {
    val effectiveEnabled = enabled && state == AmitiaButtonState.Default
    val containerColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.tertiary
        AmitiaButtonState.Failure -> MaterialTheme.colorScheme.error
        else -> MaterialTheme.colorScheme.primary
    }
    val contentColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.onTertiary
        AmitiaButtonState.Failure -> MaterialTheme.colorScheme.onError
        else -> MaterialTheme.colorScheme.onPrimary
    }

    Button(
        onClick = onClick,
        modifier = modifier.height(AmitiaTouchTarget.Minimum),
        enabled = effectiveEnabled || state != AmitiaButtonState.Default,
        shape = AmitiaInputShape,
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
            disabledContentColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.38f)
        ),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Lg,
            vertical = AmitiaSpacing.Sm
        )
    ) {
        ButtonContent(
            text = text,
            state = state,
            leadingIcon = leadingIcon,
            contentColor = contentColor
        )
    }
}

@Composable
fun SecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    state: AmitiaButtonState = AmitiaButtonState.Default,
    leadingIcon: ImageVector? = null
) {
    val effectiveEnabled = enabled && state == AmitiaButtonState.Default
    val containerColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.tertiaryContainer
        AmitiaButtonState.Failure -> MaterialTheme.colorScheme.errorContainer
        else -> MaterialTheme.colorScheme.surfaceVariant
    }
    val contentColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.onTertiaryContainer
        AmitiaButtonState.Failure -> MaterialTheme.colorScheme.onErrorContainer
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }

    Button(
        onClick = onClick,
        modifier = modifier.height(AmitiaTouchTarget.Minimum),
        enabled = effectiveEnabled || state != AmitiaButtonState.Default,
        shape = AmitiaInputShape,
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.38f),
            disabledContentColor = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.38f)
        ),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Lg,
            vertical = AmitiaSpacing.Sm
        )
    ) {
        ButtonContent(
            text = text,
            state = state,
            leadingIcon = leadingIcon,
            contentColor = contentColor
        )
    }
}

@Composable
fun TertiaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    leadingIcon: ImageVector? = null
) {
    val interactionSource = remember { MutableInteractionSource() }
    val textColor = if (enabled) MaterialTheme.colorScheme.primary
    else MaterialTheme.colorScheme.primary.copy(alpha = 0.38f)

    Box(
        modifier = modifier
            .height(AmitiaTouchTarget.Minimum)
            .clip(AmitiaPillShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClick = onClick
            )
            .padding(horizontal = AmitiaSpacing.Md),
        contentAlignment = Alignment.Center
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            if (leadingIcon != null) {
                Icon(
                    imageVector = leadingIcon,
                    contentDescription = null,
                    tint = textColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Text(
                text = text,
                style = MaterialTheme.typography.labelLarge,
                color = textColor
            )
        }
    }
}

@Composable
fun DangerButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    state: AmitiaButtonState = AmitiaButtonState.Default,
    leadingIcon: ImageVector? = null
) {
    val effectiveEnabled = enabled && state == AmitiaButtonState.Default
    val containerColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.error
    }
    val contentColor = when (state) {
        AmitiaButtonState.Success -> MaterialTheme.colorScheme.onTertiary
        else -> MaterialTheme.colorScheme.onError
    }

    Button(
        onClick = onClick,
        modifier = modifier.height(AmitiaTouchTarget.Minimum),
        enabled = effectiveEnabled || state != AmitiaButtonState.Default,
        shape = AmitiaInputShape,
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = MaterialTheme.colorScheme.error.copy(alpha = 0.38f),
            disabledContentColor = MaterialTheme.colorScheme.onError.copy(alpha = 0.38f)
        ),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Lg,
            vertical = AmitiaSpacing.Sm
        )
    ) {
        ButtonContent(
            text = text,
            state = state,
            leadingIcon = leadingIcon,
            contentColor = contentColor
        )
    }
}

@Composable
fun AmitiaIconButton(
    icon: ImageVector,
    contentDescription: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    tint: Color = MaterialTheme.colorScheme.onSurfaceVariant
) {
    val interactionSource = remember { MutableInteractionSource() }
    val effectiveTint = if (enabled) tint else tint.copy(alpha = 0.38f)
    Box(
        modifier = modifier
            .size(AmitiaTouchTarget.Minimum)
            .clip(CircleShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = icon,
            contentDescription = contentDescription,
            tint = effectiveTint,
            modifier = Modifier.size(AmitiaIconSize.Nav)
        )
    }
}

@Composable
fun GlassIconButton(
    icon: ImageVector,
    contentDescription: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true
) {
    val interactionSource = remember { MutableInteractionSource() }
    val tint = if (enabled) MaterialTheme.colorScheme.onSurface
    else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f)
    AmitiaGlassSurface(
        level = GlassLevel.Chip,
        modifier = modifier.size(AmitiaTouchTarget.Minimum),
        enabled = enabled
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(AmitiaTouchTarget.Minimum)
                .clip(AmitiaPillShape)
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    enabled = enabled,
                    role = Role.Button,
                    onClick = onClick
                ),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = icon,
                contentDescription = contentDescription,
                tint = tint,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
    }
}

@Composable
fun LoadingButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    leadingIcon: ImageVector? = null
) {
    PrimaryButton(
        text = text,
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        state = if (loading) AmitiaButtonState.Loading else AmitiaButtonState.Default,
        leadingIcon = leadingIcon
    )
}

@Composable
fun SplitActionButton(
    text: String,
    onClick: () -> Unit,
    onMenuClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    leadingIcon: ImageVector? = null
) {
    val effectiveEnabled = enabled && !loading
    Row(
        modifier = modifier
            .height(AmitiaTouchTarget.Minimum)
            .clip(AmitiaInputShape),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
    ) {
        Button(
            onClick = onClick,
            modifier = Modifier.weight(1f).fillMaxWidth(),
            enabled = effectiveEnabled,
            shape = AmitiaInputShape,
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary,
                disabledContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
                disabledContentColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.38f)
            ),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(
                horizontal = AmitiaSpacing.Lg,
                vertical = AmitiaSpacing.Sm
            )
        ) {
            ButtonContent(
                text = text,
                state = if (loading) AmitiaButtonState.Loading else AmitiaButtonState.Default,
                leadingIcon = leadingIcon,
                contentColor = MaterialTheme.colorScheme.onPrimary
            )
        }
        Button(
            onClick = onMenuClick,
            enabled = effectiveEnabled,
            shape = AmitiaInputShape,
            colors = ButtonDefaults.buttonColors(
                containerColor = MaterialTheme.colorScheme.primary,
                contentColor = MaterialTheme.colorScheme.onPrimary,
                disabledContainerColor = MaterialTheme.colorScheme.primary.copy(alpha = 0.38f),
                disabledContentColor = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.38f)
            ),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(
                horizontal = AmitiaSpacing.Sm
            )
        ) {
            Icon(
                imageVector = AmitiaIcons.ArrowDropDown,
                contentDescription = "更多操作",
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

@Composable
private fun ButtonContent(
    text: String,
    state: AmitiaButtonState,
    leadingIcon: ImageVector?,
    contentColor: Color
) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        when (state) {
            AmitiaButtonState.Loading -> {
                CircularProgressIndicator(
                    modifier = Modifier.size(AmitiaIconSize.Medium),
                    strokeWidth = 2.dp,
                    color = contentColor
                )
            }
            AmitiaButtonState.Success -> {
                AnimatedVisibility(
                    visible = true,
                    enter = scaleIn(animationSpec = tween(AmitiaMotionDuration.Standard)) +
                        fadeIn(animationSpec = tween(AmitiaMotionDuration.Standard)),
                    exit = scaleOut() + fadeOut()
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = contentColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
            AmitiaButtonState.Failure -> {
                Icon(
                    imageVector = AmitiaIcons.Close,
                    contentDescription = null,
                    tint = contentColor,
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            AmitiaButtonState.Default -> {
                if (leadingIcon != null) {
                    Icon(
                        imageVector = leadingIcon,
                        contentDescription = null,
                        tint = contentColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
        }
        Text(
            text = if (state == AmitiaButtonState.Loading) "处理中..." else text,
            style = MaterialTheme.typography.labelLarge,
            color = contentColor,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Preview(name = "Buttons - Light", showBackground = true)
@Composable
private fun AmitiaButtonsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            PrimaryButton(text = "Primary", onClick = {}, modifier = Modifier.fillMaxWidth())
            SecondaryButton(text = "Secondary", onClick = {}, modifier = Modifier.fillMaxWidth())
            TertiaryButton(text = "Tertiary", onClick = {})
            DangerButton(text = "Danger", onClick = {}, modifier = Modifier.fillMaxWidth())
            LoadingButton(text = "Loading", onClick = {}, loading = true, modifier = Modifier.fillMaxWidth())
            SplitActionButton(text = "Split", onClick = {}, onMenuClick = {}, modifier = Modifier.fillMaxWidth())
            Row(
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
                verticalAlignment = Alignment.CenterVertically
            ) {
                AmitiaIconButton(icon = AmitiaIcons.Add, contentDescription = "Add", onClick = {})
                GlassIconButton(icon = AmitiaIcons.Search, contentDescription = "Search", onClick = {})
            }
        }
    }
}

@Preview(name = "Buttons - Dark", showBackground = true)
@Composable
private fun AmitiaButtonsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            PrimaryButton(text = "Primary", onClick = {}, modifier = Modifier.fillMaxWidth())
            SecondaryButton(text = "Secondary", onClick = {}, modifier = Modifier.fillMaxWidth())
            DangerButton(text = "Danger", onClick = {}, modifier = Modifier.fillMaxWidth())
            PrimaryButton(
                text = "Success",
                onClick = {},
                state = AmitiaButtonState.Success,
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}
