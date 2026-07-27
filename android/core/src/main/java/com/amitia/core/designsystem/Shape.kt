package com.amitia.core.designsystem

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.shape.ZeroCornerSize
import androidx.compose.material3.Shapes
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp

object AmitiaRadius {
    val Xs = 8.dp
    val S = 12.dp
    val M = 16.dp
    val L = 20.dp
    val XL = 24.dp
    val XXL = 28.dp
    val Hero = 32.dp
}

val AmitiaShapes = Shapes(
    extraSmall = RoundedCornerShape(AmitiaRadius.Xs),
    small = RoundedCornerShape(AmitiaRadius.S),
    medium = RoundedCornerShape(AmitiaRadius.M),
    large = RoundedCornerShape(AmitiaRadius.L),
    extraLarge = RoundedCornerShape(AmitiaRadius.XXL)
)

val AmitiaCardShape: Shape = RoundedCornerShape(AmitiaRadius.L)
val AmitiaLargeCardShape: Shape = RoundedCornerShape(AmitiaRadius.XL)
val AmitiaHeroCardShape: Shape = RoundedCornerShape(AmitiaRadius.Hero)
val AmitiaListItemShape: Shape = RoundedCornerShape(AmitiaRadius.M)
val AmitiaInputShape: Shape = RoundedCornerShape(AmitiaRadius.L)
val AmitiaBottomSheetShape: Shape = RoundedCornerShape(
    topStart = AmitiaRadius.XXL,
    topEnd = AmitiaRadius.XXL,
    bottomEnd = ZeroCornerSize,
    bottomStart = ZeroCornerSize
)
val AmitiaBottomNavShape: Shape = RoundedCornerShape(AmitiaRadius.XXL)
val AmitiaChatDockShape: Shape = RoundedCornerShape(AmitiaRadius.XL)
val AmitiaPillShape: Shape = RoundedCornerShape(50)
val AmitiaSmallButtonShape: Shape = RoundedCornerShape(AmitiaRadius.S)
