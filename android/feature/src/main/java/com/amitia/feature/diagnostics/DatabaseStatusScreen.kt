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
fun DatabaseStatusScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("数据库状态") }
        when (val ps = state.databases) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 3) }
            is ScreenState.Content -> {
                items(ps.data) { db -> DatabaseCard(db) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun DatabaseCard(db: DiagDatabaseStatus) {
    val statusColor = amitiaStatusColor(db.status)
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                Text("${db.type.label} - ${db.name}", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                Text(amitiaStatusText(db.status), style = MaterialTheme.typography.labelMedium, color = statusColor)
            }
            db.migrationVersion?.let { DetailRow("迁移版本", it) }
            DetailRow("连接", db.connection)
            DetailRow("健康检查", db.healthCheck)
            DetailRow("存储大小", db.storageSize)
            db.details.forEach { (key, value) -> DetailRow(key, value) }
        }
    }
}

@Preview(name = "Databases - Light", showBackground = true)
@Composable
private fun DatabaseStatusScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            DatabaseStatusScreen()
        }
    }
}

@Preview(name = "Databases - Dark", showBackground = true)
@Composable
private fun DatabaseStatusScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            DatabaseStatusScreen()
        }
    }
}
