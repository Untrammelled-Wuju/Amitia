package com.amitia.feature.emoji

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
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
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun EmojiImportResultScreen(
    onBack: () -> Unit,
    onComplete: () -> Unit,
    onFixMeaning: () -> Unit,
    viewModel: EmojiViewModel = hiltViewModel()
) {
    val state by viewModel.importResultState.collectAsStateWithLifecycle()
    EmojiImportResultContent(
        state = state,
        onBack = onBack,
        onComplete = onComplete,
        onFixMeaning = onFixMeaning
    )
}

@Composable
fun EmojiImportResultContent(
    state: ScreenState<EmojiImportResult>,
    onBack: () -> Unit,
    onComplete: () -> Unit,
    onFixMeaning: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "导入结果", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    InlineLoading(message = "处理中...")
                }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = "导入失败",
                    description = state.error.message,
                    onRetry = {},
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Image,
                    title = "无导入结果",
                    modifier = Modifier.fillMaxSize()
                )
            }
            is ScreenState.Content -> ImportResultBody(
                result = state.data,
                onComplete = onComplete,
                onFixMeaning = onFixMeaning
            )
            is ScreenState.Partial -> ImportResultBody(
                result = state.data,
                onComplete = onComplete,
                onFixMeaning = onFixMeaning
            )
        }
    }
}

@Composable
private fun ImportResultBody(
    result: EmojiImportResult,
    onComplete: () -> Unit,
    onFixMeaning: () -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item(key = "summary") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(20.dp),
                color = MaterialTheme.colorScheme.primaryContainer
            ) {
                Column(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = "导入完成",
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onPrimaryContainer,
                        fontWeight = FontWeight.Medium
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        ResultStat(count = result.successCount, label = "成功", color = MaterialTheme.colorScheme.tertiary)
                        ResultStat(count = result.duplicateCount, label = "重复", color = MaterialTheme.colorScheme.onSurfaceVariant)
                        ResultStat(count = result.failedCount, label = "失败", color = MaterialTheme.colorScheme.error)
                        ResultStat(count = result.needsMeaningCount, label = "待完善", color = MaterialTheme.colorScheme.primary)
                    }
                }
            }
        }

        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (result.needsMeaningCount > 0) {
                    SecondaryButton(
                        text = "完善含义 (${result.needsMeaningCount})",
                        onClick = onFixMeaning,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Edit
                    )
                }
                if (result.failedCount > 0) {
                    SecondaryButton(
                        text = "查看失败项 (${result.failedCount})",
                        onClick = {},
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Error
                    )
                }
            }
        }

        item(key = "detail_header") {
            Text(
                text = "详细列表",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(vertical = AmitiaSpacing.Xs)
            )
        }

        items(result.items, key = { it.id }) { item ->
            ImportResultRow(item = item)
        }

        item(key = "complete") {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "完成",
                onClick = onComplete,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Composable
private fun RowScope.ResultStat(count: Int, label: String, color: Color) {
    Column(
        modifier = Modifier.weight(1f),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = count.toString(),
            style = MaterialTheme.typography.headlineSmall,
            color = color,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
        )
    }
}

@Composable
private fun ImportResultRow(item: EmojiImportItem) {
    val (icon, color, statusText) = when (item.status) {
        EmojiImportStatus.Success -> Triple(AmitiaIcons.CheckCircle, MaterialTheme.colorScheme.tertiary, "成功")
        EmojiImportStatus.Duplicate -> Triple(AmitiaIcons.ContentCopy, MaterialTheme.colorScheme.onSurfaceVariant, "重复")
        EmojiImportStatus.Failed -> Triple(AmitiaIcons.Error, MaterialTheme.colorScheme.error, "失败")
        EmojiImportStatus.NeedsMeaning -> Triple(AmitiaIcons.Help, MaterialTheme.colorScheme.primary, "待完善")
        EmojiImportStatus.Pending -> Triple(AmitiaIcons.Schedule, MaterialTheme.colorScheme.onSurfaceVariant, "待处理")
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .clip(RoundedCornerShape(8.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Image,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                    modifier = Modifier.size(AmitiaIconSize.Medium)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "图片 ${item.id}",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (item.meaning != null) {
                    Text(
                        text = item.meaning,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                if (item.errorMessage != null) {
                    Text(
                        text = item.errorMessage,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
                if (item.duplicateOf != null) {
                    Text(
                        text = "与图片 ${item.duplicateOf} 重复",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            Surface(
                shape = RoundedCornerShape(6.dp),
                color = color.copy(alpha = 0.12f)
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    Icon(
                        imageVector = icon,
                        contentDescription = null,
                        tint = color,
                        modifier = Modifier.size(12.dp)
                    )
                    Text(
                        text = statusText,
                        style = MaterialTheme.typography.labelSmall,
                        color = color,
                        fontWeight = FontWeight.Medium
                    )
                }
            }
        }
    }
}

@Preview(name = "Import Result - Light", showBackground = true)
@Composable
private fun EmojiImportResultLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiImportResultContent(
            state = ScreenState.Content(
                EmojiImportResult(
                    successCount = 3,
                    duplicateCount = 1,
                    failedCount = 1,
                    needsMeaningCount = 1,
                    items = listOf(
                        EmojiImportItem("1", "", EmojiImportStatus.Success, "开心"),
                        EmojiImportItem("2", "", EmojiImportStatus.Success, "惊讶"),
                        EmojiImportItem("3", "", EmojiImportStatus.Duplicate, duplicateOf = "1"),
                        EmojiImportItem("4", "", EmojiImportStatus.NeedsMeaning),
                        EmojiImportItem("5", "", EmojiImportStatus.Failed, errorMessage = "文件格式不支持"),
                        EmojiImportItem("6", "", EmojiImportStatus.Success, "思考")
                    )
                )
            ),
            onBack = {},
            onComplete = {},
            onFixMeaning = {}
        )
    }
}

@Preview(name = "Import Result - Dark", showBackground = true)
@Composable
private fun EmojiImportResultDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiImportResultContent(
            state = ScreenState.Loading,
            onBack = {},
            onComplete = {},
            onFixMeaning = {}
        )
    }
}
