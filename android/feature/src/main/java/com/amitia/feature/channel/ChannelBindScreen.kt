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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun ChannelBindScreen(
    onBack: () -> Unit,
    onSuccess: () -> Unit,
    viewModel: ChannelBindViewModel = hiltViewModel()
) {
    val bindState by viewModel.bindState.collectAsStateWithLifecycle()
    LaunchedEffect(bindState.success) {
        if (bindState.success == true) onSuccess()
    }
    ChannelBindContent(
        bindState = bindState,
        onBack = onBack,
        onStartScan = viewModel::startScan,
        onRefresh = viewModel::refresh,
        onSimulateScan = { viewModel.markScanned(true) },
        onSimulateFail = { viewModel.markScanned(false) }
    )
}

@Composable
fun ChannelBindContent(
    bindState: ChannelBindState,
    onBack: () -> Unit,
    onStartScan: () -> Unit,
    onRefresh: () -> Unit,
    onSimulateScan: () -> Unit,
    onSimulateFail: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "渠道绑定", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            BindStatusCard(bindState = bindState)
            when {
                bindState.scanning -> ScanningBody(
                    countdown = bindState.countdownSeconds,
                    onSimulateScan = onSimulateScan,
                    onSimulateFail = onSimulateFail
                )
                bindState.success == true -> SuccessBody(onBack = onBack)
                bindState.success == false -> FailBody(reason = bindState.failReason, onRetry = onRefresh)
                else -> IdleBody(onStartScan = onStartScan)
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        }
    }
}

@Composable
private fun BindStatusCard(bindState: ChannelBindState) {
    val (text, color) = when {
        bindState.scanning -> "等待扫描中..." to MaterialTheme.colorScheme.primary
        bindState.success == true -> "绑定成功" to MaterialTheme.colorScheme.tertiary
        bindState.success == false -> "绑定失败" to MaterialTheme.colorScheme.error
        else -> "未开始" to MaterialTheme.colorScheme.onSurfaceVariant
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Box(modifier = Modifier.size(10.dp).clip(CircleShape).background(color))
            Text(
                text = text,
                style = MaterialTheme.typography.titleSmall,
                color = color,
                fontWeight = FontWeight.Medium
            )
        }
    }
}

@Composable
private fun ScanningBody(
    countdown: Int,
    onSimulateScan: () -> Unit,
    onSimulateFail: () -> Unit
) {
    QrPlaceholder(scanning = true)
    Text(
        text = "二维码有效期剩余 ${countdown}s",
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant
    )
    SecondaryButton(
        text = "刷新二维码",
        onClick = {},
        modifier = Modifier.fillMaxWidth(),
        leadingIcon = AmitiaIcons.Refresh
    )
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        SecondaryButton(
            text = "模拟扫描成功",
            onClick = onSimulateScan,
            modifier = Modifier.weight(1f),
            leadingIcon = AmitiaIcons.CheckCircle
        )
        DangerButton(
            text = "模拟失败",
            onClick = onSimulateFail,
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun SuccessBody(onBack: () -> Unit) {
    Box(
        modifier = Modifier
            .size(96.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.tertiaryContainer),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.CheckCircle,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.tertiary,
            modifier = Modifier.size(48.dp)
        )
    }
    Text(
        text = "渠道绑定成功",
        style = MaterialTheme.typography.titleMedium,
        color = MaterialTheme.colorScheme.onSurface,
        fontWeight = FontWeight.Medium
    )
    Text(
        text = "即将进入渠道详情页",
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center
    )
    PrimaryButton(
        text = "完成",
        onClick = onBack,
        modifier = Modifier.fillMaxWidth(),
        leadingIcon = AmitiaIcons.Check
    )
}

@Composable
private fun FailBody(reason: String?, onRetry: () -> Unit) {
    Box(
        modifier = Modifier
            .size(96.dp)
            .clip(CircleShape)
            .background(MaterialTheme.colorScheme.errorContainer),
        contentAlignment = Alignment.Center
    ) {
        Icon(
            imageVector = AmitiaIcons.Error,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.error,
            modifier = Modifier.size(48.dp)
        )
    }
    Text(
        text = "绑定失败",
        style = MaterialTheme.typography.titleMedium,
        color = MaterialTheme.colorScheme.error,
        fontWeight = FontWeight.Medium
    )
    Text(
        text = reason ?: "请重试",
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center
    )
    PrimaryButton(
        text = "重新绑定",
        onClick = onRetry,
        modifier = Modifier.fillMaxWidth(),
        leadingIcon = AmitiaIcons.Refresh
    )
}

@Composable
private fun IdleBody(onStartScan: () -> Unit) {
    QrPlaceholder(scanning = false)
    Text(
        text = "点击下方按钮生成二维码并等待扫描",
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        textAlign = TextAlign.Center
    )
    PrimaryButton(
        text = "开始绑定",
        onClick = onStartScan,
        modifier = Modifier.fillMaxWidth(),
        leadingIcon = AmitiaIcons.QrCode
    )
}

@Composable
private fun QrPlaceholder(scanning: Boolean) {
    Surface(
        modifier = Modifier.size(200.dp),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                imageVector = if (scanning) AmitiaIcons.QrCodeScanner else AmitiaIcons.QrCode,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(96.dp)
            )
        }
    }
}

@Preview(name = "ChannelBind - Light", showBackground = true)
@Composable
private fun ChannelBindLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelBindContent(
            bindState = ChannelBindState(scanning = true, countdownSeconds = 96, scanned = false, success = null, failReason = null),
            onBack = {}, onStartScan = {}, onRefresh = {}, onSimulateScan = {}, onSimulateFail = {}
        )
    }
}

@Preview(name = "ChannelBind - Dark", showBackground = true)
@Composable
private fun ChannelBindDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelBindContent(
            bindState = ChannelBindState(scanning = false, countdownSeconds = 0, scanned = true, success = true, failReason = null),
            onBack = {}, onStartScan = {}, onRefresh = {}, onSimulateScan = {}, onSimulateFail = {}
        )
    }
}
