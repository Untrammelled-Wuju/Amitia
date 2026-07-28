package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.amitia.core.designsystem.LocalIsDarkTheme

@Composable
private fun stepTextColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xFFF2F4F2) else Color(0xFF171A18)

@Composable
private fun stepMutedColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xFFA4AAA6) else Color(0xFF6A706B)

@Composable
private fun stepSuccessColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xFF7FB28E) else Color(0xFF5E836F)

@Composable
private fun stepErrorColor(): Color =
    if (LocalIsDarkTheme.current) Color(0xFFD49292) else Color(0xFFC06A5A)

@Composable
fun ModelConfigStepPage(
    state: OnboardingFlowUiState,
    onFieldChange: (String, String, String) -> Unit,
    onTest: (String) -> Unit,
    onNext: () -> Unit,
    onBack: () -> Unit,
    modifier: Modifier = Modifier
) {
    Box(modifier = modifier.fillMaxSize()) {
        StepContentScroll(
            topPadding = contentTopPaddingForStep(OnboardingFlowStep.ModelConfig)
        ) {
            RevealContent(delayMs = 0) {
                StepLabel(text = "4 / 6")
                OnboardingTitle(text = "连接模型能力")
                OnboardingDescription(text = "为 Amitia 配置 AI 模型，至少需要文本模型。")
            }

            Spacer(modifier = Modifier.height(20.dp))

            GlassCard(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "文本模型",
                        color = stepTextColor(),
                        fontSize = 14.sp,
                        fontWeight = FontWeight(620)
                    )
                    SoftField(
                        label = "Provider",
                        value = state.textModel.provider,
                        onValueChange = { onFieldChange("text", "provider", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "模型",
                        value = state.textModel.model,
                        onValueChange = { onFieldChange("text", "model", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "API Key",
                        value = state.textModel.apiKey,
                        onValueChange = { onFieldChange("text", "apiKey", it) },
                        modifier = Modifier.fillMaxWidth(),
                        isPassword = true
                    )
                    PrimaryGlassButton(
                        text = "测试连接",
                        onClick = { onTest("text") },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.textModel.testing
                    )
                    ModelTestStatus(model = state.textModel)
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            GlassCard(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "视觉模型",
                        color = stepTextColor(),
                        fontSize = 14.sp,
                        fontWeight = FontWeight(620)
                    )
                    SoftField(
                        label = "Provider",
                        value = state.visionModel.provider,
                        onValueChange = { onFieldChange("vision", "provider", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "模型",
                        value = state.visionModel.model,
                        onValueChange = { onFieldChange("vision", "model", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "API Key",
                        value = state.visionModel.apiKey,
                        onValueChange = { onFieldChange("vision", "apiKey", it) },
                        modifier = Modifier.fillMaxWidth(),
                        isPassword = true
                    )
                    PrimaryGlassButton(
                        text = "测试连接",
                        onClick = { onTest("vision") },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.visionModel.testing
                    )
                    ModelTestStatus(model = state.visionModel)
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            GlassCard(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "语音模型",
                        color = stepTextColor(),
                        fontSize = 14.sp,
                        fontWeight = FontWeight(620)
                    )
                    SoftField(
                        label = "TTS Provider",
                        value = state.voiceTts.provider,
                        onValueChange = { onFieldChange("tts", "provider", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "TTS 模型",
                        value = state.voiceTts.model,
                        onValueChange = { onFieldChange("tts", "model", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "TTS Key",
                        value = state.voiceTts.apiKey,
                        onValueChange = { onFieldChange("tts", "apiKey", it) },
                        modifier = Modifier.fillMaxWidth(),
                        isPassword = true
                    )
                    PrimaryGlassButton(
                        text = "测试 TTS",
                        onClick = { onTest("tts") },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.voiceTts.testing
                    )
                    ModelTestStatus(model = state.voiceTts)

                    Spacer(modifier = Modifier.height(4.dp))

                    SoftField(
                        label = "STT Provider",
                        value = state.voiceStt.provider,
                        onValueChange = { onFieldChange("stt", "provider", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "STT 模型",
                        value = state.voiceStt.model,
                        onValueChange = { onFieldChange("stt", "model", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "STT Key",
                        value = state.voiceStt.apiKey,
                        onValueChange = { onFieldChange("stt", "apiKey", it) },
                        modifier = Modifier.fillMaxWidth(),
                        isPassword = true
                    )
                    PrimaryGlassButton(
                        text = "测试 STT",
                        onClick = { onTest("stt") },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.voiceStt.testing
                    )
                    ModelTestStatus(model = state.voiceStt)
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            GlassCard(modifier = Modifier.fillMaxWidth()) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "向量模型",
                        color = stepTextColor(),
                        fontSize = 14.sp,
                        fontWeight = FontWeight(620)
                    )
                    SoftField(
                        label = "Provider",
                        value = state.vectorModel.provider,
                        onValueChange = { onFieldChange("vector", "provider", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "模型",
                        value = state.vectorModel.model,
                        onValueChange = { onFieldChange("vector", "model", it) },
                        modifier = Modifier.fillMaxWidth()
                    )
                    SoftField(
                        label = "API Key",
                        value = state.vectorModel.apiKey,
                        onValueChange = { onFieldChange("vector", "apiKey", it) },
                        modifier = Modifier.fillMaxWidth(),
                        isPassword = true
                    )
                    PrimaryGlassButton(
                        text = "测试连接",
                        onClick = { onTest("vector") },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.vectorModel.testing
                    )
                    ModelTestStatus(model = state.vectorModel)
                }
            }
        }

        BottomActionContainer(
            modifier = Modifier.align(Alignment.BottomCenter)
        ) {
            PrimaryGlassButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = state.textModel.tested
            )
        }
    }
}

@Composable
private fun ModelTestStatus(model: ModelSetupState) {
    when {
        model.tested -> Text(
            text = "连接成功",
            color = stepSuccessColor(),
            fontSize = 12.sp
        )
        model.failed -> Text(
            text = "连接失败",
            color = stepErrorColor(),
            fontSize = 12.sp
        )
        model.testing -> Text(
            text = "测试中...",
            color = stepMutedColor(),
            fontSize = 12.sp
        )
    }
}
