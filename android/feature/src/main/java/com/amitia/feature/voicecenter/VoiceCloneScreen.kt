package com.amitia.feature.voicecenter

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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

private val cloneSteps = listOf(
    "说明与授权",
    "录音或上传",
    "质量检查",
    "提交",
    "处理状态",
    "试听",
    "绑定角色"
)

@Composable
fun VoiceCloneScreen(
    onBack: () -> Unit,
    viewModel: VoiceCenterViewModel = hiltViewModel()
) {
    val currentStep by viewModel.cloneStep.collectAsStateWithLifecycle()
    VoiceCloneContent(
        currentStep = currentStep,
        onNext = viewModel::nextCloneStep,
        onBack = onBack
    )
}

@Composable
fun VoiceCloneContent(
    currentStep: Int,
    onNext: () -> Unit,
    onBack: () -> Unit
) {
    var authorized by remember { mutableStateOf(false) }
    var isRecording by remember { mutableStateOf(false) }
    var isProcessing by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "声音复刻", onBack = onBack)
        StepIndicator(currentStep = currentStep, totalSteps = cloneSteps.size)
        Column(
            modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Text(
                text = "步骤 ${currentStep + 1} / ${cloneSteps.size}: ${cloneSteps[currentStep]}",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium
            )
            when (currentStep) {
                0 -> StepAuthorize(
                    authorized = authorized,
                    onAuthorizeChange = { authorized = it },
                    onNext = onNext
                )
                1 -> StepRecord(
                    isRecording = isRecording,
                    onToggleRecord = { isRecording = !isRecording },
                    onNext = onNext
                )
                2 -> StepQualityCheck(onNext = onNext)
                3 -> StepSubmit(
                    isProcessing = isProcessing,
                    onSubmit = { isProcessing = true; onNext() }
                )
                4 -> StepProcessing(onNext = onNext)
                5 -> StepPreview(onNext = onNext)
                else -> StepBindCharacter(onFinish = onBack)
            }
        }
    }
}

@Composable
private fun StepIndicator(currentStep: Int, totalSteps: Int) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        repeat(totalSteps) { index ->
            val isCompleted = index < currentStep
            val isCurrent = index == currentStep
            val color = when {
                isCompleted -> AmitiaStateColors.Running
                isCurrent -> MaterialTheme.colorScheme.primary
                else -> MaterialTheme.colorScheme.surfaceVariant
            }
            Box(
                modifier = Modifier.weight(1f).height(4.dp).clip(RoundedCornerShape(2.dp)).background(color)
            )
        }
    }
}

