package com.amitia.feature.creativeworkshop

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
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
import androidx.compose.runtime.LaunchedEffect
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
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaPillShape
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaCodeEditorSurface
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun SchemaUiEditScreen(
    projectId: String,
    onBack: () -> Unit,
    onPreview: (String) -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadSchema(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()
    var codeMode by remember { mutableStateOf(false) }
    var schemaText by remember { mutableStateOf("") }

    LaunchedEffect(state.schemaJson) { schemaText = state.schemaJson }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Schema UI 编辑", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                actions = {
                    AmitiaIconButton(icon = AmitiaIcons.Visibility, contentDescription = "预览", onClick = { onPreview(projectId) })
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        bottomBar = {
            Surface(color = MaterialTheme.colorScheme.background) {
                PrimaryButton(
                    text = "保存 Schema",
                    onClick = { viewModel.updateSchemaJson(schemaText) },
                    leadingIcon = AmitiaIcons.Check,
                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base)
                )
            }
        }
    ) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                ModeTab("可视化", !codeMode) { codeMode = false }
                ModeTab("代码", codeMode) { codeMode = true }
            }
            when (state.schemaNodes) {
                is ScreenState.Loading -> LoadingSkeleton(lineCount = 5)
                is ScreenState.Error -> Text("加载失败", color = MaterialTheme.colorScheme.error)
                else -> {
                    if (codeMode) {
                        AmitiaSectionHeader(title = "Schema 源码")
                        AmitiaCodeEditorSurface(
                            value = schemaText,
                            onValueChange = { schemaText = it },
                            language = "json"
                        )
                    } else {
                        AmitiaSectionHeader(title = "可用组件类型")
                        val types = CwSchemaNodeType.entries
                        LazyRow(horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                            items(types) { type ->
                                SchemaTypeChip(type)
                            }
                        }
                        AmitiaSectionHeader(title = "已添加节点")
                        val nodes = (state.schemaNodes as? ScreenState.Content)?.data ?: emptyList()
                        nodes.forEach { node -> SchemaNodeItem(node) }
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun RowScope.ModeTab(label: String, selected: Boolean, onClick: () -> Unit) {
    Surface(
        modifier = Modifier.weight(1f),
        shape = MaterialTheme.shapes.medium,
        color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.surface,
        onClick = onClick
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelLarge,
            color = if (selected) MaterialTheme.colorScheme.onPrimary else MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(AmitiaSpacing.Base)
        )
    }
}

@Composable
private fun SchemaTypeChip(type: CwSchemaNodeType) {
    Surface(
        shape = AmitiaPillShape,
        color = MaterialTheme.colorScheme.primaryContainer
    ) {
        Row(
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Icon(AmitiaIcons.Add, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(16.dp))
            Text(type.label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onPrimaryContainer)
        }
    }
}

@Composable
private fun SchemaNodeItem(node: CwSchemaNode) {
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
                modifier = Modifier.size(32.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
                contentAlignment = Alignment.Center
            ) {
                Icon(AmitiaIcons.Widgets, contentDescription = null, tint = MaterialTheme.colorScheme.onPrimaryContainer, modifier = Modifier.size(18.dp))
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(node.label, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                Text(node.type.label, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (node.required) {
                Surface(shape = AmitiaPillShape, color = MaterialTheme.colorScheme.errorContainer) {
                    Text("必填", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onErrorContainer, modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs))
                }
            }
        }
    }
}

@Preview(name = "Schema UI Edit - Light", showBackground = true)
@Composable
private fun SchemaUiEditScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SchemaUiEditScreen(projectId = "1", onBack = {}, onPreview = {})
    }
}

@Preview(name = "Schema UI Edit - Dark", showBackground = true)
@Composable
private fun SchemaUiEditScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SchemaUiEditScreen(projectId = "1", onBack = {}, onPreview = {})
    }
}
