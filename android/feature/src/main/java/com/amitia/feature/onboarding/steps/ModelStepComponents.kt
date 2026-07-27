package com.amitia.feature.onboarding.steps

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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ModelStepScaffold(
    title: String,
    description: String,
    characterName: String,
    onNext: () -> Unit,
    nextEnabled: Boolean,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit
) {
    Surface(
        modifier = modifier.fillMaxSize(),
        color = MaterialTheme.colorScheme.background
    ) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Xxl),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            CharacterAvatar(name = characterName, size = 72)
            StepTitle(text = title)
            StepDescription(text = description)
            content()
            PrimaryButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = nextEnabled,
                leadingIcon = AmitiaIcons.ArrowForward
            )
        }
    }
}

@Composable
fun ModelConfigSection(
    model: ModelSetupState,
    onFieldChange: (String, String) -> Unit,
    onTest: () -> Unit,
    providerLabel: String = "Provider",
    modelLabel: String = "模型",
    apiKeyLabel: String = "API Key",
    testLabel: String = "测试连接",
    capabilitySummary: String? = null,
    icon: ImageVector = AmitiaIcons.SmartToy
) {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
        AmitiaTextField(
            value = model.provider,
            onValueChange = { onFieldChange("provider", it) },
            label = providerLabel,
            placeholder = "请输入 Provider",
            leadingIcon = icon
        )
        AmitiaTextField(
            value = model.model,
            onValueChange = { onFieldChange("model", it) },
            label = modelLabel,
            placeholder = "请输入模型名称"
        )
        AmitiaTextField(
            value = model.apiKey,
            onValueChange = { onFieldChange("apiKey", it) },
            label = apiKeyLabel,
            placeholder = "请输入 API Key",
            leadingIcon = AmitiaIcons.Key
        )
        LoadingButton(
            text = testLabel,
            onClick = onTest,
            modifier = Modifier.fillMaxWidth(),
            enabled = model.provider.isNotBlank() && model.model.isNotBlank() && !model.testing,
            loading = model.testing,
            leadingIcon = AmitiaIcons.Bolt
        )
        ModelTestStatus(model = model, capabilitySummary = capabilitySummary)
    }
}

@Composable
private fun ModelTestStatus(model: ModelSetupState, capabilitySummary: String?) {
    if (model.tested) {
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = AmitiaCardShape,
            color = AmitiaStateColors.Running.copy(alpha = 0.1f)
        ) {
            Row(
                modifier = Modifier.padding(AmitiaSpacing.Base),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier
                        .size(24.dp)
                        .clip(CircleShape)
                        .background(AmitiaStateColors.Running),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.size(16.dp)
                    )
                }
                Column {
                    Text(
                        text = "连接成功",
                        style = MaterialTheme.typography.labelMedium,
                        color = AmitiaStateColors.Running,
                        fontWeight = FontWeight.Medium
                    )
                    if (capabilitySummary != null) {
                        Text(
                            text = capabilitySummary,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
        }
    } else if (model.failed) {
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = AmitiaCardShape,
            color = AmitiaStateColors.Failed.copy(alpha = 0.1f)
        ) {
            Text(
                text = "连接失败，请检查配置后重试",
                style = MaterialTheme.typography.bodySmall,
                color = AmitiaStateColors.Failed,
                modifier = Modifier.padding(AmitiaSpacing.Base)
            )
        }
    }
}
