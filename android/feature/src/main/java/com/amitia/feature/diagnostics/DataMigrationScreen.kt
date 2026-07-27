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

@Composable
fun DataMigrationScreen(
    viewModel: DiagnosticsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { DiagSectionTitle("数据迁移与灰度") }
        when (val ps = state.migrations) {
            is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 2) }
            is ScreenState.Content -> {
                items(ps.data) { migration -> MigrationCard(migration) }
            }
            else -> Unit
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
    }
}

@Composable
private fun MigrationCard(migration: DiagMigration) {
    val hasRollbackIssue = migration.rollbackStatus != null
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
                    text = migration.script,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = migration.version,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            DetailRow("灰度批次", "第 ${migration.batch} 批")
            DetailRow("成功率", migration.successRate)
            DetailRow("回滚点", migration.rollbackPoint)
            if (hasRollbackIssue) {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f)
                ) {
                    Text(
                        text = migration.rollbackStatus!!,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier.padding(AmitiaSpacing.Sm)
                    )
                }
            }
        }
    }
}

@Preview(name = "DataMigration - Light", showBackground = true)
@Composable
private fun DataMigrationScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize()) {
            DataMigrationScreen()
        }
    }
}

@Preview(name = "DataMigration - Dark", showBackground = true)
@Composable
private fun DataMigrationScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize()) {
            DataMigrationScreen()
        }
    }
}
