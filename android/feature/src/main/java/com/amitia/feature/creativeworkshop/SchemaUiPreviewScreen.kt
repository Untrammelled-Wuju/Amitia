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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.SecondaryButton

private enum class PreviewMode { Light, Dark, Phone, Tablet, Loading, Error, Empty }

@Composable
fun SchemaUiPreviewScreen(
    projectId: String,
    onBack: () -> Unit
) {
    var mode by remember { mutableStateOf(PreviewMode.Light) }
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Schema UI 预览", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
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
            AmitiaSectionHeader(title = "预览模式")
            LazyRow(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                items(PreviewMode.entries.toList()) { p ->
                    ModeChip(p.label, mode == p) { mode = p }
                }
            }
            AmitiaSectionHeader(title = "主题一致性检查")
            Row(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                CheckChip("颜色一致", true)
                CheckChip("字体一致", true)
                CheckChip("间距一致", true)
                CheckChip("圆角一致", false)
            }
            AmitiaSectionHeader(title = "预览区域")
            PreviewArea(mode)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

private val PreviewMode.label: String
    get() = when (this) {
        PreviewMode.Light -> "亮色"
        PreviewMode.Dark -> "暗色"
        PreviewMode.Phone -> "手机"
        PreviewMode.Tablet -> "平板"
        PreviewMode.Loading -> "加载态"
        PreviewMode.Error -> "错误态"
        PreviewMode.Empty -> "空态"
    }

@Composable
private fun ModeChip(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        shape = AmitiaPillShape,
        color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surfaceVariant,
        onClick = onClick
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
        )
    }
}

@Composable
private fun CheckChip(label: String, passed: Boolean) {
    Surface(shape = AmitiaPillShape, color = MaterialTheme.colorScheme.surfaceVariant) {
        Row(modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Icon(
                if (passed) AmitiaIcons.CheckCircle else AmitiaIcons.Warning,
                contentDescription = null,
                tint = if (passed) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.error,
                modifier = Modifier.size(14.dp)
            )
            Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun PreviewArea(mode: PreviewMode) {
    val isDark = mode == PreviewMode.Dark
    val isTablet = mode == PreviewMode.Tablet
    val bgColor = if (isDark) MaterialTheme.colorScheme.surfaceVariant else MaterialTheme.colorScheme.surface
    val modifier = if (isTablet) Modifier.fillMaxWidth().height(400.dp) else Modifier.fillMaxWidth().height(300.dp)
    Surface(
        modifier = modifier,
        shape = MaterialTheme.shapes.large,
        color = bgColor,
        tonalElevation = 2.dp
    ) {
        when (mode) {
            PreviewMode.Loading -> Box(contentAlignment = Alignment.Center, modifier = Modifier.fillMaxSize()) {
                Text("加载中...", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            PreviewMode.Error -> Box(contentAlignment = Alignment.Center, modifier = Modifier.fillMaxSize()) {
                Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    Icon(AmitiaIcons.Error, contentDescription = null, tint = MaterialTheme.colorScheme.error, modifier = Modifier.size(48.dp))
                    Text("预览渲染失败", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
                }
            }
            PreviewMode.Empty -> Box(contentAlignment = Alignment.Center, modifier = Modifier.fillMaxSize()) {
                Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    Icon(AmitiaIcons.Widgets, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(48.dp))
                    Text("暂无内容", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            else -> Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Text("表单预览", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.onSurface)
                Surface(shape = MaterialTheme.shapes.medium, color = MaterialTheme.colorScheme.background) {
                    Text("输入框示例", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(AmitiaSpacing.Base))
                }
                Surface(shape = MaterialTheme.shapes.medium, color = MaterialTheme.colorScheme.background) {
                    Text("按钮组示例", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(AmitiaSpacing.Base))
                }
            }
        }
    }
}

@Preview(name = "Schema UI Preview - Light", showBackground = true)
@Composable
private fun SchemaUiPreviewScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SchemaUiPreviewScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Schema UI Preview - Dark", showBackground = true)
@Composable
private fun SchemaUiPreviewScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SchemaUiPreviewScreen(projectId = "1", onBack = {})
    }
}
