package com.amitia.core.designsystem

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.shape.CornerSize
import androidx.compose.material3.Shapes
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp

object AmitiaRadius {
    val Xs = 10.dp
    val S = 14.dp
    val M = 18.dp
    val L = 22.dp
    val XL = 26.dp
    val XXL = 30.dp
    val Hero = 32.dp
    val Pill = 50.dp
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
    topStart = CornerSize(AmitiaRadius.XXL),
    topEnd = CornerSize(AmitiaRadius.XXL),
    bottomEnd = CornerSize(0.dp),
    bottomStart = CornerSize(0.dp)
)
val AmitiaBottomNavShape: Shape = RoundedCornerShape(AmitiaRadius.XXL)
val AmitiaChatDockShape: Shape = RoundedCornerShape(AmitiaRadius.XL)
val AmitiaPillShape: Shape = RoundedCornerShape(50)
val AmitiaSmallButtonShape: Shape = RoundedCornerShape(AmitiaRadius.S)
