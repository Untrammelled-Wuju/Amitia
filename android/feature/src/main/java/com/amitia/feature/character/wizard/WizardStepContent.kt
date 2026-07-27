package com.amitia.feature.character.wizard

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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.AddAPhoto
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.GraphicEq
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Psychology
import androidx.compose.material.icons.outlined.Security
import androidx.compose.material.icons.outlined.SmartToy
import androidx.compose.material.icons.outlined.Hub
import androidx.compose.material.icons.outlined.Memory
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField

@Composable
fun WizardStepContent(
    step: WizardStep,
    draftName: String,
    onDraftNameChange: (String) -> Unit,
    draftIdentity: String,
    onDraftIdentityChange: (String) -> Unit,
    draftDescription: String,
    onDraftDescriptionChange: (String) -> Unit
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        StepHeader(step)
        when (step) {
            WizardStep.Appearance -> AppearanceStep()
            WizardStep.Name -> NameStep(
                draftName = draftName,
                onDraftNameChange = onDraftNameChange,
                draftIdentity = draftIdentity,
                onDraftIdentityChange = onDraftIdentityChange,
                draftDescription = draftDescription,
                onDraftDescriptionChange = onDraftDescriptionChange
            )
            WizardStep.Personality -> PersonalityStep()
            WizardStep.Voice -> VoiceStep()
            WizardStep.Model -> ModelStep()
            WizardStep.Memory -> MemoryStep()
            WizardStep.Proactive -> ProactiveStep()
            WizardStep.Channel -> ChannelStep()
            WizardStep.Permission -> PermissionStep()
            WizardStep.Preview -> PreviewStep(
                draftName = draftName,
                draftIdentity = draftIdentity,
                draftDescription = draftDescription
            )
        }
    }
}

@Composable
private fun StepHeader(step: WizardStep) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            text = step.title,
            style = MaterialTheme.typography.headlineSmall,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
        Text(
            text = step.description,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}

@Composable
private fun AppearanceStep() {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Box(
            modifier = Modifier
                .size(120.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center
        ) {
            Icon(
                imageVector = Icons.Outlined.AddAPhoto,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(32.dp)
            )
        }
        TertiaryActionRow(
            icon = Icons.Outlined.AddAPhoto,
            title = "上传头像",
            subtitle = "支持 JPG、PNG，建议 512x512"
        )
        TertiaryActionRow(
            icon = Icons.Outlined.Person,
            title = "上传立绘",
            subtitle = "全身形象，透明背景更佳"
        )
        Text(
            text = "主题色",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf(
                0xFF8FA8A0, 0xFFC9A17F, 0xFFC9A0A8,
                0xFF5E7477, 0xFFB0857A
            ).forEach { color ->
                Box(
                    modifier = Modifier
                        .size(36.dp)
                        .clip(CircleShape)
                        .background(androidx.compose.ui.graphics.Color(color))
                )
            }
        }
    }
}

@Composable
private fun NameStep(
    draftName: String,
    onDraftNameChange: (String) -> Unit,
    draftIdentity: String,
    onDraftIdentityChange: (String) -> Unit,
    draftDescription: String,
    onDraftDescriptionChange: (String) -> Unit
) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        AmitiaTextField(
            value = draftName,
            onValueChange = onDraftNameChange,
            label = "角色名称",
            placeholder = "为你的角色取一个名字"
        )
        AmitiaTextField(
            value = draftIdentity,
            onValueChange = onDraftIdentityChange,
            label = "身份",
            placeholder = "如：温柔知性的陪伴助手"
        )
        AmitiaTextField(
            value = "",
            onValueChange = {},
            label = "称呼用户的方式",
            placeholder = "如：主人、朋友、你"
        )
        AmitiaMultilineField(
            value = draftDescription,
            onValueChange = onDraftDescriptionChange,
            label = "简介",
            placeholder = "简要描述角色背景",
            charLimit = 200
        )
        AmitiaMultilineField(
            value = "",
            onValueChange = {},
            label = "语言风格",
            placeholder = "描述角色的说话风格",
            minLines = 2
        )
    }
}

@Composable
private fun PersonalityStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            text = "预设模板",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        listOf("均衡型", "温柔型", "活力型", "沉稳型", "俏皮型").forEachIndexed { index, preset ->
            AmitiaSelectionRow(
                title = preset,
                subtitle = "点击选择此预设作为基础",
                selected = index == 0,
                onSelect = {}
            )
        }
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            text = "关键维度预览（完整 36 维可在详情页配置）",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        AmitiaSlider(
            value = 0.6f,
            onValueChange = {},
            label = "主动发起",
            valueRange = 0f..1f,
            valueFormatter = { "${(it * 100).toInt()}%" }
        )
        AmitiaSlider(
            value = 0.8f,
            onValueChange = {},
            label = "情绪稳定性",
            valueRange = 0f..1f,
            valueFormatter = { "${(it * 100).toInt()}%" }
        )
        AmitiaSlider(
            value = 0.5f,
            onValueChange = {},
            label = "幽默倾向",
            valueRange = 0f..1f,
            valueFormatter = { "${(it * 100).toInt()}%" }
        )
    }
}

