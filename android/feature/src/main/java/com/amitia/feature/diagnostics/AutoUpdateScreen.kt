package com.amitia.feature.diagnostics

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.amitiaStatusText

@Composable
fun AutoUpdateScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("自动更新") }
        item {
            Text(
                text = "覆盖桌面菜单、托盘、快捷键扩展和 Android 可用扩展的更新状态",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        when (val ps = state.updates) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 3) }
            is ScreenState.Content -> {
                items(ps.data) { update -> UpdateCard(update) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun UpdateCard(update: DiagUpdateInfo) {
    val statusColor = amitiaStatusColor(update.status)
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = update.target,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = amitiaStatusText(update.status),
                    style = MaterialTheme.typography.labelMedium,
                    color = statusColor
                )
            }
            DetailRow("当前版本", update.currentVersion)
            update.availableVersion?.let {
                DetailRow("可用版本", it)
            } ?: DetailRow("可用版本", "已是最新")
            DetailRow("包兼容性", if (update.compatible) "兼容" else "不兼容")
            DetailRow("最近检查", update.lastCheck)
        }
    }
}

@Preview(name = "AutoUpdate - Light", showBackground = true)
@Composable
private fun AutoUpdateScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            AutoUpdateScreen()
        }
    }
}

@Preview(name = "AutoUpdate - Dark", showBackground = true)
@Composable
private fun AutoUpdateScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            AutoUpdateScreen()
        }
    }
}
