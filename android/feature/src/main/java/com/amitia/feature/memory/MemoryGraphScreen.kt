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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
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
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

private const val MAX_VISIBLE_NODES = 30

@Composable
fun MemoryGraphScreen(
    onBack: () -> Unit,
    viewModel: MemoryPagesViewModel = hiltViewModel()
) {
    val state by viewModel.graphState.collectAsStateWithLifecycle()
    var filter by remember { mutableStateOf(GraphNodeFilter.All) }
    var useListMode by remember { mutableStateOf(false) }
    MemoryGraphContent(
        state = state,
        filter = filter,
        onFilterChange = { filter = it },
        useListMode = useListMode,
        onToggleMode = { useListMode = !useListMode },
        onBack = onBack
    )
}

@Composable
fun MemoryGraphContent(
    state: ScreenState<MemoryGraphView>,
    filter: GraphNodeFilter,
    onFilterChange: (GraphNodeFilter) -> Unit,
    useListMode: Boolean,
    onToggleMode: () -> Unit,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = "记忆图谱",
            onBack = onBack,
            actions = {
                val interactionSource = remember { MutableInteractionSource() }
                Surface(
                    modifier = Modifier
                        .clip(AmitiaPillShape)
                        .clickable(
                            interactionSource = interactionSource,
                            indication = null,
                            role = Role.Button,
                            onClick = onToggleMode
                        ),
                    shape = AmitiaPillShape,
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                    ) {
                        Icon(
                            imageVector = if (useListMode) AmitiaIcons.Hub else AmitiaIcons.ViewList,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                        Text(
                            text = if (useListMode) "图谱" else "列表",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
        )
        FilterRow(filter = filter, onFilterChange = onFilterChange)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "加载图谱...")
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
                    icon = AmitiaIcons.Hub,
                    title = "暂无图谱数据",
                    description = "记忆图谱会在记忆积累后自动生成",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> {
                val filteredGraph = filterGraph(state.data, filter)
                val cappedGraph = capNodes(filteredGraph, MAX_VISIBLE_NODES)
                if (useListMode) {
                    GraphListView(graph = cappedGraph)
                } else {
                    GraphCanvasView(graph = cappedGraph, totalCount = state.data.totalNodes)
                }
            }
            is ScreenState.Partial -> {
                val filteredGraph = filterGraph(state.data, filter)
                val cappedGraph = capNodes(filteredGraph, MAX_VISIBLE_NODES)
                if (useListMode) {
                    GraphListView(graph = cappedGraph)
                } else {
                    GraphCanvasView(graph = cappedGraph, totalCount = state.data.totalNodes)
                }
            }
        }
    }
}

@Composable
private fun FilterRow(filter: GraphNodeFilter, onFilterChange: (GraphNodeFilter) -> Unit) {
    val filters = listOf(
        GraphNodeFilter.All to "全部",
        GraphNodeFilter.Character to "角色",
        GraphNodeFilter.Person to "人物",
        GraphNodeFilter.Place to "地点",
        GraphNodeFilter.Event to "事件"
    )
    LazyRow(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(filters.size) { index ->
            val (type, label) = filters[index]
            val selected = filter == type
            val interactionSource = remember { MutableInteractionSource() }
            Surface(
                modifier = Modifier
                    .clip(AmitiaPillShape)
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Tab,
                        onClick = { onFilterChange(type) }
                    ),
                shape = AmitiaPillShape,
                color = if (selected) MaterialTheme.colorScheme.primaryContainer
                else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Text(
                    text = label,
                    style = MaterialTheme.typography.labelMedium,
                    color = if (selected) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
                )
            }
        }
    }
}

@Composable
private fun GraphCanvasView(graph: MemoryGraphView, totalCount: Int) {
    Box(modifier = Modifier.fillMaxSize()) {
        androidx.compose.foundation.Canvas(
            modifier = Modifier.fillMaxSize()
        ) {
            val canvasWidth = size.width
            val canvasHeight = size.height
            val nodePositions = graph.nodes.associate { node ->
                node.id to Offset(
                    x = canvasWidth * node.x,
                    y = canvasHeight * node.y
                )
            }
            val edgeColor = Color.Gray.copy(alpha = 0.3f)
            graph.edges.forEach { edge ->
                val source = nodePositions[edge.source]
                val target = nodePositions[edge.target]
                if (source != null && target != null) {
                    drawLine(
                        color = edgeColor,
                        start = source,
                        end = target,
                        strokeWidth = 2f
                    )
                }
            }
        }
        graph.nodes.forEach { node ->
            val nodeColor = nodeTypeColor(node.type)
            val interactionSource = remember { MutableInteractionSource() }
            Surface(
                modifier = Modifier
                    .padding(
                        start = (node.x * 320).dp - 28.dp,
                        top = (node.y * 480).dp - 28.dp
                    )
                    .size(56.dp)
                    .clip(CircleShape)
                    .clickable(
                        interactionSource = interactionSource,
                        indication = null,
                        role = Role.Button,
                        onClick = {}
                    ),
                shape = CircleShape,
                color = nodeColor
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        text = node.label.take(2),
                        style = MaterialTheme.typography.labelSmall,
                        color = Color.White,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1
                    )
                }
            }
        }
        Surface(
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .padding(AmitiaSpacing.Base),
            shape = RoundedCornerShape(12.dp),
            color = MaterialTheme.colorScheme.surface.copy(alpha = 0.9f)
        ) {
            Text(
                text = "显示 ${graph.nodes.size} / $totalCount 个节点",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
            )
        }
    }
}

