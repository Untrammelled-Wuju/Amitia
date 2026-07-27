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
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun HooksScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("第三方 Hook") }
        when (val ps = state.hooks) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 4) }
            is ScreenState.Content -> {
                items(ps.data) { hook -> HookCard(hook) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun HookCard(hook: DiagHook) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(hook.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                Text(if (hook.enabled) "已启用" else "已禁用", style = MaterialTheme.typography.labelMedium, color = if (hook.enabled) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.onSurfaceVariant)
            }
            DetailRow("触发点", hook.triggerPoint)
            DetailRow("所属扩展", hook.extension)
            DetailRow("执行顺序", hook.order.toString())
            DetailRow("耗时", hook.duration)
            DetailRow("失败策略", hook.failurePolicy)
        }
    }
}

@Preview(name = "Hooks - Light", showBackground = true)
@Composable
private fun HooksScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            HooksScreen()
        }
    }
}

@Preview(name = "Hooks - Dark", showBackground = true)
@Composable
private fun HooksScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            HooksScreen()
        }
    }
}
