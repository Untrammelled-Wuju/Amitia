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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaCardShape
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun PermissionsStepPage(
    state: OnboardingFlowUiState,
    onToggle: (String) -> Unit,
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
            StepTitle(text = "基础权限")
            StepDescription(text = "以下权限用于核心功能，你可以按需开启，稍后也可在设置中调整。")
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSwitchRow(
                title = "通知",
                subtitle = "接收消息推送和提醒",
                checked = state.permissionNotifications,
                onCheckedChange = { onToggle("notifications") },
                leadingIcon = AmitiaIcons.Notifications
            )
            AmitiaSwitchRow(
                title = "麦克风",
                subtitle = "语音输入和通话功能",
                checked = state.permissionMicrophone,
                onCheckedChange = { onToggle("microphone") },
                leadingIcon = AmitiaIcons.Mic
            )
            AmitiaSwitchRow(
                title = "文件 / 媒体选择器",
                subtitle = "选择图片、文件等附件",
                checked = state.permissionFiles,
                onCheckedChange = { onToggle("files") },
                leadingIcon = AmitiaIcons.Folder
            )
            AmitiaSwitchRow(
                title = "开机自启动",
                subtitle = "设备重启后自动恢复服务",
                checked = state.permissionAutoStart,
                onCheckedChange = { onToggle("autostart") },
                leadingIcon = AmitiaIcons.PowerSettingsNew
            )
            WebChatPermissionCard()
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "下一步",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.ArrowForward
            )
        }
    }
}

@Composable
private fun WebChatPermissionCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = AmitiaCardShape,
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .clip(CircleShape)
                    .background(AmitiaStateColors.Running.copy(alpha = 0.15f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = AmitiaIcons.Chat,
                    contentDescription = null,
                    tint = AmitiaStateColors.Running,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "Web 聊天",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "已开启，不可关闭",
                    style = MaterialTheme.typography.bodySmall,
                    color = AmitiaStateColors.Running
                )
            }
            Surface(
                shape = AmitiaCardShape,
                color = AmitiaStateColors.Running.copy(alpha = 0.1f)
            ) {
                Text(
                    text = "已开启",
                    style = MaterialTheme.typography.labelSmall,
                    color = AmitiaStateColors.Running,
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = AmitiaSpacing.Xs)
                )
            }
        }
    }
}

@Preview(name = "Permissions - Light", showBackground = true)
@Composable
private fun PermissionsStepPageLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PermissionsStepPage(
            state = OnboardingFlowUiState(
                permissionNotifications = true,
                permissionMicrophone = true
            ),
            onToggle = {},
            onNext = {}
        )
    }
}

@Preview(name = "Permissions - Dark", showBackground = true)
@Composable
private fun PermissionsStepPageDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PermissionsStepPage(
            state = OnboardingFlowUiState(),
            onToggle = {},
            onNext = {}
        )
    }
}
