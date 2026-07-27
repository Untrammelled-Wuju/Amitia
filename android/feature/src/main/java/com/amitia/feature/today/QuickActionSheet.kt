package com.amitia.feature.today

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaBottomSheetShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaTouchTarget
import com.amitia.core.designsystem.component.AmitiaBottomSheet

@Composable
fun QuickActionSheet(
    onDismiss: () -> Unit,
    onAction: (QuickAction) -> Unit
) {
    val actions = remember { defaultQuickActions() }
    AmitiaBottomSheet(
        onDismiss = onDismiss,
        title = "快捷操作"
    ) {
        LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = AmitiaSpacing.Base)
        ) {
            items(actions, key = { it.id }) { action ->
                QuickActionItem(action = action) {
                    onAction(action)
                    onDismiss()
                }
            }
        }
    }
}

@Composable
private fun QuickActionItem(action: QuickAction, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Column(
        modifier = Modifier
            .clip(AmitiaBottomSheetShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            )
            .padding(AmitiaSpacing.Md),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Box(
            modifier = Modifier
                .size(AmitiaTouchTarget.Minimum)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = quickActionIcon(action.iconType),
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Nav)
            )
        }
        Text(
            text = action.label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            textAlign = TextAlign.Center
        )
    }
}

private fun defaultQuickActions() = listOf(
    QuickAction("chat", "开始对话", QuickActionIcon.Chat),
    QuickAction("voice", "开始语音", QuickActionIcon.Voice),
    QuickAction("memory", "新建记忆", QuickActionIcon.Memory),
    QuickAction("schedule", "新建日程", QuickActionIcon.Schedule),
    QuickAction("switch", "切换角色", QuickActionIcon.SwitchCharacter),
    QuickAction("import", "导入内容", QuickActionIcon.Import)
)

private fun quickActionIcon(type: QuickActionIcon): ImageVector = when (type) {
    QuickActionIcon.Chat -> AmitiaIcons.Chat
    QuickActionIcon.Voice -> AmitiaIcons.Mic
    QuickActionIcon.Memory -> AmitiaIcons.Memory
    QuickActionIcon.Schedule -> AmitiaIcons.Event
    QuickActionIcon.SwitchCharacter -> AmitiaIcons.People
    QuickActionIcon.Import -> AmitiaIcons.Upload
}

@Preview(name = "Quick Action Sheet - Light", showBackground = true)
@Composable
private fun QuickActionSheetLightPreview() {
    AmitiaTheme(darkTheme = false) {
        QuickActionSheet(onDismiss = {}, onAction = {})
    }
}

@Preview(name = "Quick Action Sheet - Dark", showBackground = true)
@Composable
private fun QuickActionSheetDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        QuickActionSheet(onDismiss = {}, onAction = {})
    }
}
