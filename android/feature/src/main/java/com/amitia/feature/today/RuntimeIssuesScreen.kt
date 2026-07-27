package com.amitia.feature.today

import androidx.compose.foundation.background
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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
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
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun RuntimeIssuesScreen(
    onBack: () -> Unit,
    onOpenDiagnostics: () -> Unit,
    viewModel: TodayViewModel = hiltViewModel()
) {
    val state by viewModel.issuesState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "运行异常", onBack = onBack)
        when (val s = state) {
            is ScreenState.Loading -> {
                Box(modifier = Modifier.padding(AmitiaSpacing.Base)) { LoadingSkeleton(lineCount = 4) }
            }
            is ScreenState.Error -> {
                AmitiaErrorState(
                    icon = AmitiaIcons.Error,
                    title = s.error.title,
                    description = s.error.message,
                    onRetry = viewModel::loadIssues
                )
            }
            is ScreenState.Empty -> {
                AmitiaEmptyState(
                    icon = AmitiaIcons.CheckCircle,
                    title = "一切正常",
                    description = "当前没有需要关注的运行异常"
                )
            }
            else -> {
                val issues = (state as ScreenState.Content<List<RuntimeIssue>>).data
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    item(key = "summary") {
                        IssuesSummary(count = issues.size)
                    }
                    items(issues, key = { it.id }) { issue ->
                        IssueCard(
                            issue = issue,
                            onFix = { viewModel.retryFix(issue.id) },
                            onDiagnostics = onOpenDiagnostics
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun IssuesSummary(count: Int) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.error),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = count.toString(),
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onError,
                    fontWeight = FontWeight.Medium
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "发现 $count 项异常",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "以下问题可能影响正常使用，建议尽快处理",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun IssueCard(
    issue: RuntimeIssue,
    onFix: () -> Unit,
    onDiagnostics: () -> Unit
) {
    val statusColor = amitiaStatusColor(issue.level.status)
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
            ) {
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .clip(CircleShape)
                        .background(statusColor.copy(alpha = 0.15f)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = issueIcon(issue),
                        contentDescription = null,
                        tint = statusColor,
                        modifier = Modifier.size(AmitiaIconSize.Medium)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = issue.title,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = issue.description,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 3,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (issue.fixable) {
                    TertiaryButton(
                        text = "尝试修复",
                        onClick = onFix,
                        leadingIcon = AmitiaIcons.Build
                    )
                }
                TertiaryButton(
                    text = "查看高级诊断",
                    onClick = onDiagnostics,
                    leadingIcon = AmitiaIcons.BugReport
                )
            }
        }
    }
}

private fun issueIcon(issue: RuntimeIssue): ImageVector {
    val title = issue.title
    return when {
        title.contains("存储") -> AmitiaIcons.Storage
        title.contains("渠道") -> AmitiaIcons.Hub
        title.contains("数据库") -> AmitiaIcons.Database
        title.contains("模型") -> AmitiaIcons.SmartToy
        title.contains("服务") || title.contains("启动") -> AmitiaIcons.Error
        else -> AmitiaIcons.Warning
    }
}

@Preview(name = "Runtime Issues - Light", showBackground = true)
@Composable
private fun RuntimeIssuesLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
                Text("Runtime Issues", style = MaterialTheme.typography.titleMedium)
            }
        }
    }
}

@Preview(name = "Runtime Issues - Dark", showBackground = true)
@Composable
private fun RuntimeIssuesDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            Box(modifier = Modifier.fillMaxSize().padding(16.dp)) {
                Text("Runtime Issues", style = MaterialTheme.typography.titleMedium)
            }
        }
    }
}
