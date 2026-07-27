package com.amitia.feature.onboarding.steps

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.shrinkVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun EnvCheckStepPage(
    state: OnboardingFlowUiState,
    onCheck: () -> Unit,
    onNext: () -> Unit,
    modifier: Modifier = Modifier
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
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
        ) {
            StepTitle(text = "运行环境检查")
            StepDescription(text = "确认设备满足运行条件，必需项全部通过后可继续。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            state.envItems.forEach { item ->
                EnvCheckRow(item = item)
            }
            if (state.envChecking) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    AmitiaLoadingIndicator()
                    Text(
                        text = "正在检查…",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            if (state.envItems.isEmpty()) {
                PrimaryButton(
                    text = "开始检查",
                    onClick = onCheck,
                    modifier = Modifier.fillMaxWidth()
                )
            } else {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    SecondaryButton(
                        text = "重新检查",
                        onClick = onCheck,
                        modifier = Modifier.weight(1f),
                        enabled = !state.envChecking
                    )
                    PrimaryButton(
                        text = "下一步",
                        onClick = onNext,
                        modifier = Modifier.weight(1f),
                        enabled = state.allEnvRequiredPassed && !state.envChecking
                    )
                }
            }
        }
    }
}

@Composable
private fun EnvCheckRow(item: EnvCheckItem) {
    var expanded by remember { mutableStateOf(false) }
    val statusColor = if (item.passed) AmitiaStateColors.Running else AmitiaStateColors.Failed
    val interactionSource = remember { MutableInteractionSource() }
    Column {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clip(AmitiaCardShape)
                .background(MaterialTheme.colorScheme.surface)
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                    role = Role.Button,
                    onClick = { if (!item.passed) expanded = !expanded }
                )
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(24.dp)
                    .clip(CircleShape)
                    .background(if (item.passed) statusColor else MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                if (item.passed) {
                    Icon(
                        imageVector = AmitiaIcons.Check,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.size(16.dp)
                    )
                } else {
                    Icon(
                        imageVector = AmitiaIcons.Warning,
                        contentDescription = null,
                        tint = statusColor,
                        modifier = Modifier.size(16.dp)
                    )
                }
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = item.name,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                if (item.detail.isNotBlank()) {
                    Text(
                        text = item.detail,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            if (!item.required) {
                Text(
                    text = "可选",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
        }
        AnimatedVisibility(
            visible = expanded && !item.passed,
            enter = expandVertically() + fadeIn(),
            exit = shrinkVertically()
        ) {
            Surface(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = AmitiaSpacing.Xs),
                shape = AmitiaCardShape,
                color = statusColor.copy(alpha = 0.08f)
            ) {
                Text(
                    text = "建议：更新系统或释放资源后重新检查此项目。",
                    style = MaterialTheme.typography.bodySmall,
                    color = statusColor,
                    modifier = Modifier.padding(AmitiaSpacing.Base)
                )
            }
        }
    }
}

@Preview(name = "EnvCheck - Light", showBackground = true)
@Composable
private fun EnvCheckStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EnvCheckStepPage(
            state = OnboardingFlowUiState(
                envChecking = false,
                envItems = listOf(
                    EnvCheckItem("Android 版本", true, "Android 13+"),
                    EnvCheckItem("CPU 架构", true, "arm64-v8a"),
                    EnvCheckItem("可用存储", true, "剩余 4.2 GB"),
                    EnvCheckItem("内存", false, "不足", required = false)
                ),
                currentStep = OnboardingFlowStep.EnvCheck
            ),
            onCheck = {},
            onNext = {}
        )
    }
}

@Preview(name = "EnvCheck - Dark", showBackground = true)
@Composable
private fun EnvCheckStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EnvCheckStepPage(
            state = OnboardingFlowUiState(
                envChecking = true,
                currentStep = OnboardingFlowStep.EnvCheck
            ),
            onCheck = {},
            onNext = {}
        )
    }
}

private enum class Role { Button, RadioButton, Switch, Checkbox }
