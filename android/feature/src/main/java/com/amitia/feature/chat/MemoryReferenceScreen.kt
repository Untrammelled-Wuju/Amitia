package com.amitia.feature.chat

import androidx.compose.foundation.background
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
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.MemoryReferenceCard

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MemoryReferenceScreen(
    messageId: String,
    onBack: () -> Unit,
    onOpenMemory: (String) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(messageId) { viewModel.loadMemoryReferences(messageId) }
    val state by viewModel.memoryReferencesState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "记忆引用", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 4, lineHeight = 64)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadMemoryReferences(messageId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Memory,
                title = "暂无记忆引用",
                description = "该消息未关联任何记忆"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val references = (s as ScreenState.Content<List<MemoryReferenceDetail>>).data
                MemoryReferenceList(references = references, onOpenMemory = onOpenMemory)
            }
        }
    }
}

@Composable
private fun MemoryReferenceList(
    references: List<MemoryReferenceDetail>,
    onOpenMemory: (String) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item(key = "reference_count") {
            Text(
                text = "共 ${references.size} 条记忆引用",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        items(references, key = { it.id }) { ref ->
            MemoryReferenceItem(detail = ref, onClick = { onOpenMemory(ref.id) })
        }
    }
}

@Composable
private fun MemoryReferenceItem(
    detail: MemoryReferenceDetail,
    onClick: () -> Unit
) {
    AmitiaContentSurface(modifier = Modifier.fillMaxWidth().clip(AmitiaCardShape)) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier.size(AmitiaIconSize.Medium).clip(CircleShape)
                        .background(MaterialTheme.colorScheme.secondaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Memory,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onSecondaryContainer,
                        modifier = Modifier.size(AmitiaIconSize.Small)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = detail.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = detail.source,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = "${(detail.relevance * 100).toInt()}%",
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.Bold
                )
            }
            Text(
                text = detail.content,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis
            )
            LinearProgressIndicator(
                progress = { detail.relevance },
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant
            )
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = "创建于 ${detail.createdAt}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
                Spacer(modifier = Modifier.weight(1f))
                Text(
                    text = "引用于 ${detail.quotedIn}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
        }
    }
}

@Preview(name = "Memory Reference - Light", showBackground = true)
@Composable
private fun MemoryReferenceLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            MemoryReferenceItem(
                detail = MemoryReferenceDetail(
                    id = "mr1",
                    title = "用户位置偏好",
                    content = "用户通常在上海活动，关注上海的天气情况",
                    source = "对话记录",
                    relevance = 0.95f,
                    createdAt = "昨天",
                    quotedIn = "第3条回复"
                ),
                onClick = {}
            )
        }
    }
}

@Preview(name = "Memory Reference - Dark", showBackground = true)
@Composable
private fun MemoryReferenceDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            MemoryReferenceItem(
                detail = MemoryReferenceDetail(
                    id = "mr2",
                    title = "回复风格偏好",
                    content = "用户偏好简洁的回复风格",
                    source = "用户反馈",
                    relevance = 0.88f,
                    createdAt = "3天前",
                    quotedIn = "第5条回复"
                ),
                onClick = {}
            )
        }
    }
}
