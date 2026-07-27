package com.amitia.feature.voice

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun VoiceCaptionScreen(
    onBack: () -> Unit,
    viewModel: VoiceViewModel = hiltViewModel()
) {
    val state by viewModel.captionState.collectAsStateWithLifecycle()
    VoiceCaptionContent(state = state, onBack = onBack)
}

@Composable
fun VoiceCaptionContent(
    state: ScreenState<VoiceCaptionUiState>,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "实时字幕", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    InlineLoading(message = "正在加载字幕...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = state.error.title,
                    description = state.error.message,
                    onRetry = onBack,
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.ChatBubble,
                    title = "暂无字幕",
                    description = "通话开始后将自动显示实时字幕",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> CaptionList(data = state.data)
            is ScreenState.Partial -> CaptionList(data = state.data)
        }
    }
}

@Composable
private fun CaptionList(data: VoiceCaptionUiState) {
    val listState = rememberLazyListState()
    LaunchedEffect(data.captions.size) {
        if (data.autoScroll && data.captions.isNotEmpty()) {
            listState.animateScrollToItem(data.captions.lastIndex)
        }
    }
    LazyColumn(
        state = listState,
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
            ) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(RoundedCornerShape(4.dp))
                        .background(
                            if (data.isLive) MaterialTheme.colorScheme.tertiary
                            else MaterialTheme.colorScheme.onSurfaceVariant
                        )
                )
                Text(
                    text = if (data.isLive) "实时转写中" else "通话已结束",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        items(data.captions, key = { it.id }) { caption ->
            CaptionBubble(caption = caption)
        }
        if (data.isLive) {
            item {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = AmitiaSpacing.Xs),
                    horizontalArrangement = Arrangement.Center
                ) {
                    InlineLoading(message = "正在聆听...", size = 16)
                }
            }
        }
    }
}

@Composable
private fun CaptionBubble(caption: VoiceCaptionItem) {
    val alignment = if (caption.isUser) Alignment.End else Alignment.Start
    val containerColor = if (caption.isUser)
        MaterialTheme.colorScheme.primaryContainer
    else MaterialTheme.colorScheme.surfaceVariant
    val contentColor = if (caption.isUser)
        MaterialTheme.colorScheme.onPrimaryContainer
    else MaterialTheme.colorScheme.onSurface

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = alignment
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Text(
                text = caption.speaker,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = caption.timestamp,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
            )
        }
        Spacer(modifier = Modifier.height(AmitiaSpacing.Xxs))
        Surface(
            modifier = Modifier.fillMaxWidth(0.82f),
            shape = RoundedCornerShape(16.dp),
            color = containerColor
        ) {
            Column(modifier = Modifier.padding(AmitiaSpacing.Md)) {
                Text(
                    text = caption.text,
                    style = MaterialTheme.typography.bodyMedium,
                    color = contentColor
                )
                if (caption.isUncertain) {
                    Spacer(modifier = Modifier.height(AmitiaSpacing.Xs))
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Help,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.tertiary,
                            modifier = Modifier.size(AmitiaIconSize.Small)
                        )
                        Text(
                            text = "识别不确定",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.tertiary
                        )
                    }
                }
            }
        }
    }
}

@Preview(name = "Voice Caption - Light", showBackground = true)
@Composable
private fun VoiceCaptionLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceCaptionContent(
            state = ScreenState.Content(
                VoiceCaptionUiState(
                    captions = listOf(
                        VoiceCaptionItem("1", "艾米", "你好，今天感觉怎么样？", "14:30:01", isUser = false),
                        VoiceCaptionItem("2", "我", "还不错，刚忙完手头的事。", "14:30:08", isUser = true),
                        VoiceCaptionItem("3", "艾米", "那很好，记得放松一下。", "14:30:14", isUser = false)
                    )
                )
            ),
            onBack = {}
        )
    }
}

@Preview(name = "Voice Caption - Dark", showBackground = true)
@Composable
private fun VoiceCaptionDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceCaptionContent(
            state = ScreenState.Empty(),
            onBack = {}
        )
    }
}
