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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import coil.compose.AsyncImage
import coil.request.ImageRequest
import androidx.compose.ui.platform.LocalContext
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.LoadingSkeleton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaGalleryScreen(
    conversationId: String,
    onBack: () -> Unit,
    onOpenImage: (String) -> Unit,
    onOpenFile: (String) -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(conversationId) { viewModel.loadMedia(conversationId) }
    val state by viewModel.mediaState.collectAsStateWithLifecycle()
    var selectedFilter by remember { mutableStateOf(0) }
    val filters = listOf("全部", "图片", "文件", "语音", "链接")

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "媒体库", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        Box(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs)) {
            AmitiaChipSelector(
                items = filters.mapIndexed { index, label -> AmitiaChipItem(label, index == selectedFilter) },
                onToggle = { selectedFilter = it },
                multiSelect = false
            )
        }
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 4, lineHeight = 100)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadMedia(conversationId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.ImageOutlined,
                title = "暂无媒体文件",
                description = "对话中的图片、文件等会显示在这里"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val allMedia = (s as ScreenState.Content<List<MediaItem>>).data
                val filtered = filterMedia(allMedia, selectedFilter)
                if (filtered.isEmpty()) {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.FolderOutlined,
                        title = "此分类无内容",
                        description = "试试其他分类"
                    )
                } else {
                    MediaGrid(
                        media = filtered,
                        onOpenImage = onOpenImage,
                        onOpenFile = onOpenFile
                    )
                }
            }
        }
    }
}

@Composable
private fun MediaGrid(
    media: List<MediaItem>,
    onOpenImage: (String) -> Unit,
    onOpenFile: (String) -> Unit
) {
    LazyVerticalGrid(
        columns = GridCells.Fixed(3),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        items(media, key = { it.id }) { item ->
            MediaGridItem(
                item = item,
                onClick = {
                    when (item.type) {
                        MediaType.Image -> onOpenImage(item.url)
                        else -> onOpenFile(item.id)
                    }
                }
            )
        }
    }
}

@Composable
private fun MediaGridItem(
    item: MediaItem,
    onClick: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth().height(120.dp).clip(AmitiaCardShape).clickable(onClick = onClick),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Box(contentAlignment = Alignment.Center) {
            if (item.type == MediaType.Image && item.url.isNotBlank()) {
                val context = LocalContext.current
                AsyncImage(
                    model = ImageRequest.Builder(context).data(item.url).crossfade(true).build(),
                    contentDescription = item.title,
                    modifier = Modifier.fillMaxSize()
                )
            } else {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center
                ) {
                    Box(
                        modifier = Modifier.size(40.dp).clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = mediaIconFor(item.type),
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                        textAlign = androidx.compose.ui.text.style.TextAlign.Center
                    )
                    if (item.size != null) {
                        Text(
                            text = item.size,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                        )
                    }
                }
            }
            Box(
                modifier = Modifier.align(Alignment.TopEnd).padding(AmitiaSpacing.Xs)
                    .clip(RoundedCornerShape(4.dp))
                    .background(MaterialTheme.colorScheme.surface.copy(alpha = 0.8f))
                    .padding(horizontal = 6.dp, vertical = 2.dp)
            ) {
                Text(
                    text = item.timestamp,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

private fun mediaIconFor(type: MediaType): ImageVector = when (type) {
    MediaType.Image -> AmitiaIcons.Image
    MediaType.File -> AmitiaIcons.FileCopy
    MediaType.Voice -> AmitiaIcons.Mic
    MediaType.Link -> AmitiaIcons.Link
}

private fun filterMedia(all: List<MediaItem>, filterIndex: Int): List<MediaItem> {
    return when (filterIndex) {
        1 -> all.filter { it.type == MediaType.Image }
        2 -> all.filter { it.type == MediaType.File }
        3 -> all.filter { it.type == MediaType.Voice }
        4 -> all.filter { it.type == MediaType.Link }
        else -> all
    }
}

@Preview(name = "Media Gallery - Light", showBackground = true)
@Composable
private fun MediaGalleryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            MediaGrid(
                media = listOf(
                    MediaItem("med1", MediaType.Image, "", null, "截图1", "14:30"),
                    MediaItem("med2", MediaType.File, "", null, "文档.pdf", "13:00", "1.2MB"),
                    MediaItem("med3", MediaType.Voice, "", null, "语音", "12:00", "15s")
                ),
                onOpenImage = {},
                onOpenFile = {}
            )
        }
    }
}

@Preview(name = "Media Gallery - Dark", showBackground = true)
@Composable
private fun MediaGalleryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            MediaGridItem(
                item = MediaItem("med1", MediaType.Image, "https://example.com/1.png", null, "截图1", "14:30"),
                onClick = {}
            )
        }
    }
}
