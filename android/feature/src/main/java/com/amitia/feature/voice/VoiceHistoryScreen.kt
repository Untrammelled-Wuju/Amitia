package com.amitia.feature.voice

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun VoiceHistoryScreen(
    onBack: () -> Unit,
    onOpenDetail: (String) -> Unit,
    viewModel: VoiceViewModel = hiltViewModel()
) {
    val state by viewModel.historyState.collectAsStateWithLifecycle()
    VoiceHistoryContent(state = state, onBack = onBack, onOpenDetail = onOpenDetail)
}

@Composable
fun VoiceHistoryContent(
    state: ScreenState<VoiceHistoryUiState>,
    onBack: () -> Unit,
    onOpenDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "通话历史", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载通话记录...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = state.error.title,
                    description = state.error.message,
                    onRetry = onBack,
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Phone,
                    title = "暂无通话记录",
                    description = "和角色的语音通话会记录在这里",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> HistoryList(items = state.data.items, onOpenDetail = onOpenDetail)
            is ScreenState.Partial -> HistoryList(items = state.data.items, onOpenDetail = onOpenDetail)
        }
    }
}

@Composable
private fun HistoryList(items: List<VoiceCallHistoryItem>, onOpenDetail: (String) -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(items, key = { it.id }) { item ->
            HistoryRow(item = item, onClick = { onOpenDetail(item.id) })
        }
    }
}

@Composable
private fun HistoryRow(item: VoiceCallHistoryItem, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    val resultColor = when (item.result) {
        VoiceCallResult.Completed -> MaterialTheme.colorScheme.tertiary
        VoiceCallResult.Missed -> MaterialTheme.colorScheme.error
        VoiceCallResult.Declined -> MaterialTheme.colorScheme.onSurfaceVariant
        VoiceCallResult.Failed -> MaterialTheme.colorScheme.error
    }
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            ),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (item.result == VoiceCallResult.Missed) AmitiaIcons.PhoneOff
                    else AmitiaIcons.Phone,
                    contentDescription = null,
                    tint = if (item.result == VoiceCallResult.Missed) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(AmitiaIconSize.Nav)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = item.characterName,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = item.result.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = resultColor
                    )
                }
                Text(
                    text = item.time,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Schedule,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                    Text(
                        text = item.duration,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    if (item.hasCaption) {
                        Spacer(modifier = Modifier.size(AmitiaSpacing.Xs))
                        Icon(
                            imageVector = AmitiaIcons.ChatBubble,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                        Text(
                            text = "字幕",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                        )
                    }
                }
            }
            Icon(
                imageVector = AmitiaIcons.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        }
    }
}

@Preview(name = "Voice History - Light", showBackground = true)
@Composable
private fun VoiceHistoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceHistoryContent(
            state = ScreenState.Content(
                VoiceHistoryUiState(
                    items = listOf(
                        VoiceCallHistoryItem("1", "艾米", "温柔知性助手", "今天 14:30", "05:23", VoiceCallResult.Completed, true),
                        VoiceCallHistoryItem("2", "艾米", "温柔知性助手", "昨天 09:42", "00:00", VoiceCallResult.Missed, false)
                    )
                )
            ),
            onBack = {},
            onOpenDetail = {}
        )
    }
}

@Preview(name = "Voice History - Dark", showBackground = true)
@Composable
private fun VoiceHistoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceHistoryContent(
            state = ScreenState.Empty(),
            onBack = {},
            onOpenDetail = {}
        )
    }
}
