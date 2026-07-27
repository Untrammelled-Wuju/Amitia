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
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun TrustedServiceRuntimeScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("Trusted Service Runtime") }
        when (val ps = state.trustedServices) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 3) }
            is ScreenState.Content -> {
                items(ps.data) { service -> TrustedServiceCard(service) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun TrustedServiceCard(service: DiagTrustedService) {
    val lifecycleColor = when (service.lifecycle) {
        DiagLifecycle.Active -> MaterialTheme.colorScheme.tertiary
        DiagLifecycle.Idle -> MaterialTheme.colorScheme.onSurfaceVariant
        DiagLifecycle.Stopped -> MaterialTheme.colorScheme.onSurfaceVariant
        DiagLifecycle.Crashed -> MaterialTheme.colorScheme.error
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Box(modifier = Modifier.size(8.dp).background(lifecycleColor, CircleShape))
                Text(service.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface, modifier = Modifier.weight(1f).padding(start = AmitiaSpacing.Sm))
                Text(service.lifecycle.label, style = MaterialTheme.typography.labelMedium, color = lifecycleColor)
            }
            DetailRow("调用状态", service.callStatus)
            DetailRow("崩溃次数", service.crashes.toString())
            DetailRow("重启次数", service.restarts.toString())
            Text("权限", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                service.permissions.forEach { perm ->
                    Surface(shape = AmitiaPillShape, color = MaterialTheme.colorScheme.surfaceVariant) {
                        Text(perm, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp))
                    }
                }
            }
        }
    }
}

@Preview(name = "Trusted Service - Light", showBackground = true)
@Composable
private fun TrustedServiceRuntimeScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            TrustedServiceRuntimeScreen()
        }
    }
}

@Preview(name = "Trusted Service - Dark", showBackground = true)
@Composable
private fun TrustedServiceRuntimeScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            TrustedServiceRuntimeScreen()
        }
    }
}