@Composable
private fun StepAuthorize(
    authorized: Boolean,
    onAuthorizeChange: (Boolean) -> Unit,
    onNext: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Icon(
                    imageVector = AmitiaIcons.Security,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(24.dp)
                )
                Text(
                    text = "声音使用授权",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "请确认您拥有所录制声音的合法使用权。克隆他人声音需获得本人授权，否则可能涉及法律风险。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSwitchRow(
                title = "我已确认拥有声音使用授权",
                checked = authorized,
                onCheckedChange = onAuthorizeChange,
                leadingIcon = AmitiaIcons.VerifiedUser
            )
        }
    }
    PrimaryButton(
        text = "下一步",
        onClick = onNext,
        enabled = authorized,
        leadingIcon = AmitiaIcons.ArrowForward,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepRecord(
    isRecording: Boolean,
    onToggleRecord: () -> Unit,
    onNext: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth().height(200.dp),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Box(contentAlignment = Alignment.Center) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Box(
                    modifier = Modifier.size(64.dp).clip(CircleShape)
                        .background(if (isRecording) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = if (isRecording) AmitiaIcons.Stop else AmitiaIcons.Mic,
                        contentDescription = null,
                        tint = if (isRecording) MaterialTheme.colorScheme.onError else MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.size(32.dp)
                    )
                }
                Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
                Text(
                    text = if (isRecording) "正在录音... 点击停止" else "点击开始录音",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
    Text(
        text = "请录制 15-30 秒清晰语音，建议在安静环境中进行",
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center
    )
    PrimaryButton(
        text = "使用录音继续",
        onClick = onNext,
        leadingIcon = AmitiaIcons.ArrowForward,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepQualityCheck(onNext: () -> Unit) {
    val checks = listOf(
        "音频格式检查" to true,
        "采样率检查 (48kHz)" to true,
        "噪音水平检查" to true,
        "音量稳定性检查" to true,
        "时长检查 (15-30s)" to false
    )
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            checks.forEach { (label, passed) ->
                Row(
                    modifier = Modifier.fillMaxWidth().padding(vertical = AmitiaSpacing.Xs),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Icon(
                        imageVector = if (passed) AmitiaIcons.CheckCircle else AmitiaIcons.WarningAmber,
                        contentDescription = null,
                        tint = if (passed) AmitiaStateColors.Running else AmitiaStateColors.Degraded,
                        modifier = Modifier.size(20.dp)
                    )
                    Text(
                        text = label,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                    Spacer(modifier = Modifier.weight(1f))
                    Text(
                        text = if (passed) "通过" else "需改善",
                        style = MaterialTheme.typography.labelSmall,
                        color = if (passed) AmitiaStateColors.Running else AmitiaStateColors.Degraded
                    )
                }
            }
        }
    }
    PrimaryButton(
        text = "继续提交",
        onClick = onNext,
        leadingIcon = AmitiaIcons.ArrowForward,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepSubmit(
    isProcessing: Boolean,
    onSubmit: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                imageVector = AmitiaIcons.CloudUpload,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(48.dp)
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "确认提交",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = "录音将上传至服务器进行处理，请保持网络连接",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
        }
    }
    LoadingButton(
        text = "提交",
        onClick = onSubmit,
        loading = isProcessing,
        leadingIcon = AmitiaIcons.Upload,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepProcessing(onNext: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Lg), horizontalAlignment = Alignment.CenterHorizontally) {
            com.amitia.core.designsystem.component.AmitiaLoadingIndicator(size = 48)
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "正在处理...",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                text = "声音模型训练中，预计需要 1-3 分钟",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center
            )
        }
    }
    PrimaryButton(
        text = "完成处理",
        onClick = onNext,
        leadingIcon = AmitiaIcons.ArrowForward,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepPreview(onNext: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Lg), horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                imageVector = AmitiaIcons.CheckCircle,
                contentDescription = null,
                tint = AmitiaStateColors.Running,
                modifier = Modifier.size(48.dp)
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Text(
                text = "声音克隆成功",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurface,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Base))
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Box(
                    modifier = Modifier.size(48.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.PlayArrow,
                        contentDescription = "试听",
                        tint = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.size(24.dp)
                    )
                }
                Text(
                    text = "点击试听克隆效果",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface
                )
            }
        }
    }
    PrimaryButton(
        text = "下一步",
        onClick = onNext,
        leadingIcon = AmitiaIcons.ArrowForward,
        modifier = Modifier.fillMaxWidth()
    )
}

@Composable
private fun StepBindCharacter(onFinish: () -> Unit) {
    Text(
        text = "将克隆的声音绑定到角色",
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant
    )
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
                Box(
                    modifier = Modifier.size(40.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Person,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(20.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = "艾米",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = "当前使用: 晓晓 (Azure)",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                TertiaryButton(text = "绑定", onClick = {})
            }
        }
    }
    PrimaryButton(
        text = "完成",
        onClick = onFinish,
        leadingIcon = AmitiaIcons.Check,
        modifier = Modifier.fillMaxWidth()
    )
}

@Preview(name = "VoiceClone - Light", showBackground = true)
@Composable
private fun VoiceCloneLightPreview() {
    AmitiaTheme(darkTheme = false) {
        VoiceCloneContent(currentStep = 0, onNext = {}, onBack = {})
    }
}

@Preview(name = "VoiceClone - Dark", showBackground = true)
@Composable
private fun VoiceCloneDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        VoiceCloneContent(currentStep = 5, onNext = {}, onBack = {})
    }
}
