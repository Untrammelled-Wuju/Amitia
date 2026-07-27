package com.amitia.feature.voicecenter

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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSearchField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun VoiceLibraryScreen(
    onBack: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val state by viewModel.voicesState.collectAsStateWithLifecycle()
    var query by remember { mutableStateOf("") }
    LaunchedEffect(Unit) { viewModel.loadVoices() }
    VoiceLibraryContent(
        state = state,
        query = query,
        onQueryChange = { query = it },
        onBack = onBack,
        onToggleFavorite = viewModel::toggleFavorite,
        onRetry = viewModel::loadVoices
    )
}

@Composable
fun VoiceLibraryContent(
    state: ScreenState<List<VoiceItemUiModel>>,
    query: String,
    onQueryChange: (String) -> Unit,
    onBack: () -> Unit,
    onToggleFavorite: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "声音库", onBack = onBack)
        Column(modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)) {
            AmitiaSearchField(
                value = query,
                onValueChange = onQueryChange,
                onClear = { onQueryChange("") },
                placeholder = "搜索声音..."
            )
        }
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "加载声音库...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.MicNone,
                title = "暂无声音",
                description = "请配置 TTS Provider 后再使用声音库",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> {
                val filtered = if (query.isBlank()) state.data
                else state.data.filter { it.name.contains(query, ignoreCase = true) || it.provider.contains(query, ignoreCase = true) }
                val grouped = filtered.groupBy { it.provider }
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    grouped.forEach { (provider, voices) ->
                        item(key = "header_${provider}") {
                            AmitiaSectionHeader(title = "$provider (${voices.size})")
                        }
                        items(voices, key = { it.id }) { voice ->
                            VoiceCard(voice = voice, onToggleFavorite = { onToggleFavorite(voice.id) })
                        }
                    }
                    if (filtered.isEmpty()) {
                        item(key = "no_results") {
                            AmitiaEmptyState(
                                icon = AmitiaIcons.Search,
                                title = "未找到匹配的声音",
                                description = "尝试更换关键词",
                                modifier = Modifier.fillMaxWidth()
                            )
                        }
                    }
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun VoiceCard(voice: VoiceItemUiModel, onToggleFavorite: () -> Unit) {
    val interactionSource = remember { MutableInteractionSource() }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            val playInteraction = remember { MutableInteractionSource() }
            Box(
                modifier = Modifier.size(44.dp).clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primaryContainer)
                    .clickable(
                        interactionSource = playInteraction,
                        indication = null,
                        role = Role.Button,
                        onClick = {}
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.PlayArrow,
                    contentDescription = "试听",
                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Text(
                        text = voice.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    if (voice.customCloned) {
                        Surface(shape = RoundedCornerShape(6.dp), color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.5f)) {
                            Text(
                                text = "克隆",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onTertiaryContainer,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp)
                            )
                        }
                    }
                }
                val descParts = buildList {
                    add(voice.language)
                    voice.gender?.let { add(it) }
                    voice.style?.let { add(it) }
                }
                Text(
                    text = descParts.joinToString(" · "),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (voice.usedByCharacters.isNotEmpty()) {
                    Text(
                        text = "使用: ${voice.usedByCharacters.joinToString(", ")}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            val favInteraction = remember { MutableInteractionSource() }
            Box(
                modifier = Modifier.size(36.dp).clip(CircleShape)
                    .clickable(
                        interactionSource = favInteraction,
                        indication = null,
                        role = Role.Button,
                        onClick = onToggleFavorite
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = if (voice.isFavorite) AmitiaIcons.Star else AmitiaIcons.StarBorder,
                    contentDescription = "收藏",
                    tint = if (voice.isFavorite) AmitiaStateColors.Degraded else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                    modifier = Modifier.size(20.dp)
                )
            }
        }
    }
}

@Preview(name = "VoiceLibrary - Light", showBackground = true)
@Composable
private fun VoiceLibraryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceLibraryContent(
            state = ScreenState.Content(
                listOf(
                    VoiceItemUiModel("1", "晓晓", "Azure", "zh-CN", "女", "温柔", isFavorite = true, usedByCharacters = listOf("艾米")),
                    VoiceItemUiModel("2", "云健", "Azure", "zh-CN", "男", "沉稳"),
                    VoiceItemUiModel("3", "克隆声音 A", "自定义", "zh-CN", null, null, customCloned = true, usedByCharacters = listOf("星河"))
                )
            ),
            query = "",
            onQueryChange = {},
            onBack = {},
            onToggleFavorite = {},
            onRetry = {}
        )
    }
}

@Preview(name = "VoiceLibrary - Dark", showBackground = true)
@Composable
private fun VoiceLibraryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceLibraryContent(
            state = ScreenState.Empty(),
            query = "",
            onQueryChange = {},
            onBack = {},
            onToggleFavorite = {},
            onRetry = {}
        )
    }
}
