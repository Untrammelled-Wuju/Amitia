package com.amitia.feature.memory

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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.AssistChip
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.model.MemoryDto
import com.amitia.core.model.MemoryGraphDto
import com.amitia.core.model.MemoryTimelineItem

@Composable
fun MemoryScreen(
    onOpenDetail: (String) -> Unit,
    onCreate: () -> Unit,
    viewModel: MemoryViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "记忆",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        floatingActionButton = {
            FloatingActionButton(
                onClick = onCreate,
                containerColor = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.onPrimaryContainer
            ) {
                Icon(Icons.Outlined.Add, contentDescription = "新建记忆")
            }
        }
    ) { padding ->
        Column(modifier = Modifier
            .fillMaxSize()
            .padding(padding)) {
            SearchAndFilterBar(state = state, viewModel = viewModel)
            TabRow(
                selectedTabIndex = state.activeTab.ordinal,
                containerColor = MaterialTheme.colorScheme.background,
                contentColor = MaterialTheme.colorScheme.primary
            ) {
                Tab(
                    selected = state.activeTab == MemoryTab.LIST,
                    onClick = { viewModel.switchTab(MemoryTab.LIST) },
                    text = { Text(text = "记忆") }
                )
                Tab(
                    selected = state.activeTab == MemoryTab.TIMELINE,
                    onClick = { viewModel.switchTab(MemoryTab.TIMELINE) },
                    text = { Text(text = "时间线") }
                )
                Tab(
                    selected = state.activeTab == MemoryTab.GRAPH,
                    onClick = { viewModel.switchTab(MemoryTab.GRAPH) },
                    text = { Text(text = "图谱") }
                )
            }
            when (state.activeTab) {
                MemoryTab.LIST -> MemoryListTab(
                    state = state,
                    onOpenDetail = onOpenDetail,
                    onFilterType = viewModel::filterByType,
                    onFilterCharacter = viewModel::filterByCharacter
                )
                MemoryTab.TIMELINE -> TimelineTab(state = state, onOpenDetail = onOpenDetail)
                MemoryTab.GRAPH -> GraphTab(state = state, onOpenDetail = onOpenDetail)
            }
        }
    }
}

@Composable
private fun SearchAndFilterBar(state: MemoryUiState, viewModel: MemoryViewModel) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        OutlinedTextField(
            value = state.searchQuery,
            onValueChange = viewModel::searchMemory,
            modifier = Modifier.fillMaxWidth(),
            placeholder = { Text(text = "搜索记忆") },
            leadingIcon = { Icon(Icons.Outlined.Search, contentDescription = null) },
            singleLine = true
        )
    }
}

@Composable
private fun MemoryListTab(
    state: MemoryUiState,
    onOpenDetail: (String) -> Unit,
    onFilterType: (MemoryTypeFilter) -> Unit,
    onFilterCharacter: (String?) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        item {
            TypeFilterRow(state = state, onFilterType = onFilterType)
        }
        item {
            CharacterFilterRow(state = state, onFilterCharacter = onFilterCharacter)
        }
        if (state.loading && state.memories.isEmpty()) {
            item {
                Box(modifier = Modifier.fillMaxWidth().padding(32.dp), contentAlignment = Alignment.Center) {
                    AmitiaLoadingIndicator()
                }
            }
        } else if (state.memories.isEmpty()) {
            item {
                AmitiaEmptyState(
                    title = "暂无记忆",
                    subtitle = "创建一条初始记忆开始",
                    icon = Icons.Outlined.Add,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        } else {
            items(state.memories, key = { it.id }) { memory ->
                MemoryRow(memory = memory, onClick = { onOpenDetail(memory.id) })
            }
        }
    }
}

@Composable
private fun TypeFilterRow(state: MemoryUiState, onFilterType: (MemoryTypeFilter) -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        MemoryTypeFilter.entries.forEach { filter ->
            FilterChip(
                selected = state.typeFilter == filter,
                onClick = { onFilterType(filter) },
                label = {
                    Text(
                        text = when (filter) {
                            MemoryTypeFilter.ALL -> "全部"
                            MemoryTypeFilter.LONG_TERM -> "长期"
                            MemoryTypeFilter.EPISODIC -> "情景"
                            MemoryTypeFilter.INITIAL -> "初始"
                            MemoryTypeFilter.WORLD_BOOK -> "世界书"
                        }
                    )
                }
            )
        }
    }
}

@Composable
private fun CharacterFilterRow(state: MemoryUiState, onFilterCharacter: (String?) -> Unit) {
    if (state.characters.isEmpty()) return
    LazyRow(
        contentPadding = PaddingValues(vertical = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        item {
            FilterChip(
                selected = state.filterCharacterId == null,
                onClick = { onFilterCharacter(null) },
                label = { Text(text = "全部角色") }
            )
        }
        items(state.characters, key = { it.id }) { character ->
            FilterChip(
                selected = state.filterCharacterId == character.id,
                onClick = { onFilterCharacter(character.id) },
                label = { Text(text = character.name) }
            )
        }
    }
}

@Composable
private fun MemoryRow(memory: MemoryDto, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = memory.content,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            Spacer(modifier = Modifier.height(4.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                val type = memory.type
                if (!type.isNullOrBlank()) {
                    AssistChip(onClick = {}, label = { Text(text = type) })
                    Spacer(modifier = Modifier.size(8.dp))
                }
                Text(
                    text = memory.createdAt ?: "未知时间",
                    style = MaterialTheme.typography.labelSmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
        }
    }
}

@Composable
private fun TimelineTab(state: MemoryUiState, onOpenDetail: (String) -> Unit) {
    if (state.timeline.isEmpty() && !state.loading) {
        AmitiaEmptyState(
            title = "尚无时间线",
            subtitle = "切换到「记忆」标签查看或创建",
            modifier = Modifier.fillMaxWidth()
        )
        return
    }
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        itemsIndexed(state.timeline, key = { _, item -> item.id }) { _, item ->
            TimelineRow(item = item, onClick = { onOpenDetail(item.id) })
        }
    }
}

@Composable
private fun TimelineRow(item: MemoryTimelineItem, onClick: () -> Unit) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(modifier = Modifier.padding(16.dp)) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .background(MaterialTheme.colorScheme.tertiary, RoundedCornerShape(4.dp))
            )
            Spacer(modifier = Modifier.size(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.content,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = item.timestamp,
                    style = MaterialTheme.typography.labelSmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
        }
    }
}

@Composable
private fun GraphTab(state: MemoryUiState, onOpenDetail: (String) -> Unit) {
    val graph: MemoryGraphDto = state.graphSummary ?: run {
        AmitiaEmptyState(
            title = "尚未加载图谱",
            subtitle = "等待加载完成",
            modifier = Modifier.fillMaxWidth()
        )
        return
    }
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        item {
            Text(
                text = "节点 ${graph.nodes.size} · 关系 ${graph.edges.size}",
                style = MaterialTheme.typography.labelMedium,
                color = AmitiaColors.OnSurfaceMuted,
                modifier = Modifier.padding(vertical = 4.dp)
            )
        }
        items(graph.nodes, key = { it.id }) { node ->
            Surface(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { onOpenDetail(node.id) },
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = node.label,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    val nodeType = node.type
                    if (nodeType != null) {
                        Text(
                            text = nodeType,
                            style = MaterialTheme.typography.labelSmall,
                            color = AmitiaColors.OnSurfaceMuted
                        )
                    }
                }
            }
        }
    }
}
