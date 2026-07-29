package com.amitia.feature.onboarding.steps

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.draw.scale
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.LocalIsDarkTheme
import kotlin.math.cos
import kotlin.math.sin

private val OnboardingPrimary = Color(0xFF6B7F74)
private val OnboardingPrimarySoft = Color(0xFF89A091)
private val OnboardingPrimaryDark = Color(0xFF2F3B35)
private val OnboardingWarm = Color(0xFFB48A6D)
private val OnboardingSuccess = Color(0xFF5E836F)
private val OnboardingMuted = Color(0xFF6A706B)
private val OnboardingMutedSecondary = Color(0xFF949A95)
private val OnboardingText = Color(0xFF171A18)

private val DarkPrimary = Color(0xFFA6BBAF)
private val DarkPrimarySoft = Color(0xFFC7D7CF)
private val DarkPrimaryDark = Color(0xFFDCE9E1)
private val DarkWarm = Color(0xFFC49A7D)
private val DarkSuccess = Color(0xFF7FB28E)
private val DarkMuted = Color(0xFFA4AAA6)
private val DarkMutedSecondary = Color(0xFF747A76)
private val DarkText = Color(0xFFF2F4F2)
private val DarkBg = Color(0xFF0A0C0B)

@Composable
private fun primaryColor() = if (LocalIsDarkTheme.current) DarkPrimary else OnboardingPrimary
@Composable
private fun primarySoftColor() = if (LocalIsDarkTheme.current) DarkPrimarySoft else OnboardingPrimarySoft
@Composable
private fun primaryDarkColor() = if (LocalIsDarkTheme.current) DarkPrimaryDark else OnboardingPrimaryDark
@Composable
private fun warmColor() = if (LocalIsDarkTheme.current) DarkWarm else OnboardingWarm
@Composable
private fun successColor() = if (LocalIsDarkTheme.current) DarkSuccess else OnboardingSuccess
@Composable
private fun mutedColor() = if (LocalIsDarkTheme.current) DarkMuted else OnboardingMuted
@Composable
private fun mutedSecondaryColor() = if (LocalIsDarkTheme.current) DarkMutedSecondary else OnboardingMutedSecondary
@Composable
private fun textColor() = if (LocalIsDarkTheme.current) DarkText else OnboardingText
@Composable
private fun bgColor() = if (LocalIsDarkTheme.current) DarkBg else Color(0xFFF3F1EC)

@Composable
private fun glassBgColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xD91B211D) else Color(0x2EFFFFFF)

@Composable
private fun glassBorderColor(): Color =
    if (LocalIsDarkTheme.current) Color(0x14FFFFFF) else Color(0x48FFFFFF)

@Composable
private fun glassBorderSoftColor(): Color =
    if (LocalIsDarkTheme.current) Color(0x08FFFFFF) else Color(0x28FFFFFF)

@Composable
private fun glassSelectedBgColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xAE2A3931) else Color(0x38FFFFFF)

@Composable
fun characterSizeForStep(step: OnboardingFlowStep): Int = when (step) {
    OnboardingFlowStep.Welcome -> 278
    OnboardingFlowStep.ModeSelection -> 205
    OnboardingFlowStep.Environment -> 176
    OnboardingFlowStep.Account -> 140
    OnboardingFlowStep.ModelConfig -> 140
    OnboardingFlowStep.CharacterSetup -> 140
    OnboardingFlowStep.UserInfo -> 140
    OnboardingFlowStep.Complete -> 220
}

@Composable
fun contentTopPaddingForStep(step: OnboardingFlowStep): Int = when (step) {
    OnboardingFlowStep.Welcome -> 365
    OnboardingFlowStep.ModeSelection -> 245
    OnboardingFlowStep.Environment -> 220
    OnboardingFlowStep.Account -> 175
    OnboardingFlowStep.ModelConfig -> 175
    OnboardingFlowStep.CharacterSetup -> 175
    OnboardingFlowStep.UserInfo -> 175
    OnboardingFlowStep.Complete -> 326
}

