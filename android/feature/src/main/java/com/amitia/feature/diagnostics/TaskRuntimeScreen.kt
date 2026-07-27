package com.amitia.feature.diagnostics

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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun TaskRuntimeScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("Task Runtime") }
        when (val ps = state.tasks) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 4) }
            is ScreenState.Content -> {
                items(ps.data) { task -> TaskCard(task) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun TaskCard(task: DiagTaskInfo) {
    val statusColor = when (task.status) {
        DiagTaskStatus.Running -> MaterialTheme.colorScheme.tertiary
        DiagTaskStatus.Pending -> MaterialTheme.colorScheme.secondary
        DiagTaskStatus.Completed -> MaterialTheme.colorScheme.primary
        DiagTaskStatus.Failed -> MaterialTheme.colorScheme.error
        DiagTaskStatus.Cancelled -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier.size(8.dp).background(statusColor, CircleShape)
                )
                Text(task.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface, modifier = Modifier.weight(1f).padding(start = AmitiaSpacing.Sm))
                Text(task.status.label, style = MaterialTheme.typography.labelMedium, color = statusColor)
            }
            DetailRow("所属扩展", task.extension)
            task.nextRun?.let { DetailRow("下次运行", it) }
            DetailRow("超时", task.timeout)
            DetailRow("重试次数", task.retryCount.toString())
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                AmitiaIconButton(icon = AmitiaIcons.Stop, contentDescription = "取消", onClick = {})
            }
        }
    }
}

@Preview(name = "Task Runtime - Light", showBackground = true)
@Composable
private fun TaskRuntimeScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            TaskRuntimeScreen()
        }
    }
}

@Preview(name = "Task Runtime - Dark", showBackground = true)
@Composable
private fun TaskRuntimeScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            TaskRuntimeScreen()
        }
    }
}
