package com.amitia.feature.character.pet

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Animation
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.LoopMode
import com.amitia.feature.character.model.PetActionItem
import com.amitia.feature.character.model.PetActionStatus

@Composable
fun CharacterPetActionLibraryScreen(
    onBack: () -> Unit,
    onGenerate: () -> Unit,
    viewModel: CharacterDetailViewModel = hiltViewModel()
) {
    val state by viewModel.petActionState.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "桌宠动作库",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    IconButton(onClick = onGenerate) {
                        Icon(Icons.Outlined.AutoAwesome, contentDescription = "生成动作")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        when (state) {
            ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> com.amitia.core.designsystem.component.AmitiaErrorState(
                icon = Icons.Outlined.Animation,
                title = "加载失败",
                description = (state as ScreenState.Error).error.message,
                onRetry = { viewModel.loadPetActions() },
                modifier = Modifier.fillMaxSize().padding(padding)
            )
            is ScreenState.Content -> ActionLibraryContent(
                actions = (state as ScreenState.Content).data,
                onGenerate = onGenerate,
                modifier = Modifier.padding(padding)
            )
            else -> Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Text("暂无数据", color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Composable
private fun ActionLibraryContent(
    actions: List<PetActionItem>,
    onGenerate: () -> Unit,
    modifier: Modifier = Modifier
) {
    val grouped = actions.groupBy { it.category }

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "summary") {
            ActionSummaryCard(actions)
        }
        grouped.forEach { (category, items) ->
            item(key = "category_$category") {
                AmitiaSection(title = category, subtitle = "${items.size} 个动作") {
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        items.forEach { action ->
                            ActionRow(action, onGenerate)
                        }
                    }
                }
            }
        }
        item(key = "generate_button") {
            PrimaryButton(
                text = "生成新动作",
                onClick = onGenerate,
                leadingIcon = Icons.Outlined.AutoAwesome,
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}

@Composable
private fun ActionSummaryCard(actions: List<PetActionItem>) {
    val readyCount = actions.count { it.status == PetActionStatus.Ready }
    val pendingCount = actions.count { it.status == PetActionStatus.Pending }
    val generatingCount = actions.count { it.status == PetActionStatus.Generating }
    val missingCount = actions.count { it.status == PetActionStatus.Missing }

    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "动作概览",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                SummaryStat("就绪", "$readyCount", MaterialTheme.colorScheme.primary)
                SummaryStat("待生成", "$pendingCount", MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f))
                SummaryStat("生成中", "$generatingCount", MaterialTheme.colorScheme.tertiary)
                SummaryStat("缺失", "$missingCount", MaterialTheme.colorScheme.error)
            }
        }
    }
}

@Composable
private fun SummaryStat(label: String, value: String, color: androidx.compose.ui.graphics.Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            style = MaterialTheme.typography.titleMedium,
            color = color,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = color.copy(alpha = 0.7f)
        )
    }
}

@Composable
private fun ActionRow(
    action: PetActionItem,
    onGenerate: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(statusBackgroundColor(action.status)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.Animation,
                    contentDescription = null,
                    tint = statusContentColor(action.status),
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = action.name,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                if (action.frameCount > 0) {
                    Text(
                        text = "${action.frameCount} 帧 · ${loopModeLabel(action.loopMode)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                if (action.boundEmotion != null) {
                    Text(
                        text = "绑定情绪：${action.boundEmotion}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                    )
                }
            }
            Column(horizontalAlignment = Alignment.End) {
                Surface(
                    shape = RoundedCornerShape(8.dp),
                    color = statusBackgroundColor(action.status).copy(alpha = 0.3f)
                ) {
                    Text(
                        text = statusLabel(action.status),
                        style = MaterialTheme.typography.labelSmall,
                        color = statusContentColor(action.status),
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                    )
                }
                if (action.status == PetActionStatus.Missing || action.status == PetActionStatus.Pending) {
                    Spacer(modifier = Modifier.height(4.dp))
                    TertiaryButton(text = "生成", onClick = onGenerate)
                } else if (action.status == PetActionStatus.Ready) {
                    Spacer(modifier = Modifier.height(4.dp))
                    IconButton(onClick = {}, modifier = Modifier.size(32.dp)) {
                        Icon(
                            imageVector = Icons.Outlined.PlayArrow,
                            contentDescription = "预览",
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun statusBackgroundColor(status: PetActionStatus): androidx.compose.ui.graphics.Color = when (status) {
    PetActionStatus.Ready -> MaterialTheme.colorScheme.tertiaryContainer
    PetActionStatus.Pending -> MaterialTheme.colorScheme.surfaceVariant
    PetActionStatus.Generating -> MaterialTheme.colorScheme.primaryContainer
    PetActionStatus.Missing -> MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f)
}

@Composable
private fun statusContentColor(status: PetActionStatus): androidx.compose.ui.graphics.Color = when (status) {
    PetActionStatus.Ready -> MaterialTheme.colorScheme.onTertiaryContainer
    PetActionStatus.Pending -> MaterialTheme.colorScheme.onSurfaceVariant
    PetActionStatus.Generating -> MaterialTheme.colorScheme.onPrimaryContainer
    PetActionStatus.Missing -> MaterialTheme.colorScheme.error
}

private fun statusLabel(status: PetActionStatus): String = when (status) {
    PetActionStatus.Ready -> "就绪"
    PetActionStatus.Pending -> "待生成"
    PetActionStatus.Generating -> "生成中"
    PetActionStatus.Missing -> "缺失"
}

private fun loopModeLabel(mode: LoopMode): String = when (mode) {
    LoopMode.None -> "不循环"
    LoopMode.Loop -> "循环"
    LoopMode.PingPong -> "往返"
    LoopMode.Once -> "单次"
}

@Preview(name = "PetActionLibrary - Light", showBackground = true)
@Composable
private fun CharacterPetActionLibraryLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ActionLibraryContent(
                actions = listOf(
                    PetActionItem("1", "待机", "基础", PetActionStatus.Ready, 8, LoopMode.Loop, null, true),
                    PetActionItem("2", "开心", "情绪", PetActionStatus.Pending, 0, LoopMode.None, "愉悦", false)
                ),
                onGenerate = {}
            )
        }
    }
}

@Preview(name = "PetActionLibrary - Dark", showBackground = true)
@Composable
private fun CharacterPetActionLibraryDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ActionLibraryContent(
                actions = listOf(),
                onGenerate = {}
            )
        }
    }
}
