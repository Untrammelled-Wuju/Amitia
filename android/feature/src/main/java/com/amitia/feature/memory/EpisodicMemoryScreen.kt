package com.amitia.feature.memory

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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
fun EpisodicMemoryScreen(
    onBack: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.episodicState.collectAsStateWithLifecycle()
    EpisodicMemoryContent(state = state, onBack = onBack)
}

@Composable
fun EpisodicMemoryContent(
    state: ScreenState<List<EpisodicMemoryItem>>,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "情景记忆", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载情景记忆...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = "加载失败",
                    description = state.error.message,
                    onRetry = {},
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Event,
                    title = "暂无情景记忆",
                    description = "情景记忆会从事件性对话中自动提取",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> EpisodicList(items = state.data)
            is ScreenState.Partial -> EpisodicList(items = state.data)
        }
    }
}

@Composable
private fun EpisodicList(items: List<EpisodicMemoryItem>) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(items, key = { it.id }) { item ->
            EpisodicCard(item = item)
        }
    }
}

@Composable
private fun EpisodicCard(item: EpisodicMemoryItem) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(36.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Event,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = item.timestamp,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            EpisodicSection(label = "起因", content = item.cause)
            EpisodicSection(label = "过程", content = item.process)
            EpisodicSection(label = "结果", content = item.result)
            if (item.participants.isNotEmpty()) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.People,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f),
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                    Text(
                        text = "参与：${item.participants.joinToString("、")}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Composable
private fun EpisodicSection(label: String, content: String) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.primary
        )
        Text(
            text = content,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface,
            maxLines = 3,
            overflow = TextOverflow.Ellipsis
        )
    }
}

@Preview(name = "Episodic - Light", showBackground = true)
@Composable
private fun EpisodicMemoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EpisodicMemoryContent(
            state = ScreenState.Content(
                listOf(
                    EpisodicMemoryItem("1", "下午会议讨论", "用户提到有会议", "讨论了新方案但未最终定", "方案待确定", listOf("我", "艾米"), "今天 14:30"),
                    EpisodicMemoryItem("2", "周末看电影", "用户周末想放松", "看了科幻电影", "心情愉快", listOf("我", "艾米"), "7月25日")
                )
            ),
            onBack = {}
        )
    }
}

@Preview(name = "Episodic - Dark", showBackground = true)
@Composable
private fun EpisodicMemoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EpisodicMemoryContent(
            state = ScreenState.Empty(),
            onBack = {}
        )
    }
}
