package com.amitia.feature.channel

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaPasswordField
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun ChannelCreateScreen(
    onBack: () -> Unit,
    onComplete: () -> Unit,
    viewModel: ChannelCreateViewModel = hiltViewModel()
) {
    val step by viewModel.step.collectAsStateWithLifecycle()
    val selectedType by viewModel.selectedType.collectAsStateWithLifecycle()
    ChannelCreateContent(
        step = step,
        steps = viewModel.steps,
        selectedType = selectedType,
        onBack = onBack,
        onSelectType = viewModel::selectType,
        onNext = viewModel::next,
        onPrev = viewModel::back,
        onComplete = onComplete
    )
}

@Composable
fun ChannelCreateContent(
    step: Int,
    steps: List<ChannelCreateStep>,
    selectedType: ChannelType?,
    onBack: () -> Unit,
    onSelectType: (ChannelType) -> Unit,
    onNext: () -> Unit,
    onPrev: () -> Unit,
    onComplete: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "新建渠道", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            StepIndicator(current = step, total = steps.size)
            StepHeader(title = steps[step].title, description = steps[step].description)
            when (step) {
                0 -> StepTypeSelect(selectedType = selectedType, onSelect = onSelectType)
                1 -> StepInfoForm()
                2 -> StepBindForm()
                3 -> StepConfirmForm()
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                if (step > 0) {
                    SecondaryButton(
                        text = "上一步",
                        onClick = onPrev,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.ArrowBack
                    )
                }
                if (step < steps.lastIndex) {
                    LoadingButton(
                        text = "下一步",
                        onClick = onNext,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.ArrowForward
                    )
                } else {
                    LoadingButton(
                        text = "完成创建",
                        onClick = onComplete,
                        modifier = Modifier.weight(1f),
                        leadingIcon = AmitiaIcons.Check
                    )
                }
            }
        }
    }
}

@Composable
private fun StepIndicator(current: Int, total: Int) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
    ) {
        repeat(total) { index ->
            val isCurrent = index == current
            val isDone = index < current
            Box(
                modifier = Modifier
                    .weight(1f)
                    .height(4.dp)
                    .clip(RoundedCornerShape(2.dp))
                    .background(
                        if (isCurrent || isDone) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant
                    )
            )
        }
    }
    Text(
        text = "步骤 ${current + 1} / $total",
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant
    )
}

@Composable
private fun StepHeader(title: String, description: String) {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xxs)) {
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
}

@Composable
private fun StepTypeSelect(selectedType: ChannelType?, onSelect: (ChannelType) -> Unit) {
    ChannelType.entries.forEach { type ->
        AmitiaSelectionRow(
            title = type.label,
            selected = selectedType == type,
            onSelect = { onSelect(type) },
            subtitle = typeSubtitle(type),
            leadingIcon = channelIcon(type)
        )
    }
}

private fun typeSubtitle(type: ChannelType) = when (type) {
    ChannelType.Web -> "基础渠道，固定开启"
    ChannelType.WeChat -> "通过扫码绑定微信账号"
    ChannelType.QQ -> "通过扫码绑定 QQ 账号"
    ChannelType.Api -> "通过 API Key 接入第三方系统"
    ChannelType.ThirdParty -> "从已安装的公共插件选择"
}

@Composable
private fun StepInfoForm() {
    AmitiaTextField(value = "", onValueChange = {}, label = "渠道名称", placeholder = "例如：生产 API")
    AmitiaTextField(value = "", onValueChange = {}, label = "端点", placeholder = "https://...", leadingIcon = AmitiaIcons.Link)
    AmitiaPasswordField(value = "", onValueChange = {}, label = "凭据", placeholder = "API Key 或 Token")
}

@Composable
private fun StepBindForm() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(
                modifier = Modifier
                    .size(160.dp)
                    .clip(RoundedCornerShape(12.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.QrCode,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(80.dp)
                )
            }
            Text(
                text = "使用对应客户端扫描二维码完成授权",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun StepConfirmForm() {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "确认以下配置后完成创建：",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
        ConfirmLine("默认角色", "艾米")
        ConfirmLine("通知策略", "新消息 + 失败提醒")
        ConfirmLine("重试策略", "指数退避")
    }
}

@Composable
private fun ConfirmLine(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(text = label, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(text = value, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Medium)
    }
}

@Preview(name = "ChannelCreate - Light", showBackground = true)
@Composable
private fun ChannelCreateLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelCreateContent(
            step = 0,
            steps = ChannelMockData.createSteps,
            selectedType = ChannelType.WeChat,
            onBack = {}, onSelectType = {}, onNext = {}, onPrev = {}, onComplete = {}
        )
    }
}

@Preview(name = "ChannelCreate - Dark", showBackground = true)
@Composable
private fun ChannelCreateDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelCreateContent(
            step = 2,
            steps = ChannelMockData.createSteps,
            selectedType = null,
            onBack = {}, onSelectType = {}, onNext = {}, onPrev = {}, onComplete = {}
        )
    }
}
