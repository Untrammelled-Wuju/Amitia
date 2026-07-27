package com.amitia.feature.diagnostics

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.amitiaStatusColor
import com.amitia.core.designsystem.component.amitiaStatusText

@Composable
fun WasmRuntimeScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("WASM Runtime") }
        when (val ps = state.wasmModules) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 3) }
            is ScreenState.Content -> {
                items(ps.data) { module -> WasmModuleCard(module) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun WasmModuleCard(module: DiagWasmModule) {
    val statusColor = amitiaStatusColor(module.instanceStatus)
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Box(modifier = Modifier.size(8.dp).background(statusColor, CircleShape))
                Text(module.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface, modifier = Modifier.weight(1f).padding(start = AmitiaSpacing.Sm))
                Text("v${module.version}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(amitiaStatusText(module.instanceStatus), style = MaterialTheme.typography.labelMedium, color = statusColor, modifier = Modifier.padding(start = AmitiaSpacing.Sm))
            }
            DetailRow("内存限制", module.memoryLimit)
            DetailRow("CPU 时间", module.cpuTime)
            Text("权限", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                module.permissions.forEach { perm ->
                    Surface(shape = AmitiaPillShape, color = MaterialTheme.colorScheme.surfaceVariant) {
                        Text(perm, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp))
                    }
                }
            }
        }
    }
}

@Preview(name = "WASM Runtime - Light", showBackground = true)
@Composable
private fun WasmRuntimeScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            WasmRuntimeScreen()
        }
    }
}

@Preview(name = "WASM Runtime - Dark", showBackground = true)
@Composable
private fun WasmRuntimeScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            WasmRuntimeScreen()
        }
    }
}