@Composable
fun OnboardingScaffold(
    currentStep: OnboardingFlowStep,
    transitioning: Boolean,
    preparingEntry: Boolean,
    onBack: (() -> Unit)?,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    val bg = bgColor()
    val isDark = LocalIsDarkTheme.current
    val pColor = primaryColor()

    Box(
        modifier = modifier
            .fillMaxSize()
            .drawBehind {
                drawRect(bg)
                drawRect(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            pColor.copy(alpha = 0.16f),
                            Color.Transparent
                        ),
                        center = Offset(size.width * 0.5f, size.height * 0.12f),
                        radius = size.width * 0.85f
                    )
                )
            }
    ) {
        AmbientBackground(modifier = Modifier.fillMaxSize())

        if (onBack != null && !preparingEntry) {
            GlassBackButton(
                onClick = onBack,
                modifier = Modifier
                    .padding(top = 48.dp, start = 12.dp)
                    .align(Alignment.TopStart)
            )
        }

        CharacterVisual(
            step = currentStep,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .padding(top = 48.dp)
        )

        ContentFade(modifier = Modifier.fillMaxSize())

        AnimatedVisibility(
            visible = !transitioning,
            enter = fadeIn(),
            exit = fadeOut(),
            modifier = Modifier.fillMaxSize()
        ) {
            content()
        }
    }
}

@Composable
fun AmbientBackground(modifier: Modifier = Modifier) {
    val isDark = LocalIsDarkTheme.current
    val infiniteTransition = rememberInfiniteTransition(label = "ambient")

    Box(modifier = modifier) {
        Box(
            modifier = Modifier
                .size(340.dp)
                .align(Alignment.BottomStart)
                .offset(x = (-120).dp, y = 130.dp)
                .background(
                    Brush.radialGradient(
                        colors = listOf(
                            warmColor().copy(alpha = 0.11f),
                            Color.Transparent
                        )
                    )
                )
        )
        Box(
            modifier = Modifier
                .size(210.dp)
                .align(Alignment.BottomEnd)
                .offset(x = (-40).dp, y = (-100).dp)
                .background(
                    Brush.radialGradient(
                        colors = listOf(
                            if (isDark) Color.White.copy(alpha = 0.04f) else Color.White.copy(alpha = 0.22f),
                            Color.Transparent
                        )
                    )
                )
        )

        repeat(24) { index ->
            val seed = remember(index) { (index * 0.618033f) % 1f }
            val xPos = seed * 100f
            val yPos = ((seed * 1.618f) % 1f) * 100f
            val duration = 3500 + (seed * 5000).toInt()

            val floatX by infiniteTransition.animateFloat(
                initialValue = -15f,
                targetValue = 15f,
                animationSpec = infiniteRepeatable(
                    animation = tween(duration, easing = LinearEasing),
                    repeatMode = RepeatMode.Reverse
                ),
                label = "particleX$index"
            )
            val floatY by infiniteTransition.animateFloat(
                initialValue = -24f,
                targetValue = 24f,
                animationSpec = infiniteRepeatable(
                    animation = tween(duration, easing = LinearEasing),
                    repeatMode = RepeatMode.Reverse
                ),
                label = "particleY$index"
            )
            val alpha by infiniteTransition.animateFloat(
                initialValue = 0.11f,
                targetValue = 0.26f,
                animationSpec = infiniteRepeatable(
                    animation = tween(duration, easing = LinearEasing),
                    repeatMode = RepeatMode.Reverse
                ),
                label = "particleA$index"
            )

            Box(
                modifier = Modifier
                    .size(4.dp)
                    .offset(
                        x = LocalDensity.current.run { (xPos * 3.5f).dp },
                        y = LocalDensity.current.run { (yPos * 7f).dp }
                    )
                    .offset(
                        x = floatX.dp,
                        y = floatY.dp
                    )
                    .background(primaryColor().copy(alpha = alpha), CircleShape)
            )
        }
    }
}

