package com.amitia.feature.modelcenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSegmentedTabs
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.LogRow
import com.amitia.core.designsystem.component.AmitiaLogLevel
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun ModelDetailScreen(
    onBack: () -> Unit,
    modelId: String,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val saving by viewModel.saving.collectAsStateWithLifecycle()
    ModelDetailContent(
        modelId = modelId,
        saving = saving,
        onBack = onBack,
        onSave = {}
    )
}

@Composable
fun ModelDetailContent(
    modelId: String,
    saving: Boolean,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    var name by remember { mutableStateOf("GPT-4o") }
    var provider by remember { mutableStateOf("OpenAI") }
    var endpoint by remember { mutableStateOf("https://api.openai.com/v1") }
    var temperature by remember { mutableStateOf(0.7f) }
    var maxTokens by remember { mutableStateOf("4096") }
    var contextWindow by remember { mutableStateOf("128000") }
    var enabled by remember { mutableStateOf(true) }
    var streaming by remember { mutableStateOf(true) }
    var toolCalling by remember { mutableStateOf(true) }
    var selectedTab by remember { mutableStateOf(0) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "模型详情", onBack = onBack)
        AmitiaSegmentedTabs(
            tabs = listOf("基本信息", "参数", "能力", "角色绑定", "测试", "日志"),
            selectedIndex = selectedTab,
            onSelected = { selectedTab = it },
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            when (selectedTab) {
                0 -> {
                    AmitiaSection(title = "基本信息") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                            AmitiaTextField(value = name, onValueChange = { name = it }, label = "模型名称")
                            AmitiaTextField(value = provider, onValueChange = { provider = it }, label = "Provider")
                            AmitiaTextField(value = endpoint, onValueChange = { endpoint = it }, label = "Endpoint")
                            AmitiaTextField(value = contextWindow, onValueChange = { contextWindow = it }, label = "上下文长度")
                            AmitiaSwitchRow(title = "启用模型", checked = enabled, onCheckedChange = { enabled = it })
                        }
                    }
                }
                1 -> {
                    AmitiaSection(title = "生成参数") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                            AmitiaSlider(
                                value = temperature,
                                onValueChange = { temperature = it },
                                valueRange = 0f..2f,
                                label = "Temperature",
                                valueFormatter = { String.format("%.2f", it) }
                            )
                            AmitiaNumberField(value = maxTokens, onValueChange = { maxTokens = it }, label = "最大输出 Token")
                        }
                    }
                }
                2 -> {
                    AmitiaSection(title = "模型能力") {
                        Column {
                            AmitiaSwitchRow(title = "流式输出", checked = streaming, onCheckedChange = { streaming = it })
                            AmitiaSwitchRow(title = "工具调用", checked = toolCalling, onCheckedChange = { toolCalling = it })
                        }
                    }
                }
                3 -> {
                    AmitiaSection(title = "角色绑定") {
                        Column {
                            Text("当前绑定角色：艾米", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurface)
                            Text("该模型将被艾米用于对话生成", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
                4 -> {
                    AmitiaSection(title = "快速测试") {
                        Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                            Text("请在模型测试页面进行完整测试", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            SecondaryButton(text = "前往测试页", onClick = {}, modifier = Modifier.fillMaxWidth(), leadingIcon = AmitiaIcons.Science)
                        }
                    }
                }
                5 -> {
                    AmitiaSection(title = "错误与日志") {
                        Column {
                            LogRow(message = "模型调用成功", timestamp = "14:30:00", level = AmitiaLogLevel.Info, source = "Model-$modelId")
                            LogRow(message = "Token 使用: 1,234", timestamp = "14:29:55", level = AmitiaLogLevel.Debug, source = "Model-$modelId")
                            LogRow(message = "接近速率限制", timestamp = "14:28:30", level = AmitiaLogLevel.Warning, source = "Provider")
                        }
                    }
                }
            }
            LoadingButton(
                text = "保存修改",
                onClick = onSave,
                loading = saving,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Preview(name = "ModelDetail - Light", showBackground = true)
@Composable
private fun ModelDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModelDetailContent(modelId = "1", saving = false, onBack = {}, onSave = {})
    }
}

@Preview(name = "ModelDetail - Dark", showBackground = true)
@Composable
private fun ModelDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModelDetailContent(modelId = "1", saving = false, onBack = {}, onSave = {})
    }
}
