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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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
import com.amitia.core.designsystem.component.AmitiaSearchTopBar
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.model.MemoryDto

@Composable
fun MemorySearchScreen(
    onBack: () -> Unit,
    onMemoryDetail: (String) -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.searchState.collectAsStateWithLifecycle()
    var keyword by remember { mutableStateOf("") }
    var semanticEnabled by remember { mutableStateOf(true) }
    val semanticAvailable = true
    MemorySearchContent(
        keyword = keyword,
        onKeywordChange = { keyword = it },
        semanticEnabled = semanticEnabled,
        semanticAvailable = semanticAvailable,
        onSemanticToggle = { semanticEnabled = it },
        state = state,
        onBack = onBack,
        onSearch = { viewModel.search(MemorySearchFilters(keyword = keyword, semanticEnabled = semanticEnabled, semanticAvailable = semanticAvailable)) },
        onMemoryDetail = onMemoryDetail
    )
}

@Composable
fun MemorySearchContent(
    keyword: String,
    onKeywordChange: (String) -> Unit,
    semanticEnabled: Boolean,
    semanticAvailable: Boolean,
    onSemanticToggle: (Boolean) -> Unit,
    state: ScreenState<List<MemoryDto>>,
    onBack: () -> Unit,
    onSearch: () -> Unit,
    onMemoryDetail: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaSearchTopBar(
            query = keyword,
            onQueryChange = onKeywordChange,
            onBack = onBack,
            onClear = { onKeywordChange("") }
        )
        if (keyword.isNotEmpty()) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = if (semanticAvailable) AmitiaIcons.Psychology else AmitiaIcons.WarningAmber,
                    contentDescription = null,
                    tint = if (semanticAvailable) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.tertiary,
                    modifier = Modifier.size(AmitiaIconSize.Small)
                )
                Text(
                    text = if (semanticAvailable) "语义搜索${if (semanticEnabled) "已开启" else "已关闭"}"
                    else "语义搜索不可用，使用关键词搜索",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Spacer(modifier = Modifier.weight(1f))
                if (semanticAvailable) {
                    val interactionSource = remember { MutableInteractionSource() }
                    Surface(
                        modifier = Modifier
                            .clip(RoundedCornerShape(20.dp))
                            .clickable(
                                interactionSource = interactionSource,
                                indication = null,
                                role = Role.Switch,
                                onClick = { onSemanticToggle(!semanticEnabled) }
                            ),
                        shape = RoundedCornerShape(20.dp),
                        color = if (semanticEnabled) MaterialTheme.colorScheme.primaryContainer
                        else MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Text(
                            text = if (semanticEnabled) "语义" else "关键词",
                            style = MaterialTheme.typography.labelSmall,
                            color = if (semanticEnabled) MaterialTheme.colorScheme.onPrimaryContainer
                            else MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                        )
                    }
                }
            }
        }
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "搜索中...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = "搜索失败",
                    description = state.error.message,
                    onRetry = {},
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Search,
                    title = if (keyword.isEmpty()) "输入关键词搜索" else "未找到相关记忆",
                    description = if (keyword.isEmpty()) "支持关键词和语义搜索" else "尝试更换关键词或调整筛选条件",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> SearchResultList(results = state.data, onMemoryDetail = onMemoryDetail)
            is ScreenState.Partial -> SearchResultList(results = state.data, onMemoryDetail = onMemoryDetail)
        }
    }
}

@Composable
private fun SearchResultList(results: List<MemoryDto>, onMemoryDetail: (String) -> Unit) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(results, key = { it.id }) { memory ->
            SearchResultCard(memory = memory, onClick = { onMemoryDetail(memory.id) })
        }
    }
}

@Composable
private fun SearchResultCard(memory: MemoryDto, onClick: () -> Unit) {
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
                memory.type?.let { SearchTag(text = it) }
                memory.scope?.let { SearchTag(text = it) }
                memory.importance?.let {
                    SearchTag(text = "P${it.toInt()}")
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

@Composable
private fun SearchTag(text: String) {
    Surface(
        shape = RoundedCornerShape(6.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
        )
    }
}

@Preview(name = "Search - Light", showBackground = true)
@Composable
private fun MemorySearchLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemorySearchContent(
            keyword = "会议",
            onKeywordChange = {},
            semanticEnabled = true,
            semanticAvailable = true,
            onSemanticToggle = {},
            state = ScreenState.Content(
                listOf(
                    MemoryDto("1", "用户提到今天有重要会议", "episodic", "global", "1", 4.0, createdAt = "今天 14:30"),
                    MemoryDto("2", "讨论了新方案但未最终定", "episodic", "global", "1", 3.0, createdAt = "今天 14:35")
                )
            ),
            onBack = {},
            onSearch = {},
            onMemoryDetail = {}
        )
    }
}

@Preview(name = "Search - Dark", showBackground = true)
@Composable
private fun MemorySearchDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemorySearchContent(
            keyword = "",
            onKeywordChange = {},
            semanticEnabled = true,
            semanticAvailable = false,
            onSemanticToggle = {},
            state = ScreenState.Empty(),
            onBack = {},
            onSearch = {},
            onMemoryDetail = {}
        )
    }
}