@Composable
fun CharacterVisual(
    step: OnboardingFlowStep,
    modifier: Modifier = Modifier
) {
    val sizeDp = characterSizeForStep(step).dp
    val pColor = primaryColor()
    val wColor = warmColor()
    val pdColor = primaryDarkColor()
    val isDark = LocalIsDarkTheme.current

    val sizeAnim by animateFloatAsState(
        targetValue = characterSizeForStep(step).toFloat(),
        animationSpec = tween(740),
        label = "charSize"
    )

    val infiniteTransition = rememberInfiniteTransition(label = "character")
    val haloScale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = 1.06f,
        animationSpec = infiniteRepeatable(
            animation = tween(4800, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "haloScale"
    )
    val haloAlpha by infiniteTransition.animateFloat(
        initialValue = 0.76f,
        targetValue = 0.54f,
        animationSpec = infiniteRepeatable(
            animation = tween(4800, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "haloAlpha"
    )
    val orbit1Rotation by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(28000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "orbit1"
    )
    val orbit2Rotation by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = -360f,
        animationSpec = infiniteRepeatable(
            animation = tween(34000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "orbit2"
    )
    val orbit3Rotation by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(22000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "orbit3"
    )
    val symbolScale by infiniteTransition.animateFloat(
        initialValue = 0.95f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(5800, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "symbolScale"
    )
    val symbolRotation by infiniteTransition.animateFloat(
        initialValue = -2f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(5800, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "symbolRot"
    )

    val animSize = sizeAnim.dp

    Box(
        modifier = modifier
            .size(animSize)
            .drawBehind {
                val canvasSize = this.size.minDimension
                val center = Offset(this.size.width / 2f, this.size.height / 2f)

                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            pColor.copy(alpha = 0.17f),
                            pColor.copy(alpha = 0.035f),
                            Color.Transparent
                        ),
                        center = center,
                        radius = canvasSize * 0.46f
                    ),
                    radius = canvasSize * 0.46f * haloScale,
                    center = center,
                    alpha = haloAlpha
                )

                val orbit1Radius = canvasSize * 0.475f
                drawCircle(
                    color = pColor.copy(alpha = if (isDark) 0.11f else 0.14f),
                    radius = orbit1Radius,
                    center = center,
                    style = Stroke(width = 1.dp.toPx())
                )
                val dot1Angle = Math.toRadians(orbit1Rotation.toDouble())
                drawCircle(
                    color = wColor,
                    radius = 3.5.dp.toPx(),
                    center = Offset(
                        (center.x + orbit1Radius * cos(dot1Angle)).toFloat(),
                        (center.y + orbit1Radius * sin(dot1Angle)).toFloat()
                    )
                )

                val orbit2Radius = canvasSize * 0.425f
                drawCircle(
                    color = pColor.copy(alpha = if (isDark) 0.11f else 0.14f),
                    radius = orbit2Radius,
                    center = center,
                    style = Stroke(width = 1.dp.toPx(), pathEffect = androidx.compose.ui.graphics.PathEffect.dashPathEffect(floatArrayOf(6.dp.toPx(), 6.dp.toPx())))
                )
                val dot2Angle = Math.toRadians(orbit2Rotation.toDouble())
                drawCircle(
                    color = wColor,
                    radius = 3.5.dp.toPx(),
                    center = Offset(
                        (center.x + orbit2Radius * cos(dot2Angle)).toFloat(),
                        (center.y + orbit2Radius * sin(dot2Angle)).toFloat()
                    )
                )

                val orbit3Radius = canvasSize * 0.37f
                drawCircle(
                    color = pColor.copy(alpha = if (isDark) 0.06f else 0.084f),
                    radius = orbit3Radius,
                    center = center,
                    style = Stroke(width = 1.dp.toPx())
                )
                val dot3Angle = Math.toRadians(orbit3Rotation.toDouble())
                drawCircle(
                    color = wColor,
                    radius = 3.5.dp.toPx(),
                    center = Offset(
                        (center.x + orbit3Radius * cos(dot3Angle)).toFloat(),
                        (center.y + orbit3Radius * sin(dot3Angle)).toFloat()
                    )
                )

                val symbolSize = canvasSize * 0.54f * symbolScale
                val loopW = symbolSize * 0.76f
                val loopH = symbolSize * 0.38f
                val loopColor = if (isDark) pdColor.copy(alpha = 0.82f) else pdColor
                val loopStroke = 2.dp.toPx()

                for (rotationDeg in listOf(0f, 60f, 120f)) {
                    val totalRotation = rotationDeg + symbolRotation
                    withTransform({
                        rotate(totalRotation, center)
                    }) {
                        drawOval(
                            color = loopColor,
                            topLeft = Offset(
                                (center.x - loopW / 2f).toFloat(),
                                (center.y - loopH / 2f).toFloat()
                            ),
                            size = androidx.compose.ui.geometry.Size(loopW, loopH),
                            style = Stroke(width = loopStroke)
                        )
                    }
                }

                val centerSize = symbolSize * (0.18f / 0.54f)
                val centerRadius = centerSize * 0.34f
                drawRoundRect(
                    color = wColor.copy(alpha = 0.72f),
                    topLeft = Offset(
                        (center.x - centerSize / 2f).toFloat(),
                        (center.y - centerSize / 2f).toFloat()
                    ),
                    size = androidx.compose.ui.geometry.Size(centerSize, centerSize),
                    cornerRadius = androidx.compose.ui.geometry.CornerRadius(centerRadius, centerRadius),
                    style = Stroke(width = 1.dp.toPx())
                )

                val nodeRadius = 3.5.dp.toPx()
                val nodeDist = symbolSize * 0.42f
                drawCircle(color = wColor, radius = nodeRadius, center = Offset(center.x, (center.y - symbolSize * 0.42f).toFloat()))
                drawCircle(color = wColor, radius = nodeRadius, center = Offset((center.x + nodeDist * 0.88f).toFloat(), (center.y + nodeDist * 0.76f).toFloat()))
                drawCircle(color = wColor, radius = nodeRadius, center = Offset((center.x - nodeDist * 0.88f).toFloat(), (center.y + nodeDist * 0.76f).toFloat()))
            }
    )
}

@Composable
fun ContentFade(modifier: Modifier = Modifier) {
    val isDark = LocalIsDarkTheme.current
    Box(modifier = modifier) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(420.dp)
                .align(Alignment.BottomCenter)
                .background(
                    Brush.verticalGradient(
                        colors = if (isDark) {
                            listOf(
                                Color.Transparent,
                                DarkBg.copy(alpha = 0.12f),
                                DarkBg.copy(alpha = 0.16f),
                                DarkBg.copy(alpha = 0.24f)
                            )
                        } else {
                            listOf(
                                Color.Transparent,
                                Color.White.copy(alpha = 0.08f),
                                Color.White.copy(alpha = 0.14f),
                                Color.White.copy(alpha = 0.22f)
                            )
                        }
                    )
                )
        )
    }
}

@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    selected: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable () -> Unit
) {
    val isDark = LocalIsDarkTheme.current
    val bgColors = if (selected) {
        if (isDark) listOf(Color(0xAE2A3931), Color(0x992A3931))
        else listOf(Color(0x56FFFFFF), Color(0x29FFFFFF))
    } else {
        if (isDark) listOf(Color(0xD11B211D), Color(0x9E161B18))
        else listOf(Color(0x38FFFFFF), Color(0x14FFFFFF))
    }
    val borderColor = if (selected) {
        if (isDark) Color(0x1EFFFFFF) else OnboardingPrimary.copy(alpha = 0.20f)
    } else {
        glassBorderSoftColor()
    }

    val interactionSource = remember { MutableInteractionSource() }
    val baseModifier = modifier
        .clip(RoundedCornerShape(22.dp))
        .background(Brush.verticalGradient(bgColors))
        .border(1.dp, borderColor, RoundedCornerShape(22.dp))

    val finalModifier = if (onClick != null) {
        baseModifier.clickable(
            interactionSource = interactionSource,
            indication = null,
            role = Role.Button,
            onClick = onClick
        )
    } else baseModifier

    Box(modifier = finalModifier) {
        content()
    }
}