@Composable
private fun GraphListView(graph: MemoryGraphView) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(graph.nodes, key = { it.id }) { node ->
            val edges = graph.edges.filter { it.source == node.id || it.target == node.id }
            GraphNodeRow(node = node, edgeCount = edges.size, edges = edges, allNodes = graph.nodes)
        }
    }
}

@Composable
private fun GraphNodeRow(
    node: MemoryGraphNodeView,
    edgeCount: Int,
    edges: List<MemoryGraphEdgeView>,
    allNodes: List<MemoryGraphNodeView>
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(10.dp)
                        .clip(CircleShape)
                        .background(nodeTypeColor(node.type))
                )
                Text(
                    text = node.label,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = nodeTypeLabel(node.type),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            if (edges.isNotEmpty()) {
                Text(
                    text = "关联 ${edgeCount} 个节点",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                edges.take(3).forEach { edge ->
                    val targetId = if (edge.source == node.id) edge.target else edge.source
                    val targetNode = allNodes.find { it.id == targetId }
                    if (targetNode != null) {
                        Text(
                            text = "  ${edge.relation} -> ${targetNode.label}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.primary
                        )
                    }
                }
            }
        }
    }
}

private fun nodeTypeColor(type: String): Color {
    return when (type) {
        "character" -> Color(0xFFE85D4E)
        "person" -> Color(0xFFF5A623)
        "place" -> Color(0xFF4CAF50)
        "event" -> Color(0xFF2196F3)
        else -> Color(0xFF9E9E9E)
    }
}

private fun nodeTypeLabel(type: String): String {
    return when (type) {
        "character" -> "角色"
        "person" -> "人物"
        "place" -> "地点"
        "event" -> "事件"
        else -> "其他"
    }
}

private fun filterGraph(graph: MemoryGraphView, filter: GraphNodeFilter): MemoryGraphView {
    if (filter == GraphNodeFilter.All) return graph
    val filteredNodes = graph.nodes.filter { it.type == filter.name.lowercase() }
    val filteredIds = filteredNodes.map { it.id }.toSet()
    val filteredEdges = graph.edges.filter { it.source in filteredIds && it.target in filteredIds }
    return graph.copy(nodes = filteredNodes, edges = filteredEdges)
}

private fun capNodes(graph: MemoryGraphView, max: Int): MemoryGraphView {
    if (graph.nodes.size <= max) return graph
    val cappedNodes = graph.nodes.take(max)
    val cappedIds = cappedNodes.map { it.id }.toSet()
    val cappedEdges = graph.edges.filter { it.source in cappedIds && it.target in cappedIds }
    return graph.copy(nodes = cappedNodes, edges = cappedEdges)
}

@Preview(name = "Graph - Light", showBackground = true)
@Composable
private fun MemoryGraphLightPreview() {
    AmitiaTheme(darkTheme = false) {
        MemoryGraphContent(
            state = ScreenState.Content(
                MemoryGraphView(
                    nodes = listOf(
                        MemoryGraphNodeView("1", "小明", "character", 0.5f, 0.2f),
                        MemoryGraphNodeView("2", "艾米", "character", 0.5f, 0.5f),
                        MemoryGraphNodeView("3", "软件开发", "person", 0.2f, 0.7f),
                        MemoryGraphNodeView("4", "沿海城市", "place", 0.8f, 0.7f),
                        MemoryGraphNodeView("5", "下午会议", "event", 0.3f, 0.3f)
                    ),
                    edges = listOf(
                        MemoryGraphEdgeView("1", "2", "朋友"),
                        MemoryGraphEdgeView("1", "3", "职业"),
                        MemoryGraphEdgeView("2", "4", "所在地"),
                        MemoryGraphEdgeView("1", "5", "参与")
                    ),
                    totalNodes = 28,
                    totalEdges = 42
                )
            ),
            filter = GraphNodeFilter.All,
            onFilterChange = {},
            useListMode = false,
            onToggleMode = {},
            onBack = {}
        )
    }
}

@Preview(name = "Graph - Dark", showBackground = true)
@Composable
private fun MemoryGraphDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        MemoryGraphContent(
            state = ScreenState.Empty(),
            filter = GraphNodeFilter.All,
            onFilterChange = {},
            useListMode = true,
            onToggleMode = {},
            onBack = {}
        )
    }
}
