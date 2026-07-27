package com.amitia.feature.modelcenter

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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSegmentedTabs
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

private val testTabs = listOf("文本测试", "图片测试", "TTS 试听", "STT 录音", "Embedding")

@Composable
fun ModelTestScreen(
    modelId: String,
    onBack: () -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val modelsState by viewModel.modelsState.collectAsStateWithLifecycle()
    val testResult by viewModel.testResult.collectAsStateWithLifecycle()
    val testing by viewModel.testing.collectAsStateWithLifecycle()
    var selectedTab by remember { mutableIntStateOf(0) }
    var prompt by remember { mutableStateOf("") }

    LaunchedEffect(Unit) { viewModel.loadModels() }

    val modelName = (modelsState as? ScreenState.Content)?.data?.find { it.id == modelId }?.name ?: modelId

    ModelTestContent(
        modelName = modelName,
        selectedTab = selectedTab,
        onTabSelected = { selectedTab = it },
        prompt = prompt,
        onPromptChange = { prompt = it },
        testing = testing,
        testResult = testResult,
        onTest = { viewModel.testModel(modelId, prompt) },
        onBack = onBack
    )
}

@Composable
fun ModelTestContent(
    modelName: String,
    selectedTab: Int,
    onTabSelected: (Int) -> Unit,
    prompt: String,
    onPromptChange: (String) -> Unit,
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit,
    onBack: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "模型测试 - $modelName", onBack = onBack)
        Column(modifier = Modifier.fillMaxSize().padding(AmitiaSpacing.Base)) {
            AmitiaSegmentedTabs(
                tabs = testTabs,
                selectedIndex = selectedTab,
                onSelected = onTabSelected,
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
            when (selectedTab) {
                0 -> TextTestPanel(prompt, onPromptChange, testing, testResult, onTest)
                1 -> ImageTestPanel(testing, testResult, onTest)
                2 -> TtsTestPanel(prompt, onPromptChange, testing, testResult, onTest)
                3 -> SttTestPanel(testing, testResult, onTest)
                else -> EmbeddingTestPanel(prompt, onPromptChange, testing, testResult, onTest)
            }
        }
    }
}

@Composable
private fun TextTestPanel(
    prompt: String,
    onPromptChange: (String) -> Unit,
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        AmitiaSection(title = "输入提示词") {
            AmitiaMultilineField(
                value = prompt,
                onValueChange = onPromptChange,
                placeholder = "输入测试文本...",
                minLines = 3,
                maxLines = 5,
                charLimit = 2000
            )
        }
        LoadingButton(
            text = "发送测试",
            onClick = onTest,
            loading = testing,
            enabled = prompt.isNotBlank(),
            leadingIcon = AmitiaIcons.Send,
            modifier = Modifier.fillMaxWidth()
        )
        testResult?.let { TestResultCard(it) }
    }
}

@Composable
private fun ImageTestPanel(
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        AmitiaSection(title = "图片上传") {
            Surface(
                modifier = Modifier.fillMaxWidth().height(160.dp),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(
                            imageVector = AmitiaIcons.ImageOutlined,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.size(40.dp)
                        )
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        Text(
                            text = "点击上传图片",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
        }
        AmitiaSection(title = "图片描述") {
            AmitiaMultilineField(
                value = "",
                onValueChange = {},
                placeholder = "可选：描述需要识别的内容...",
                minLines = 2,
                maxLines = 3
            )
        }
        LoadingButton(
            text = "测试视觉理解",
            onClick = onTest,
            loading = testing,
            leadingIcon = AmitiaIcons.Visibility,
            modifier = Modifier.fillMaxWidth()
        )
        testResult?.let { TestResultCard(it) }
    }
}

@Composable
private fun TtsTestPanel(
    prompt: String,
    onPromptChange: (String) -> Unit,
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        AmitiaSection(title = "合成文本") {
            AmitiaMultilineField(
                value = prompt,
                onValueChange = onPromptChange,
                placeholder = "输入要合成的文本...",
                minLines = 3,
                maxLines = 5,
                charLimit = 500
            )
        }
        LoadingButton(
            text = "试听",
            onClick = onTest,
            loading = testing,
            enabled = prompt.isNotBlank(),
            leadingIcon = AmitiaIcons.PlayArrow,
            modifier = Modifier.fillMaxWidth()
        )
        testResult?.let { TestResultCard(it) }
    }
}

@Composable
private fun SttTestPanel(
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        AmitiaSection(title = "语音录入") {
            Surface(
                modifier = Modifier.fillMaxWidth().height(160.dp),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Box(
                            modifier = Modifier.size(56.dp).clip(CircleShape)
                                .background(MaterialTheme.colorScheme.primary),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = if (testing) AmitiaIcons.Stop else AmitiaIcons.Mic,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.size(28.dp)
                            )
                        }
                        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                        Text(
                            text = if (testing) "正在录音..." else "点击开始录音",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
        }
        testResult?.let { TestResultCard(it) }
    }
}

@Composable
private fun EmbeddingTestPanel(
    prompt: String,
    onPromptChange: (String) -> Unit,
    testing: Boolean,
    testResult: ModelTestResultUiModel?,
    onTest: () -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        AmitiaSection(title = "写入文本") {
            AmitiaMultilineField(
                value = prompt,
                onValueChange = onPromptChange,
                placeholder = "输入要写入向量库的文本...",
                minLines = 3,
                maxLines = 5,
                charLimit = 2000
            )
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            SecondaryButton(
                text = "写入",
                onClick = onTest,
                leadingIcon = AmitiaIcons.Upload,
                modifier = Modifier.weight(1f)
            )
            PrimaryButton(
                text = "查询",
                onClick = onTest,
                leadingIcon = AmitiaIcons.Search,
                modifier = Modifier.weight(1f)
            )
        }
        testResult?.let { TestResultCard(it) }
    }
}

@Composable
private fun TestResultCard(result: ModelTestResultUiModel) {
    val statusColor = if (result.success) AmitiaStateColors.Running else AmitiaStateColors.Failed
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(modifier = Modifier.size(8.dp).clip(CircleShape).background(statusColor))
                Text(
                    text = if (result.success) "测试成功" else "测试失败",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Spacer(modifier = Modifier.weight(1f))
                result.latencyMs?.let { latency ->
                    Text(
                        text = "${latency}ms",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                result.tokensUsed?.let { tokens ->
                    Text(
                        text = "$tokens tokens",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            result.response?.let { response ->
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                Text(
                    text = response,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
            result.errorMessage?.let { error ->
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                Text(
                    text = error,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error
                )
            }
        }
    }
}

@Preview(name = "ModelTest - Light", showBackground = true)
@Composable
private fun ModelTestLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModelTestContent(
            modelName = "GPT-4o",
            selectedTab = 0,
            onTabSelected = {},
            prompt = "你好",
            onPromptChange = {},
            testing = false,
            testResult = ModelTestResultUiModel(
                success = true,
                response = "你好！有什么可以帮你的吗？",
                latencyMs = 234,
                tokensUsed = 42,
                errorMessage = null,
                timestamp = ""
            ),
            onTest = {},
            onBack = {}
        )
    }
}

@Preview(name = "ModelTest - Dark", showBackground = true)
@Composable
private fun ModelTestDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModelTestContent(
            modelName = "Claude 3.5",
            selectedTab = 3,
            onTabSelected = {},
            prompt = "",
            onPromptChange = {},
            testing = true,
            testResult = null,
            onTest = {},
            onBack = {}
        )
    }
}