@Composable
fun PrimaryGlassButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true
) {
    val isDark = LocalIsDarkTheme.current
    val interactionSource = remember { MutableInteractionSource() }
    val pColor = primaryColor()
    val psColor = primarySoftColor()

    val buttonShape = RoundedCornerShape(18.dp)

    Box(
        modifier = modifier
            .height(56.dp)
            .then(
                if (enabled && !isDark) Modifier.shadow(
                    elevation = 8.dp,
                    shape = buttonShape,
                    ambientColor = pColor.copy(alpha = 0.14f),
                    spotColor = pColor.copy(alpha = 0.08f)
                ) else Modifier
            )
            .clip(buttonShape)
            .drawBehind {
                if (isDark) {
                    drawRect(
                        Brush.verticalGradient(
                            listOf(
                                pColor.copy(alpha = 0.80f),
                                pColor.copy(alpha = 0.92f)
                            )
                        )
                    )
                } else {
                    drawRect(pColor.copy(alpha = 0.60f))

                    drawCircle(
                        brush = Brush.radialGradient(
                            colors = listOf(
                                Color.White.copy(alpha = 0.12f),
                                Color.Transparent
                            ),
                            center = Offset(size.width * 0.16f, size.height * 0.24f),
                            radius = size.maxDimension * 0.38f
                        )
                    )

                    drawCircle(
                        brush = Brush.radialGradient(
                            colors = listOf(
                                psColor.copy(alpha = 0.17f),
                                Color.Transparent
                            ),
                            center = Offset(size.width * 0.78f, size.height * 0.70f),
                            radius = size.maxDimension * 0.48f
                        )
                    )

                    drawCircle(
                        brush = Brush.radialGradient(
                            colors = listOf(
                                pColor.copy(alpha = 0.12f),
                                Color.Transparent
                            ),
                            center = Offset(size.width * 0.52f, size.height * 0.46f),
                            radius = size.maxDimension * 0.64f
                        )
                    )

                    drawLine(
                        color = Color.White.copy(alpha = 0.12f),
                        start = Offset(1.dp.toPx(), 0.5f),
                        end = Offset(size.width - 1.dp.toPx(), 0.5f),
                        strokeWidth = 1.dp.toPx()
                    )
                }
            }
            .then(
                if (!isDark) Modifier.border(
                    width = 1.dp,
                    color = Color.White.copy(alpha = 0.12f),
                    shape = buttonShape
                ) else Modifier
            )
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = text,
            color = if (isDark) DarkText else Color(0xFFFDFEFD),
            fontSize = 15.sp,
            fontWeight = FontWeight(680),
            modifier = if (!enabled) Modifier.alpha(0.38f) else Modifier
        )
    }
}

