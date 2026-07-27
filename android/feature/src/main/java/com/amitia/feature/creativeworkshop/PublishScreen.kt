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
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun PublishScreen(
    projectId: String,
    onBack: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadPublishInfo(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("发布信息", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = MaterialTheme.shapes.large,
                color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
            ) {
                Row(modifier = Modifier.padding(AmitiaSpacing.Base), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    Icon(AmitiaIcons.Info, contentDescription = null, tint = MaterialTheme.colorScheme.tertiary, modifier = Modifier.size(20.dp))
                    Text(
                        "当前没有在线扩展市场，仅支持本地导出和分享",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onTertiaryContainer
                    )
                }
            }
            when (state.publishInfo) {
                is ScreenState.Loading -> LoadingSkeleton(lineCount = 4)
                is ScreenState.Content -> {
                    val info = (state.publishInfo as ScreenState.Content<CwPublishInfo>).data
                    AmitiaSectionHeader(title = "扩展包信息")
                    Surface(shape = MaterialTheme.shapes.large, color = MaterialTheme.colorScheme.surface) {
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                            InfoRow("包名", info.packageName)
                            InfoRow("描述", info.description)
                            InfoRow("文件大小", "${info.fileSize / 1024} KB")
                            InfoRow("校验和", info.checksum)
                            InfoRow("导出路径", info.exportPath)
                        }
                    }
                    AmitiaSectionHeader(title = "操作")
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        SecondaryButton(text = "导出到本地", onClick = {}, leadingIcon = AmitiaIcons.Download, modifier = Modifier.weight(1f))
                        SecondaryButton(text = "分享文件", onClick = {}, leadingIcon = AmitiaIcons.Share, modifier = Modifier.weight(1f))
                    }
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        SecondaryButton(text = "复制校验和", onClick = {}, leadingIcon = AmitiaIcons.ContentCopy, modifier = Modifier.weight(1f))
                        SecondaryButton(text = "生成说明", onClick = {}, leadingIcon = AmitiaIcons.Book, modifier = Modifier.weight(1f))
                    }
                }
                is ScreenState.Error -> Text("加载失败", color = MaterialTheme.colorScheme.error)
                else -> Unit
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(label, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Medium)
    }
}

@Preview(name = "Publish - Light", showBackground = true)
@Composable
private fun PublishScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PublishScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Publish - Dark", showBackground = true)
@Composable
private fun PublishScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PublishScreen(projectId = "1", onBack = {})
    }
}
