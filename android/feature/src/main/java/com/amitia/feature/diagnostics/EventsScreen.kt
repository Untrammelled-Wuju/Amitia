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
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun EventsScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("第三方 Event") }
        when (val ps = state.events) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 3) }
            is ScreenState.Content -> {
                items(ps.data) { event -> EventCard(event) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun EventCard(event: DiagEvent) {
    val failureColor = if (event.failures > 0) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.tertiary
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text(event.type, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                    Box(modifier = Modifier.size(8.dp).background(failureColor, CircleShape))
                    Text("失败 ${event.failures}", style = MaterialTheme.typography.labelMedium, color = failureColor)
                }
            }
            DetailRow("发布者", event.publisher)
            DetailRow("最近触发", event.lastTrigger)
            Text("订阅者", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            event.subscribers.forEach { sub ->
                Surface(shape = AmitiaPillShape, color = MaterialTheme.colorScheme.surfaceVariant) {
                    Text(sub, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp))
                }
            }
        }
    }
}

@Preview(name = "Events - Light", showBackground = true)
@Composable
private fun EventsScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            EventsScreen()
        }
    }
}

@Preview(name = "Events - Dark", showBackground = true)
@Composable
private fun EventsScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            EventsScreen()
        }
    }
}