@Composable
fun GlassBackButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isDark = LocalIsDarkTheme.current
    val interactionSource = remember { MutableInteractionSource() }
    val bgColors = if (isDark) {
        listOf(Color(0xD11B211D), Color(0x9E161B18))
    } else {
        listOf(Color(0x38FFFFFF), Color(0x14FFFFFF))
    }

    Box(
        modifier = modifier
            .size(48.dp)
            .clip(CircleShape)
            .background(Brush.verticalGradient(bgColors))
            .border(1.dp, glassBorderSoftColor(), CircleShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.ArrowBack,
            contentDescription = "返回上一步",
            tint = textColor(),
            modifier = Modifier.size(24.dp)
        )
    }
}

@Composable
fun StepLabel(
    text: String,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        color = primaryColor(),
        fontSize = 12.sp,
        fontWeight = FontWeight(680),
        letterSpacing = 0.3.sp,
        modifier = modifier.padding(bottom = 8.dp)
    )
}

@Composable
fun OnboardingTitle(
    text: String,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        color = textColor(),
        fontSize = 30.sp,
        fontWeight = FontWeight(630),
        lineHeight = (30 * 1.16).sp,
        letterSpacing = (-1.2).sp,
        modifier = modifier
    )
}

@Composable
fun OnboardingDescription(
    text: String,
    modifier: Modifier = Modifier
) {
    Text(
        text = text,
        color = mutedColor(),
        fontSize = 14.sp,
        fontWeight = FontWeight.Normal,
        lineHeight = (14 * 1.62).sp,
        modifier = modifier.padding(top = 10.dp)
    )
}

