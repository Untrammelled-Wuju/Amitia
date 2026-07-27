package com.amitia.feature.creativeworkshop

import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.SettingsRow

@Composable
fun ProjectDetailScreen(
    projectId: String,
    onBack: () -> Unit,
    onEditManifest: (String) -> Unit,
    onEditSchema: (String) -> Unit,
    onPermission: (String) -> Unit,
    onBuild: (String) -> Unit,
    onTest: (String) -> Unit,
    onPublish: (String) -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadProjectDetail(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("项目详情", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        when (val ps = state.projectDetail) {
            is ScreenState.Loading -> {
                Column(modifier = Modifier.fillMaxSize().padding(padding)) {
                    LoadingSkeleton(lineCount = 6)
                }
            }
            is ScreenState.Error -> {
                Column(modifier = Modifier.fillMaxSize().padding(padding)) {
                    AmitiaErrorState(
                        icon = AmitiaIcons.Error,
                        title = ps.error.title,
                        description = ps.error.message,
                        onRetry = { viewModel.loadProjectDetail(projectId) }
                    )
                }
            }
            is ScreenState.Content -> {
                val (project, files) = ps.data
                LazyColumn(
                    modifier = Modifier.fillMaxSize().padding(padding).padding(horizontal = AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    item { ProjectInfoCard(project) }
                    item { AmitiaSectionHeader(title = "文件结构") }
                    items(files) { file -> FileNodeItem(file) }
                    item { AmitiaSectionHeader(title = "操作") }
                    item {
                        SettingsRow(
                            title = "Manifest 编辑",
                            subtitle = "编辑项目清单文件",
                            leadingIcon = AmitiaIcons.Code,
                            onClick = { onEditManifest(projectId) }
                        )
                    }
                    item {
                        SettingsRow(
                            title = "Schema UI 编辑",
                            subtitle = "编辑界面架构",
                            leadingIcon = AmitiaIcons.Schema,
                            onClick = { onEditSchema(projectId) }
                        )
                    }
                    item {
                        SettingsRow(
                            title = "权限声明",
                            subtitle = "配置权限和风险说明",
                            leadingIcon = AmitiaIcons.Security,
                            onClick = { onPermission(projectId) }
                        )
                    }
                    item {
                        SettingsRow(
                            title = "构建与打包",
                            subtitle = "校验、构建和签名",
                            leadingIcon = AmitiaIcons.Build,
                            onClick = { onBuild(projectId) }
                        )
                    }
                    item {
                        SettingsRow(
                            title = "运行测试",
                            subtitle = "隔离测试扩展功能",
                            leadingIcon = AmitiaIcons.Science,
                            onClick = { onTest(projectId) }
                        )
                    }
                    item {
                        SettingsRow(
                            title = "发布信息",
                            subtitle = "导出和分享扩展包",
                            leadingIcon = AmitiaIcons.Publish,
                            onClick = { onPublish(projectId) }
                        )
                    }
                    item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
                }
            }
            else -> Unit
        }
    }
}

@Composable
private fun ProjectInfoCard(project: CwProject) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)) {
                Icon(
                    imageVector = project.type.icon(),
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(AmitiaIconSize.Large)
                )
                Column(modifier = Modifier.weight(1f)) {
                    Text(project.name, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurface)
                    Text("${project.type.label} · v${project.version}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.primaryContainer) {
                    Text(
                        project.status.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
                    )
                }
            }
            Text(project.description, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 3, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun FileNodeItem(file: CwFileNode) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Xs),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        Icon(
            imageVector = if (file.isDirectory) AmitiaIcons.Folder else AmitiaIcons.FileCopy,
            contentDescription = null,
            tint = if (file.isDirectory) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.size(AmitiaIconSize.Medium)
        )
        Text(
            text = file.name,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            modifier = Modifier.weight(1f)
        )
        if (!file.isDirectory && file.size > 0) {
            Text(formatFileSize(file.size), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

private fun formatFileSize(bytes: Long): String {
    return when {
        bytes < 1024 -> "$bytes B"
        bytes < 1024 * 1024 -> "${bytes / 1024} KB"
        else -> "${bytes / (1024 * 1024)} MB"
    }
}

private val AmitiaIcons.Publish: ImageVector
    get() = AmitiaIcons.Upload

@Preview(name = "Project Detail - Light", showBackground = true)
@Composable
private fun ProjectDetailScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ProjectDetailScreen(
            projectId = "1",
            onBack = {},
            onEditManifest = {},
            onEditSchema = {},
            onPermission = {},
            onBuild = {},
            onTest = {},
            onPublish = {}
        )
    }
}

@Preview(name = "Project Detail - Dark", showBackground = true)
@Composable
private fun ProjectDetailScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ProjectDetailScreen(
            projectId = "1",
            onBack = {},
            onEditManifest = {},
            onEditSchema = {},
            onPermission = {},
            onBuild = {},
            onTest = {},
            onPublish = {}
        )
    }
}
