package com.amitia.feature.today

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSearchTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun GlobalSearchScreen(
    onBack: () -> Unit,
    onOpenResult: (SearchResultItem) -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val state by viewModel.searchState.collectAsStateWithLifecycle()
    var recentQueries = remember { mutableStateOf(listOf("艾米", "会议", "记忆")) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaSearchTopBar(
            query = state.query,
            onQueryChange = viewModel::updateQuery,
            onBack = onBack,
            onClear = viewModel::clearSearch,
            placeholder = "搜索对话、记忆、角色、文件"
        )
        if (state.query.isBlank() && !state.searching) {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(AmitiaSpacing.Base),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (recentQueries.value.isNotEmpty()) {
                    item(key = "recent_header") {
                        Text(
                            text = "最近搜索",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(vertical = AmitiaSpacing.Xs)
                        )
                    }
                    items(recentQueries.value, key = { "recent_$it" }) { query ->
                        val interactionSource = remember { MutableInteractionSource() }
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable(
                                    interactionSource = interactionSource,
                                    indication = null,
                                    role = Role.Button,
                                    onClick = { viewModel.updateQuery(query) }
                                )
                                .padding(vertical = AmitiaSpacing.Sm),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.History,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.size(AmitiaIconSize.Medium)
                            )
                            Text(
                                text = query,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurface,
                                modifier = Modifier.weight(1f)
                            )
                            Icon(
                                imageVector = AmitiaIcons.ArrowForward,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                                modifier = Modifier.size(AmitiaIconSize.Medium)
                            )
                        }
                    }
                }
                item(key = "index_status") {
                    IndexReadyChip()
                }
            }
        } else if (state.searching) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "搜索中...")
            }
        } else if (state.searched && state.results.isEmpty()) {
            AmitiaEmptyState(
                icon = AmitiaIcons.SearchOutlined,
                title = "未找到「${state.query}」相关结果",
                description = "试试更换关键词，或检查搜索索引是否就绪"
            )
        } else if (state.results.isNotEmpty()) {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(bottom = 100.dp)
            ) {
                state.results.forEach { group ->
                    item(key = "group_${group.id}") {
                        Text(
                            text = group.label,
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(
                                horizontal = AmitiaSpacing.Base,
                                vertical = AmitiaSpacing.Sm
                            )
                        )
                    }
                    items(group.items, key = { "${group.id}_${it.id}" }) { item ->
                        SearchResultRow(item = item) { onOpenResult(item) }
                        HorizontalDivider(
                            color = MaterialTheme.colorScheme.outlineVariant,
                            modifier = Modifier.padding(start = AmitiaSpacing.Base)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SearchResultRow(item: SearchResultItem, onClick: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = onClick
            )
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Box(
            modifier = Modifier
                .size(40.dp)
                .clip(CircleShape),
            contentAlignment = Alignment.Center
        ) {
            Surface(
                modifier = Modifier.fillMaxSize(),
                shape = CircleShape,
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        imageVector = searchItemIcon(item.type),
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
            }
        }
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = item.title,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = item.subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }
        Icon(
            imageVector = AmitiaIcons.ChevronRight,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
    }
}

@Composable
private fun IndexReadyChip() {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier
            .clip(AmitiaPillShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.Button,
                onClick = {}
            ),
        shape = AmitiaPillShape,
        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Icon(
                imageVector = AmitiaIcons.CheckCircle,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.tertiary,
                modifier = Modifier.size(AmitiaIconSize.Small)
            )
            Text(
                text = "搜索索引已就绪",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.tertiary
            )
        }
    }
}

private fun searchItemIcon(type: SearchItemType): ImageVector = when (type) {
    SearchItemType.Conversation -> AmitiaIcons.Chat
    SearchItemType.Memory -> AmitiaIcons.Memory
    SearchItemType.Character -> AmitiaIcons.Person
    SearchItemType.File -> AmitiaIcons.FileCopy
    SearchItemType.Message -> AmitiaIcons.ChatBubble
}

@Preview(name = "Global Search - Light", showBackground = true)
@Composable
private fun GlobalSearchLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Global Search", style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Preview(name = "Global Search - Dark", showBackground = true)
@Composable
private fun GlobalSearchDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
            Text("Global Search", style = MaterialTheme.typography.titleMedium)
        }
    }
}
