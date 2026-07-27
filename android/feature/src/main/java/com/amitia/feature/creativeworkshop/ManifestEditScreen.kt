package com.amitia.feature.creativeworkshop

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaCodeEditorSurface
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun ManifestEditScreen(
    projectId: String,
    onBack: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadManifest(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()
    var advancedMode by remember { mutableStateOf(false) }
    var name by remember { mutableStateOf("") }
    var version by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var author by remember { mutableStateOf("") }
    var entryPoint by remember { mutableStateOf("") }
    var minRuntime by remember { mutableStateOf("") }

    LaunchedEffect(state.manifest) {
        if (state.manifest is ScreenState.Content) {
            val m = (state.manifest as ScreenState.Content<CwManifest>).data
            name = m.name; version = m.version; description = m.description
            author = m.author; entryPoint = m.entryPoint; minRuntime = m.minRuntimeVersion
        }
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Manifest 编辑", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                actions = {
                    TertiaryButton(
                        text = if (advancedMode) "表单模式" else "源码模式",
                        onClick = { advancedMode = !advancedMode },
                        leadingIcon = if (advancedMode) AmitiaIcons.Edit else AmitiaIcons.Code
                    )
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
                    text = "保存",
                    onClick = {
                        viewModel.updateManifest(
                            CwManifest(name, version, description, author, entryPoint, listOf("core >= 1.0.0"), minRuntime)
                        )
                    },
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
            when (state.manifest) {
                is ScreenState.Loading -> LoadingSkeleton(lineCount = 6)
                is ScreenState.Error -> Text(
                    "加载失败",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error
                )
                else -> {
                    if (advancedMode) {
                        AmitiaSectionHeader(title = "源码视图")
                        AmitiaCodeEditorSurface(
                            value = state.manifestJson,
                            onValueChange = {},
                            language = "json"
                        )
                    } else {
                        AmitiaSectionHeader(title = "基本信息")
                        AmitiaTextField(value = name, onValueChange = { name = it }, label = "名称", leadingIcon = AmitiaIcons.Edit)
                        AmitiaTextField(value = version, onValueChange = { version = it }, label = "版本", placeholder = "1.0.0", leadingIcon = AmitiaIcons.Star)
                        AmitiaTextField(value = author, onValueChange = { author = it }, label = "作者", leadingIcon = AmitiaIcons.Person)
                        AmitiaMultilineField(value = description, onValueChange = { description = it }, label = "描述", charLimit = 300)
                        AmitiaSectionHeader(title = "入口配置")
                        AmitiaTextField(value = entryPoint, onValueChange = { entryPoint = it }, label = "入口文件", placeholder = "index.js", leadingIcon = AmitiaIcons.Code)
                        AmitiaTextField(value = minRuntime, onValueChange = { minRuntime = it }, label = "最低运行时版本", placeholder = "1.0.0", leadingIcon = AmitiaIcons.VerifiedUser)
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "Manifest Edit - Light", showBackground = true)
@Composable
private fun ManifestEditScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ManifestEditScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Manifest Edit - Dark", showBackground = true)
@Composable
private fun ManifestEditScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ManifestEditScreen(projectId = "1", onBack = {})
    }
}
