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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun BuildScreen(
    projectId: String,
    onBack: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadBuildConfig(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("构建与打包", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
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
                    text = "开始构建",
                    onClick = { viewModel.startBuild(projectId) },
                    loading = state.isBuilding,
                    leadingIcon = AmitiaIcons.Build,
                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base)
                )
            }
        }
    ) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            when (state.buildConfig) {
                is ScreenState.Loading -> LoadingSkeleton(lineCount = 4)
                is ScreenState.Content -> {
                    val config = (state.buildConfig as ScreenState.Content<CwBuildConfig>).data
                    AmitiaSectionHeader(title = "构建配置")
                    Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.surface) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            ConfigRow("目标平台", config.target)
                            ConfigRow("输出文件名", config.outputName)
                            ConfigRow("输出路径", config.outputPath)
                        }
                    }
                    AmitiaSectionHeader(title = "选项")
                    AmitiaSwitchRow(
                        title = "签名",
                        subtitle = "使用证书对扩展包进行签名",
                        checked = config.signEnabled,
                        onCheckedChange = {},
                        leadingIcon = AmitiaIcons.VerifiedUser
                    )
                    AmitiaSwitchRow(
                        title = "压缩优化",
                        subtitle = "压缩和混淆代码以减小包体积",
                        checked = config.minify,
                        onCheckedChange = {},
                        leadingIcon = AmitiaIcons.Archive
                    )
                }
                else -> Unit
            }
            state.buildResult?.let { result ->
                AmitiaSectionHeader(title = "构建结果")
                BuildResultCard(result)
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun ConfigRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Medium)
    }
}

@Composable
private fun BuildResultCard(result: CwBuildResult) {
    val statusColor = if (result.success) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.error
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Icon(
                    if (result.success) AmitiaIcons.CheckCircle else AmitiaIcons.Error,
                    contentDescription = null,
                    tint = statusColor,
                    modifier = Modifier.size(24.dp)
                )
                Text(
                    if (result.success) "构建成功" else "构建失败",
                    style = MaterialTheme.typography.titleMedium,
                    color = statusColor
                )
            }
            result.steps.forEach { step ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    Icon(
                        when (step.status) {
                            CwBuildStepStatus.Success -> AmitiaIcons.CheckCircle
                            CwBuildStepStatus.Failed -> AmitiaIcons.Error
                            CwBuildStepStatus.Running -> AmitiaIcons.Sync
                            else -> AmitiaIcons.AccessTime
                        },
                        contentDescription = null,
                        tint = when (step.status) {
                            CwBuildStepStatus.Success -> MaterialTheme.colorScheme.tertiary
                            CwBuildStepStatus.Failed -> MaterialTheme.colorScheme.error
                            else -> MaterialTheme.colorScheme.onSurfaceVariant
                        },
                        modifier = Modifier.size(18.dp)
                    )
                    Text(step.name, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurface, modifier = Modifier.weight(1f))
                    Text(step.status.label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            if (result.warnings.isNotEmpty()) {
                Surface(shape = MaterialTheme.shapes.medium, color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        result.warnings.forEach { warning ->
                            Text("警告: $warning", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onTertiaryContainer)
                        }
                    }
                }
            }
            ConfigRow("耗时", "${result.duration}ms")
            ConfigRow("输出大小", "${result.outputSize / 1024} KB")
        }
    }
}

@Preview(name = "Build - Light", showBackground = true)
@Composable
private fun BuildScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        BuildScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Build - Dark", showBackground = true)
@Composable
private fun BuildScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        BuildScreen(projectId = "1", onBack = {})
    }
}