@Composable
fun RevealContent(
    delayMs: Int,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    val alphaState = androidx.compose.runtime.remember { androidx.compose.runtime.mutableFloatStateOf(0f) }
    val offsetYState = androidx.compose.runtime.remember { androidx.compose.runtime.mutableFloatStateOf(14f) }

    LaunchedEffectWithDelay(delayMs) {
        alphaState.floatValue = 1f
        offsetYState.floatValue = 0f
    }

    val welcomeEasing = remember { androidx.compose.animation.core.CubicBezierEasing(0.16f, 0.72f, 0.18f, 1f) }
    val alpha by animateFloatAsState(
        targetValue = alphaState.floatValue,
        animationSpec = tween(1050, delayMs, easing = welcomeEasing),
        label = "revealAlpha"
    )
    val offsetY by animateFloatAsState(
        targetValue = offsetYState.floatValue,
        animationSpec = tween(1050, delayMs, easing = welcomeEasing),
        label = "revealY"
    )

    Box(
        modifier = modifier
            .offset(y = offsetY.dp)
            .graphicsLayer(alpha = alpha)
    ) {
        content()
    }
}

@Composable
private fun LaunchedEffectWithDelay(delayMs: Int, action: () -> Unit) {
    androidx.compose.runtime.LaunchedEffect(delayMs) {
        kotlinx.coroutines.delay(delayMs.toLong())
        action()
    }
}

@Composable
fun ChoicePill(
    title: String,
    description: String,
    icon: ImageVector,
    selected: Boolean,
    onSelect: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isDark = LocalIsDarkTheme.current
    val pColor = primaryColor()

    GlassCard(
        modifier = modifier
            .fillMaxWidth()
            .height(82.dp),
        selected = selected,
        onClick = onSelect
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(RoundedCornerShape(16.dp))
                    .background(
                        if (isDark) Color(0x0DFFFFFF) else Color(0x38FFFFFF)
                    )
                    .border(
                        1.dp,
                        if (isDark) Color(0x14FFFFFF) else Color(0x3DFFFFFF),
                        RoundedCornerShape(16.dp)
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = pColor,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    color = textColor(),
                    fontSize = 15.sp,
                    fontWeight = FontWeight(640)
                )
                Text(
                    text = description,
                    color = mutedColor(),
                    fontSize = 12.sp,
                    lineHeight = (12 * 1.45).sp,
                    modifier = Modifier.padding(top = 4.dp)
                )
            }
            Box(
                modifier = Modifier
                    .size(22.dp)
                    .clip(CircleShape)
                    .then(
                        if (selected) Modifier.background(pColor)
                        else Modifier.border(1.5.dp, pColor.copy(alpha = 0.20f), CircleShape)
                    ),
                contentAlignment = Alignment.Center
            ) {
                if (selected) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = Color.White,
                        modifier = Modifier.size(15.dp)
                    )
                }
            }
        }
    }
}