@Composable
private fun VoiceStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        AmitiaSelectionRow(
            title = "使用默认声音",
            subtitle = "继承全局语音配置",
            selected = true,
            onSelect = {},
            leadingIcon = Icons.Outlined.GraphicEq
        )
        AmitiaSelectionRow(
            title = "选择专属声音",
            subtitle = "为该角色配置独立语音",
            selected = false,
            onSelect = {},
            leadingIcon = Icons.Outlined.GraphicEq
        )
        AmitiaSelectionRow(
            title = "声音复刻",
            subtitle = "上传参考音频进行复刻",
            selected = false,
            onSelect = {},
            leadingIcon = Icons.Outlined.GraphicEq
        )
    }
}

@Composable
private fun ModelStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        AmitiaSwitchRow(
            title = "继承全局模型配置",
            subtitle = "使用全局设置的文本、视觉、语音和向量模型",
            checked = true,
            onCheckedChange = {}
        )
        Text(
            text = "角色专属模型",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        TertiaryActionRow(
            icon = Icons.Outlined.SmartToy,
            title = "文本模型",
            subtitle = "未配置（继承全局）"
        )
        TertiaryActionRow(
            icon = Icons.Outlined.SmartToy,
            title = "视觉模型",
            subtitle = "未配置（继承全局）"
        )
        TertiaryActionRow(
            icon = Icons.Outlined.SmartToy,
            title = "向量模型",
            subtitle = "未配置（继承全局）"
        )
    }
}

@Composable
private fun MemoryStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        AmitiaSwitchRow(
            title = "长期记忆",
            subtitle = "持久化存储重要信息",
            checked = true,
            onCheckedChange = {},
            leadingIcon = Icons.Outlined.Memory
        )
        AmitiaSwitchRow(
            title = "情景记忆",
            subtitle = "记录对话场景和上下文",
            checked = true,
            onCheckedChange = {}
        )
        AmitiaSwitchRow(
            title = "自动总结",
            subtitle = "定期总结对话生成摘要记忆",
            checked = true,
            onCheckedChange = {}
        )
        TertiaryActionRow(
            icon = Icons.Outlined.Memory,
            title = "绑定世界书",
            subtitle = "选择关联的世界书条目"
        )
    }
}

@Composable
private fun ProactiveStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        AmitiaSwitchRow(
            title = "启用主动消息",
            subtitle = "角色会在合适的时间主动联系你",
            checked = true,
            onCheckedChange = {},
            leadingIcon = Icons.Outlined.NotificationsActive
        )
        AmitiaSlider(
            value = 0.5f,
            onValueChange = {},
            label = "主动频率",
            valueRange = 0f..1f,
            valueFormatter = { "${(it * 100).toInt()}%" }
        )
        Text(
            text = "时间窗",
            style = MaterialTheme.typography.titleSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        listOf("早晨 8:00-10:00", "午后 13:00-15:00", "晚间 19:00-22:00").forEach { window ->
            AmitiaSelectionRow(
                title = window,
                selected = true,
                onSelect = {},
                multiSelect = true
            )
        }
    }
}

@Composable
private fun ChannelStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            text = "选择该角色可用的渠道",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        listOf("Web 网页端", "微信", "QQ", "Telegram").forEach { channel ->
            AmitiaSelectionRow(
                title = channel,
                selected = channel == "Web 网页端",
                onSelect = {},
                multiSelect = true,
                leadingIcon = Icons.Outlined.Hub
            )
        }
    }
}

@Composable
private fun PermissionStep() {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(
            text = "设置角色可使用的权限",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        AmitiaSwitchRow(
            title = "文件访问",
            checked = true,
            onCheckedChange = {},
            leadingIcon = Icons.Outlined.Security
        )
        AmitiaSwitchRow(
            title = "通知推送",
            checked = true,
            onCheckedChange = {}
        )
        AmitiaSwitchRow(
            title = "Computer Use",
            subtitle = "桌面操作能力，需谨慎开启",
            checked = false,
            onCheckedChange = {}
        )
        AmitiaSwitchRow(
            title = "位置访问",
            checked = false,
            onCheckedChange = {}
        )
    }
}

@Composable
private fun PreviewStep(
    draftName: String,
    draftIdentity: String,
    draftDescription: String
) {
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text(
            text = "确认角色信息",
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
        PreviewInfoRow(label = "名称", value = draftName.ifBlank { "未填写" })
        PreviewInfoRow(label = "身份", value = draftIdentity.ifBlank { "未填写" })
        PreviewInfoRow(label = "简介", value = draftDescription.ifBlank { "未填写" })
        PreviewInfoRow(label = "性格", value = "均衡型预设")
        PreviewInfoRow(label = "声音", value = "继承全局")
        PreviewInfoRow(label = "模型", value = "继承全局")
        PreviewInfoRow(label = "主动消息", value = "已启用")
        PreviewInfoRow(label = "渠道", value = "Web")
        Spacer(modifier = Modifier.height(8.dp))
        Surface(
            modifier = Modifier.fillMaxWidth(),
            shape = RoundedCornerShape(12.dp),
            color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
        ) {
            Row(
                modifier = Modifier.padding(12.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Icon(
                    imageVector = Icons.Outlined.Check,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.tertiary,
                    modifier = Modifier.size(20.dp)
                )
                Text(
                    text = "创建后可随时在角色详情页调整所有设置",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onTertiaryContainer
                )
            }
        }
    }
}

@Composable
private fun TertiaryActionRow(
    icon: ImageVector,
    title: String,
    subtitle: String
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
            }
        }
    }
}

@Composable
private fun PreviewInfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f, fill = false)
        )
    }
}
