package com.amitia.feature.creativeworkshop

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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun ProjectTestScreen(
    projectId: String,
    onBack: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadTestComponents(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("项目运行测试", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        bottomBar = {
            Surface(color = MaterialTheme.colorScheme.background) {
                LoadingButton(
                    text = "运行测试",
                    onClick = { viewModel.runTests(projectId) },
                    loading = state.isTesting,
                    leadingIcon = AmitiaIcons.PlayArrow,
                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base)
                )
            }
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding).padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.large,
                    color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                ) {
                    Row(modifier = Modifier.padding(AmitiaSpacing.Base), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        Icon(AmitiaIcons.Science, contentDescription = null, tint = MaterialTheme.colorScheme.tertiary, modifier = Modifier.size(20.dp))
                        Text(
                            "隔离测试扩展的工具、事件、Hook、Schedule 和 UI Contribution",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer
                        )
                    }
                }
            }
            when (val ps = state.testComponents) {
                is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 5) }
                is ScreenState.Content -> {
                    val passed = ps.data.count { it.status == CwTestStatus.Passed }
                    val failed = ps.data.count { it.status == CwTestStatus.Failed }
                    item {
                        AmitiaSectionHeader(title = "测试结果 (${passed} 通过 / ${failed} 失败 / ${ps.data.size} 总计)")
                    }
                    items(ps.data) { component ->
                        TestComponentCard(component)
                    }
                }
                is ScreenState.Error -> item { Text("加载失败", color = MaterialTheme.colorScheme.error) }
                else -> Unit
            }
            item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
        }
    }
}

@Composable
private fun TestComponentCard(component: CwTestComponent) {
    val statusColor = when (component.status) {
        CwTestStatus.Passed -> MaterialTheme.colorScheme.tertiary
        CwTestStatus.Failed -> MaterialTheme.colorScheme.error
        CwTestStatus.Running -> MaterialTheme.colorScheme.primary
        CwTestStatus.Skipped -> MaterialTheme.colorScheme.onSurfaceVariant
        CwTestStatus.Pending -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier.size(32.dp).clip(CircleShape).background(statusColor.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    when (component.status) {
                        CwTestStatus.Passed -> AmitiaIcons.CheckCircle
                        CwTestStatus.Failed -> AmitiaIcons.Error
                        CwTestStatus.Running -> AmitiaIcons.Sync
                        else -> AmitiaIcons.AccessTime
                    },
                    contentDescription = null,
                    tint = statusColor,
                    modifier = Modifier.size(18.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(component.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                Text(component.type, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                component.message?.let {
                    Text(it, style = MaterialTheme.typography.bodySmall, color = statusColor)
                }
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(component.status.label, style = MaterialTheme.typography.labelMedium, color = statusColor)
                if (component.duration > 0) {
                    Text("${component.duration}ms", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}

@Preview(name = "Project Test - Light", showBackground = true)
@Composable
private fun ProjectTestScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ProjectTestScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Project Test - Dark", showBackground = true)
@Composable
private fun ProjectTestScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ProjectTestScreen(projectId = "1", onBack = {})
    }
}