@Composable
fun ProcessRowItem(
    title: String,
    description: String,
    state: ProcessState,
    modifier: Modifier = Modifier
) {
    val isDark = LocalIsDarkTheme.current
    val pColor = primaryColor()
    val sColor = successColor()

    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 60.dp)
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Box(
            modifier = Modifier
                .size(40.dp)
                .clip(CircleShape)
                .background(
                    when (state) {
                        ProcessState.Running -> if (isDark) Color(0x4DFFFFFF) else Color(0x4DFFFFFF)
                        ProcessState.Done -> sColor
                        ProcessState.Pending -> if (isDark) Color(0x0DFFFFFF) else Color(0x2EFFFFFF)
                    }
                )
                .border(
                    if (state == ProcessState.Pending) 1.dp else 0.dp,
                    glassBorderSoftColor(),
                    CircleShape
                ),
            contentAlignment = Alignment.Center
        ) {
            when (state) {
                ProcessState.Running -> {
                    val infiniteTransition = rememberInfiniteTransition(label = "spinner")
                    val rotation by infiniteTransition.animateFloat(
                        initialValue = 0f,
                        targetValue = 360f,
                        animationSpec = infiniteRepeatable(
                            animation = tween(800, easing = LinearEasing),
                            repeatMode = RepeatMode.Restart
                        ),
                        label = "spin"
                    )
                    Box(
                        modifier = Modifier
                            .size(19.dp)
                            .rotate(rotation)
                            .drawBehind {
                                drawArc(
                                    color = pColor.copy(alpha = 0.18f),
                                    startAngle = 0f,
                                    sweepAngle = 360f,
                                    useCenter = false,
                                    style = Stroke(width = 2.dp.toPx())
                                )
                                drawArc(
                                    color = pColor,
                                    startAngle = -90f,
                                    sweepAngle = 90f,
                                    useCenter = false,
                                    style = Stroke(width = 2.dp.toPx(), cap = StrokeCap.Round)
                                )
                            }
                    )
                }
                ProcessState.Done -> {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = Color.White,
                        modifier = Modifier.size(20.dp)
                    )
                }
                ProcessState.Pending -> {
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .background(mutedSecondaryColor(), CircleShape)
                    )
                }
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = title,
                color = textColor(),
                fontSize = 14.sp,
                fontWeight = FontWeight(620)
            )
            Text(
                text = description,
                color = mutedColor(),
                fontSize = 12.sp,
                modifier = Modifier.padding(top = 2.dp)
            )
        }
        Text(
            text = when (state) {
                ProcessState.Running -> "检查中"
                ProcessState.Done -> "已完成"
                ProcessState.Pending -> "等待"
            },
            color = when (state) {
                ProcessState.Running -> pColor
                ProcessState.Done -> sColor
                ProcessState.Pending -> mutedSecondaryColor()
            },
            fontSize = 12.sp
        )
    }
}

enum class ProcessState { Pending, Running, Done }

@Composable
fun SoftField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    isPassword: Boolean = false,
    placeholder: String = "",
    singleLine: Boolean = true
) {
    val isDark = LocalIsDarkTheme.current

    GlassCard(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 60.dp)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .padding(top = 8.dp, bottom = 8.dp)
        ) {
            Text(
                text = label,
                color = mutedColor(),
                fontSize = 11.sp
            )
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 4.dp),
                textStyle = androidx.compose.ui.text.TextStyle(
                    color = textColor(),
                    fontSize = 16.sp
                ),
                singleLine = singleLine,
                visualTransformation = if (isPassword) androidx.compose.ui.text.input.PasswordVisualTransformation()
                else androidx.compose.ui.text.input.VisualTransformation.None,
                cursorBrush = androidx.compose.ui.graphics.SolidColor(primaryColor()),
                decorationBox = { innerTextField ->
                    if (value.isEmpty() && placeholder.isNotBlank()) {
                        androidx.compose.foundation.layout.Box {
                            Text(
                                text = placeholder,
                                color = mutedSecondaryColor().copy(alpha = 0.82f),
                                fontSize = 16.sp
                            )
                            innerTextField()
                        }
                    } else {
                        innerTextField()
                    }
                }
            )
        }
    }
}

@Composable
fun BottomActionContainer(
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    val isDark = LocalIsDarkTheme.current
    Column(
        modifier = modifier
            .fillMaxWidth()
            .windowInsetsPadding(WindowInsets.navigationBars)
            .padding(horizontal = 16.dp)
            .padding(bottom = 10.dp)
    ) {
        content()
    }
}

@Composable
fun StepContentScroll(
    topPadding: Int,
    bottomPadding: Int = 124,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp)
            .padding(top = topPadding.dp)
            .padding(bottom = bottomPadding.dp)
    ) {
        content()
    }
}

@Composable
fun SoftRow(
    title: String,
    description: String,
    icon: ImageVector,
    valueText: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isDark = LocalIsDarkTheme.current
    val pColor = primaryColor()

    GlassCard(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 70.dp),
        onClick = onClick
    ) {
        Row(
            modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(RoundedCornerShape(16.dp))
                    .background(if (isDark) Color(0x992A3931) else Color(0x38FFFFFF))
                    .border(
                        1.dp,
                        if (isDark) Color(0x14FFFFFF) else Color(0x3DFFFFFF),
                        RoundedCornerShape(16.dp)
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = pColor,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    color = textColor(),
                    fontSize = 15.sp,
                    fontWeight = FontWeight(620)
                )
                Text(
                    text = description,
                    color = mutedColor(),
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 3.dp)
                )
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Text(
                    text = valueText,
                    color = mutedColor(),
                    fontSize = 12.sp
                )
                Icon(
                    imageVector = AmitiaIcons.ChevronRight,
                    contentDescription = null,
                    tint = mutedColor(),
                    modifier = Modifier.size(18.dp)
                )
            }
        }
    }
}

