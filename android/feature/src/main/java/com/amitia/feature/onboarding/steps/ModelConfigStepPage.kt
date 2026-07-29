package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons

private val modelOptions = listOf("默认模型", "高性能模型", "轻量模型")

@Composable
fun ModelConfigStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String, String) -> Unit,
    onTest: (String) -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    var openCapability by remember { mutableStateOf<String?>(null) }
    val textValue = remember { mutableStateOf("默认模型") }
    val visionValue = remember { mutableStateOf("默认模型") }
    val voiceValue = remember { mutableStateOf("默认模型") }
    val vectorValue = remember { mutableStateOf("默认模型") }

    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.ModelConfig)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "4 / 6")
                OnboardingTitle(text = "连接模型能力")
                OnboardingDescription(text = "逐项选择默认模型，交互保持轻量且不切断背景。")
            }

            Spacer(modifier = Modifier.height(20.dp))

            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                SoftRow(
                    title = "文本模型",
                    description = "对话、推理与工具规划",
                    icon = AmitiaIcons.Chat,
                    valueText = textValue.value,
                    onClick = { openCapability = "文本" }
                )
                SoftRow(
                    title = "视觉模型",
                    description = "图片、截图和界面理解",
                    icon = AmitiaIcons.Image,
                    valueText = visionValue.value,
                    onClick = { openCapability = "视觉" }
                )
                SoftRow(
                    title = "语音模型",
                    description = "语音识别、生成和通话",
                    icon = AmitiaIcons.Mic,
                    valueText = voiceValue.value,
                    onClick = { openCapability = "语音" }
                )
                SoftRow(
                    title = "向量模型",
                    description = "记忆检索和语义召回",
                    icon = AmitiaIcons.Memory,
                    valueText = vectorValue.value,
                    onClick = { openCapability = "向量" }
                )
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "继续",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth()
            )
        }

        val currentCap = openCapability
        if (currentCap != null) {
            val selectedValue = when (currentCap) {
                "文本" -> textValue.value
                "视觉" -> visionValue.value
                "语音" -> voiceValue.value
                "向量" -> vectorValue.value
                else -> "默认模型"
            }
            ModelSelectorOverlay(
                title = "选择${currentCap}模型",
                options = modelOptions,
                selectedValue = selectedValue,
                onSelect = { value ->
                    when (currentCap) {
                        "文本" -> textValue.value = value
                        "视觉" -> visionValue.value = value
                        "语音" -> voiceValue.value = value
                        "向量" -> vectorValue.value = value
                    }
                    openCapability = null
                },
                onDismiss = { openCapability = null },
                visible = true
            )
        }
    }
}
