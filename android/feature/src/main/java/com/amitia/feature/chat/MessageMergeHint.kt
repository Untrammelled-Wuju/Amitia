package com.amitia.feature.chat

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.component.TertiaryButton
import com.amitia.core.designsystem.GlassLevel

@Composable
fun MergeHintBanner(
    remainingSeconds: Int,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier
) {
    AnimatedVisibility(
        visible = remainingSeconds > 0,
        enter = slideInVertically() + fadeIn(),
        exit = slideOutVertically() + fadeOut(),
        modifier = modifier
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            contentAlignment = Alignment.BottomCenter
        ) {
            AmitiaGlassSurface(level = GlassLevel.Chip, modifier = Modifier.fillMaxWidth()) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(AmitiaIconSize.Small),
                        strokeWidth = 1.5.dp,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = "正在合并连续消息",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = "${remainingSeconds}秒后发送，继续输入可合并",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    TertiaryButton(
                        text = "立即发送",
                        onClick = onCancel
                    )
                }
            }
        }
    }
}

@Composable
fun MergeHintInline(
    remainingSeconds: Int,
    onCancel: () -> Unit,
    modifier: Modifier = Modifier
) {
    AnimatedVisibility(
        visible = remainingSeconds > 0,
        enter = fadeIn(),
        exit = fadeOut(),
        modifier = modifier
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = AmitiaSpacing.Base)
                .clip(androidx.compose.foundation.shape.RoundedCornerShape(12.dp))
                .background(MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f))
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier.size(8.dp).clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = remainingSeconds.toString(),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onPrimary,
                    fontWeight = FontWeight.Bold
                )
            }
            Text(
                text = "5秒内继续输入将合并为一条消息发送",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f)
            )
            Icon(
                imageVector = AmitiaIcons.Close,
                contentDescription = "取消合并",
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(AmitiaIconSize.Small)
            )
        }
    }
}

@Preview(name = "Merge Hint - Light", showBackground = true)
@Composable
private fun MergeHintLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            MergeHintBanner(remainingSeconds = 5, onCancel = {})
        }
    }
}

@Preview(name = "Merge Hint - Dark", showBackground = true)
@Composable
private fun MergeHintDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Box(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            MergeHintInline(remainingSeconds = 3, onCancel = {})
        }
    }
}