@Composable
fun InlineLink(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val pColor = primaryColor()
    val interactionSource = remember { MutableInteractionSource() }
    Text(
        text = text,
        color = pColor,
        fontSize = 14.sp,
        fontWeight = FontWeight(620),
        modifier = modifier
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            )
            .padding(horizontal = 4.dp)
    )
}

@Composable
fun Toast(
    text: String,
    visible: Boolean,
    modifier: Modifier = Modifier
) {
    AnimatedVisibility(
        visible = visible,
        enter = fadeIn(),
        exit = fadeOut(),
        modifier = modifier
    ) {
        Surface(
            shape = RoundedCornerShape(50),
            color = Color(0xE62F3B35),
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)
        ) {
            Text(
                text = text,
                color = Color.White,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp)
            )
        }
    }
}

@Composable
fun CompletionCheckIcon(
    modifier: Modifier = Modifier
) {
    val sColor = successColor()
    Box(
        modifier = modifier
            .size(58.dp)
            .clip(CircleShape)
            .drawBehind {
                drawRect(sColor.copy(alpha = 0.72f))
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            Color.White.copy(alpha = 0.16f),
                            Color.Transparent
                        ),
                        center = Offset(size.width * 0.3f, size.height * 0.24f),
                        radius = size.minDimension * 0.5f
                    )
                )
            }
            .border(
                width = 1.dp,
                color = Color.White.copy(alpha = 0.14f),
                shape = CircleShape
            ),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.Check,
            contentDescription = null,
            tint = Color.White,
            modifier = Modifier.size(27.dp)
        )
    }
}

@Composable
fun ModelSelectorOverlay(
    title: String,
    options: List<String>,
    selectedValue: String,
    onSelect: (String) -> Unit,
    onDismiss: () -> Unit,
    visible: Boolean,
    modifier: Modifier = Modifier
) {
    val isDark = LocalIsDarkTheme.current
    val pColor = primaryColor()

    AnimatedVisibility(
        visible = visible,
        enter = fadeIn(),
        exit = fadeOut(),
        modifier = modifier.fillMaxSize()
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .drawBehind {
                    drawRect(Color(0x0F131714))
                }
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    onClick = onDismiss
                ),
            contentAlignment = Alignment.BottomCenter
        ) {
            GlassCard(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 18.dp)
                    .padding(bottom = 18.dp)
            ) {
                Column(
                    modifier = Modifier.padding(top = 22.dp, start = 22.dp, end = 22.dp, bottom = 12.dp)
                ) {
                    Text(
                        text = title,
                        color = textColor(),
                        fontSize = 20.sp,
                        fontWeight = FontWeight(630)
                    )
                    Text(
                        text = "选择当前能力默认使用的模型。",
                        color = mutedColor(),
                        fontSize = 12.sp,
                        modifier = Modifier.padding(top = 6.dp)
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    options.forEach { option ->
                        val isSelected = option == selectedValue
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .heightIn(min = 58.dp)
                                .clip(RoundedCornerShape(16.dp))
                                .then(
                                    if (isSelected) Modifier.background(
                                        if (isDark) Color(0xA8422D24) else Color(0x38FFFFFF)
                                    ) else Modifier
                                )
                                .clickable(
                                    interactionSource = remember { MutableInteractionSource() },
                                    indication = null,
                                    role = Role.Button,
                                    onClick = { onSelect(option) }
                                )
                                .padding(horizontal = 12.dp),
                            contentAlignment = Alignment.CenterStart
                        ) {
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = option,
                                    color = textColor(),
                                    fontSize = 15.sp
                                )
                                if (isSelected) {
                                    Icon(
                                        imageVector = AmitiaIcons.Check,
                                        contentDescription = null,
                                        tint = pColor,
                                        modifier = Modifier.size(20.dp)
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
