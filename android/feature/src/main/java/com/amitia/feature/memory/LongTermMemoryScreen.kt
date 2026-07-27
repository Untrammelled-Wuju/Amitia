package com.amitia.feature.memory

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
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.model.MemoryDto

@Composable
fun LongTermMemoryScreen(
    onBack: () -> Unit,
    onMemoryDetail: (String) -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.longTermState.collectAsStateWithLifecycle()
    LongTermMemoryContent(state = state, onBack = onBack, onMemoryDetail = onMemoryDetail)
}

@Composable
fun LongTermMemoryContent(
    state: ScreenState<List<LongTermMemoryGroup>>,
    onBack: () -> Unit,
    onMemoryDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "长期记忆", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载长期记忆...")
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
                    icon = AmitiaIcons.Psychology,
                    title = "暂无长期记忆",
                    description = "长期记忆会从对话中自动提取并保存",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> LongTermGroupList(groups = state.data, onMemoryDetail = onMemoryDetail)
            is ScreenState.Partial -> LongTermGroupList(groups = state.data, onMemoryDetail = onMemoryDetail)
        }
    }
}

@Composable
private fun LongTermGroupList(groups: List<LongTermMemoryGroup>, onMemoryDetail: (String) -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        groups.forEach { group ->
            item(key = "header_${group.title}") {
                AmitiaSectionHeader(
                    title = group.title,
                    trailing = {
                        Text(
                            text = "${group.items.size}条",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                )
            }
            if (group.items.isEmpty()) {
                item(key = "empty_${group.title}") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = RoundedCornerShape(16.dp),
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Text(
                            text = "暂无记录",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
            } else {
                items(group.items, key = { it.id }) { memory ->
                    LongTermMemoryCard(memory = memory, onClick = { onMemoryDetail(memory.id) })
                }
            }
        }
    }
}

@Composable
private fun LongTermMemoryCard(memory: MemoryDto, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
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
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Text(
                text = memory.content,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                memory.scope?.let {
                    Text(
                        text = it,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                memory.createdAt?.let {
                    Text(
                        text = it,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Preview(name = "Long Term - Light", showBackground = true)
@Composable
private fun LongTermMemoryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        LongTermMemoryContent(
            state = ScreenState.Content(
                listOf(
                    LongTermMemoryGroup("用户事实", listOf(
                        MemoryDto("1", "用户名是小明", "long_term", createdAt = "初始"),
                        MemoryDto("2", "用户的工作是软件开发", "long_term", createdAt = "7月24日")
                    )),
                    LongTermMemoryGroup("用户偏好", listOf(
                        MemoryDto("3", "偏好简洁回复风格", "long_term", createdAt = "今天")
                    ))
                )
            ),
            onBack = {},
            onMemoryDetail = {}
        )
    }
}

@Preview(name = "Long Term - Dark", showBackground = true)
@Composable
private fun LongTermMemoryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        LongTermMemoryContent(
            state = ScreenState.Empty(),
            onBack = {},
            onMemoryDetail = {}
        )
    }
}
