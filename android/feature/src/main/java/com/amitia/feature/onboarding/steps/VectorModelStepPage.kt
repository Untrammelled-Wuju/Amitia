package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun VectorModelStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String) -> Unit,
    onDimensionChange: (String) -> Unit,
    onTestModel: () -> Unit,
    onTestQdrant: () -> Unit,
    onTestVectorWrite: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
) {
    ModelStepScaffold(
        title = "向量模型设置",
        description = "配置 Embedding 模型与 Qdrant 连接，用于长期记忆检索。",
        characterName = state.character.name.ifBlank { "Amitia" },
        onNext = onNext,
        nextEnabled = state.vectorModel.tested && state.vectorQdrantConnected,
        modifier = modifier
    ) {
        ModelConfigSection(
            model = state.vectorModel,
            onFieldChange = { field, value -> onFieldChange(field, value) },
            onTest = onTestModel,
            providerLabel = "Embedding Provider",
            modelLabel = "Embedding 模型",
            apiKeyLabel = "API Key",
            testLabel = "测试 Embedding",
            capabilitySummary = "向量维度 1536 · 支持中英文",
            icon = AmitiaIcons.Layers
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        VectorDimensionSection(
            dimension = state.vectorDimension,
            onChange = onDimensionChange
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        QdrantConnectionSection(
            connected = state.vectorQdrantConnected,
            onTest = onTestQdrant
        )
        Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        VectorWriteTestSection(
            modelTested = state.vectorModel.tested,
            qdrantConnected = state.vectorQdrantConnected,
            onTest = onTestVectorWrite
        )
    }
}

@Composable
private fun VectorDimensionSection(
    dimension: String,
    onChange: (String) -> Unit
) {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "向量维度",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        AmitiaTextField(
            value = dimension,
            onValueChange = onChange,
            label = "维度",
            placeholder = "如 1536",
            leadingIcon = AmitiaIcons.Tune
        )
        Text(
            text = "维度需与所选 Embedding 模型输出一致，常见为 768 / 1024 / 1536。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun QdrantConnectionSection(
    connected: Boolean,
    onTest: () -> Unit
) {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "Qdrant 连接状态",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = AmitiaCardShape,
            color = if (connected) AmitiaStateColors.Running.copy(alpha = 0.1f)
            else MaterialTheme.colorScheme.surface
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(AmitiaSpacing.Base),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(24.dp)
                        .clip(CircleShape)
                        .background(if (connected) AmitiaStateColors.Running else MaterialTheme.colorScheme.outline),
                    contentAlignment = Alignment.Center
                ) {
                    if (connected) {
                        Icon(
                            imageVector = AmitiaIcons.Check,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.size(16.dp)
                        )
                    }
                }
                Text(
                    text = if (connected) "Qdrant 已连接" else "Qdrant 未连接",
                    style = MaterialTheme.typography.bodyMedium,
                    color = if (connected) AmitiaStateColors.Running
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f)
                )
            }
        }
        LoadingButton(
            text = "测试 Qdrant 连接",
            onClick = onTest,
            modifier = Modifier.fillMaxWidth(),
            enabled = !connected,
            loading = false,
            leadingIcon = AmitiaIcons.Router
        )
    }
}

@Composable
private fun VectorWriteTestSection(
    modelTested: Boolean,
    qdrantConnected: Boolean,
    onTest: () -> Unit
) {
    val enabled = modelTested && qdrantConnected
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "向量写入与查询测试",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        PrimaryButton(
            text = "写入测试向量并查询",
            onClick = onTest,
            modifier = Modifier.fillMaxWidth(),
            enabled = enabled,
            leadingIcon = AmitiaIcons.Bolt
        )
        if (!enabled) {
            Text(
                text = "请先完成 Embedding 测试和 Qdrant 连接。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Preview(name = "VectorModel - Light", showBackground = true)
@Composable
private fun VectorModelStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VectorModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米"),
                vectorModel = ModelSetupState(provider = "OpenAI", model = "text-embedding-3-small", tested = true),
                vectorDimension = "1536",
                vectorQdrantConnected = true
            ),
            onFieldChange = { _, _ -> },
            onDimensionChange = {},
            onTestModel = {},
            onTestQdrant = {},
            onTestVectorWrite = {},
            onNext = {}
        )
    }
}

@Preview(name = "VectorModel - Dark", showBackground = true)
@Composable
private fun VectorModelStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VectorModelStepPage(
            state = OnboardingFlowUiState(
                character = CharacterSetupState(name = "艾米")
            ),
            onFieldChange = { _, _ -> },
            onDimensionChange = {},
            onTestModel = {},
            onTestQdrant = {},
            onTestVectorWrite = {},
            onNext = {}
        )
    }
}
