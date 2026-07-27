package com.amitia.feature.onboarding.steps

import androidx.compose.foundation.border
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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun ModeSelectionStepPage(
    selectedMode: OnboardingRunMode?,
    onSelect: (OnboardingRunMode) -> Unit,
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
            StepTitle(text = "选择运行模式")
            StepDescription(text = "你可以稍后在设置中切换，现在请选择一种启动方式。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            ModeCard(
                title = "本地运行",
                description = "数据优先保存在本机，需要更多存储和运行资源。",
                icon = AmitiaIcons.Storage,
                selected = selectedMode == OnboardingRunMode.Local,
                onSelect = { onSelect(OnboardingRunMode.Local) }
            )
            ModeCard(
                title = "远程连接",
                description = "连接已有 Amitia 服务端，需要服务地址或账号授权。",
                icon = AmitiaIcons.CloudDone,
                selected = selectedMode == OnboardingRunMode.Remote,
                onSelect = { onSelect(OnboardingRunMode.Remote) }
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                enabled = selectedMode != null
            )
        }
    }
}

@Composable
private fun ModeCard(
    title: String,
    description: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    selected: Boolean,
    onSelect: () -> Unit
) {
    val interactionSource = remember { MutableInteractionSource() }
    val borderColor = if (selected) MaterialTheme.colorScheme.primary
    else MaterialTheme.colorScheme.outline
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clip(AmitiaCardShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                role = Role.RadioButton,
                onClick = onSelect
            )
            .border(
                width = if (selected) 2.dp else 1.dp,
                color = borderColor,
                shape = AmitiaCardShape
            ),
        shape = AmitiaCardShape,
        color = if (selected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
        else MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(AmitiaCardShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(24.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Icon(
                imageVector = if (selected) AmitiaIcons.RadioButtonChecked else AmitiaIcons.RadioButtonUnchecked,
                contentDescription = null,
                tint = if (selected) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                modifier = Modifier.size(24.dp)
            )
        }
    }
}

@Preview(name = "ModeSelection - Light", showBackground = true)
@Composable
private fun ModeSelectionStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ModeSelectionStepPage(
            selectedMode = OnboardingRunMode.Local,
            onSelect = {},
            onNext = {}
        )
    }
}

@Preview(name = "ModeSelection - Dark", showBackground = true)
@Composable
private fun ModeSelectionStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ModeSelectionStepPage(
            selectedMode = null,
            onSelect = {},
            onNext = {}
        )
    }
}
