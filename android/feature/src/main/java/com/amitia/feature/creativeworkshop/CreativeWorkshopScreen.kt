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
import com.amitia.core.designsystem.EmptyReason
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun CreativeWorkshopScreen(
    onNewProject: () -> Unit,
    onProjectClick: (String) -> Unit,
    onImportProject: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "创意工坊",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                actions = {
                    AmitiaIconButton(
                        icon = AmitiaIcons.Refresh,
                        contentDescription = "刷新",
                        onClick = viewModel::loadProjectList
                    )
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item {
                QuickActionsRow(
                    onNewProject = onNewProject,
                    onImportProject = onImportProject
                )
            }
            item { AmitiaSectionHeader(title = "模板") }
            if (state.templates.isNotEmpty()) {
                items(state.templates) { template ->
                    AmitiaEntryCard(
                        onClick = onNewProject,
                        leading = {
                            Icon(
                                template.type.icon(),
                                contentDescription = null,
                                modifier = Modifier.size(20.dp)
                            )
                        },
                        title = template.name,
                        subtitle = template.description
                    )
                }
            }
            item { AmitiaSectionHeader(title = "最近编辑") }
            when (val ps = state.projectList) {
                is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 4) }
                is ScreenState.Empty -> item {
                    AmitiaEmptyState(
                        icon = AmitiaIcons.Extension,
                        title = "还没有项目",
                        description = "创建你的第一个扩展项目",
                        reason = EmptyReason.NoData,
                        primaryAction = { PrimaryButton(text = "新建项目", onClick = onNewProject) }
                    )
                }
                is ScreenState.Error -> item {
                    AmitiaErrorState(
                        icon = AmitiaIcons.Error,
                        title = "加载失败",
                        description = ps.error.message,
                        onRetry = viewModel::loadProjectList
                    )
                }
                is ScreenState.Content -> {
                    items(ps.data) { project ->
                        CwProjectCard(project = project, onClick = { onProjectClick(project.id) })
                    }
                }
                is ScreenState.Partial -> {
                    items(ps.data) { project ->
                        CwProjectCard(project = project, onClick = { onProjectClick(project.id) })
                    }
                }
            }
            item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
        }
    }
}

@Composable
private fun QuickActionsRow(
    onNewProject: () -> Unit,
    onImportProject: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        PrimaryButton(
            text = "新建项目",
            onClick = onNewProject,
            leadingIcon = AmitiaIcons.Add,
            modifier = Modifier.weight(1f)
        )
        SecondaryButton(
            text = "导入项目",
            onClick = onImportProject,
            leadingIcon = AmitiaIcons.Download,
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun CwProjectCard(
    project: CwProject,
    onClick: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface,
        onClick = onClick
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Icon(
                imageVector = project.type.icon(),
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(AmitiaIconSize.Large)
            )
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = project.name,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = "${project.type.label} · v${project.version}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = project.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
            Surface(
                shape = MaterialTheme.shapes.small,
                color = MaterialTheme.colorScheme.primaryContainer
            ) {
                Text(
                    text = project.status.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onPrimaryContainer,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
                )
            }
        }
    }
}

internal fun CwProjectType.icon(): ImageVector = when (this) {
    CwProjectType.Skill -> AmitiaIcons.Star
    CwProjectType.Plugin -> AmitiaIcons.Extension
    CwProjectType.McpConfig -> AmitiaIcons.Hub
    CwProjectType.UiContribution -> AmitiaIcons.Widgets
    CwProjectType.Comprehensive -> AmitiaIcons.Layers
}

@Preview(name = "Creative Workshop - Light", showBackground = true)
@Composable
private fun CreativeWorkshopScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CreativeWorkshopScreen(
            onNewProject = {},
            onProjectClick = {},
            onImportProject = {}
        )
    }
}

@Preview(name = "Creative Workshop - Dark", showBackground = true)
@Composable
private fun CreativeWorkshopScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CreativeWorkshopScreen(
            onNewProject = {},
            onProjectClick = {},
            onImportProject = {}
        )
    }
}
