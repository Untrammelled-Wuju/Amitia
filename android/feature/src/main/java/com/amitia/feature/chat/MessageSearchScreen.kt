package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.EmptyReason
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.model.MessageDto

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MessageSearchScreen(
    onBack: () -> Unit,
    onOpenMessage: (String) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    val searchState by viewModel.searchState.collectAsStateWithLifecycle()
    var query by remember { mutableStateOf("") }
    var selectedFilter by remember { mutableStateOf(0) }
    val filters = listOf("全部", "用户消息", "角色回复", "工具调用")

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "搜索消息", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaSearchField(
                value = query,
                onValueChange = { query = it },
                placeholder = "搜索消息内容",
                onClear = {
                    query = ""
                    viewModel.clearSearch()
                }
            )
        }
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaChipSelector(
                items = filters.mapIndexed { index, label -> AmitiaChipItem(label, index == selectedFilter) },
                onToggle = { selectedFilter = it },
                multiSelect = false
            )
        }
        if (searchState.searching) {
            InlineLoading(
                message = "搜索中...",
                modifier = Modifier.padding(AmitiaSpacing.Base)
            )
        } else if (searchState.error != null) {
            AmitiaEmptyState(
                icon = AmitiaIcons.ErrorOutline,
                title = "搜索失败",
                description = searchState.error,
                reason = EmptyReason.NoResults
            )
        } else if (searchState.searched && searchState.results.isEmpty()) {
            AmitiaEmptyState(
                icon = AmitiaIcons.SearchOutlined,
                title = "未找到结果",
                description = "试试其他关键词吧",
                reason = EmptyReason.NoResults
            )
        } else if (!searchState.searched) {
            AmitiaEmptyState(
                icon = AmitiaIcons.Search,
                title = "搜索消息",
                description = "输入关键词搜索对话内容、记忆和角色信息",
                reason = EmptyReason.NoData
            )
        } else {
            SearchResultsList(
                results = searchState.results,
                onOpenMessage = onOpenMessage
            )
        }
    }
}

@Composable
private fun SearchResultsList(
    results: List<MessageSearchResult>,
    onOpenMessage: (String) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item(key = "result_count") {
            Text(
                text = "找到 ${results.size} 条结果",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        items(results, key = { it.message.id }) { result ->
            SearchResultItem(result = result, onClick = { onOpenMessage(result.message.id) })
        }
    }
}

@Composable
private fun SearchResultItem(
    result: MessageSearchResult,
    onClick: () -> Unit
) {
    val isUser = result.message.role == "user"
    Surface(
        modifier = Modifier.fillMaxWidth().clip(AmitiaCardShape).clickable(onClick = onClick),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier.size(AmitiaIconSize.Medium).clip(CircleShape)
                        .background(if (isUser) MaterialTheme.colorScheme.surfaceVariant else MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (isUser) AmitiaIcons.Person else AmitiaIcons.SmartToy,
                        contentDescription = null,
                        tint = if (isUser) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = result.characterName,
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = result.conversationTitle,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
            val annotatedText = buildAnnotatedString {
                withStyle(SpanStyle(fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.primary)) {
                    append(result.matchedSnippet)
                }
            }
            Text(
                text = annotatedText,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.size(AmitiaSpacing.Xs))
            Text(
                text = result.message.createdAt ?: "",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
        }
    }
}

@Preview(name = "Message Search - Light", showBackground = true)
@Composable
private fun MessageSearchLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            SearchResultItem(
                result = MessageSearchResult(
                    message = MessageDto(id = "m1", role = "assistant", content = "关于天气的内容"),
                    conversationTitle = "与艾米的对话",
                    characterName = "艾米",
                    matchedSnippet = "...关于天气的讨论..."
                ),
                onClick = {}
            )
        }
    }
}

@Preview(name = "Message Search - Dark", showBackground = true)
@Composable
private fun MessageSearchDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            SearchResultItem(
                result = MessageSearchResult(
                    message = MessageDto(id = "m1", role = "user", content = "测试搜索"),
                    conversationTitle = "与星野的对话",
                    characterName = "星野",
                    matchedSnippet = "测试搜索结果"
                ),
                onClick = {}
            )
        }
    }
}
