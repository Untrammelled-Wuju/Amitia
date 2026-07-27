package com.amitia.feature.emoji

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
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun EmojiDetailEditScreen(
    emojiId: String?,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    var meaning by remember { mutableStateOf("") }
    var context by remember { mutableStateOf("") }
    var groupName by remember { mutableStateOf("日常表情") }
    var sharedAll by remember { mutableStateOf(true) }
    var enabled by remember { mutableStateOf(true) }

    EmojiDetailEditContent(
        meaning = meaning,
        onMeaningChange = { meaning = it },
        context = context,
        onContextChange = { context = it },
        groupName = groupName,
        onGroupNameChange = { groupName = it },
        sharedAll = sharedAll,
        onSharedAllChange = { sharedAll = it },
        enabled = enabled,
        onEnabledChange = { enabled = it },
        source = "手动导入",
        importedAt = "今天 14:30",
        onBack = onBack,
        onSave = onSave
    )
}

@Composable
fun EmojiDetailEditContent(
    meaning: String,
    onMeaningChange: (String) -> Unit,
    context: String,
    onContextChange: (String) -> Unit,
    groupName: String,
    onGroupNameChange: (String) -> Unit,
    sharedAll: Boolean,
    onSharedAllChange: (Boolean) -> Unit,
    enabled: Boolean,
    onEnabledChange: (Boolean) -> Unit,
    source: String,
    importedAt: String,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = if (meaning.isEmpty()) "编辑表情包" else "表情包详情",
            onBack = onBack,
            actions = {
                AmitiaIconButton(
                    icon = AmitiaIcons.Check,
                    contentDescription = "保存",
                    onClick = onSave
                )
            }
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(20.dp),
                color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(200.dp),
                    contentAlignment = Alignment.Center
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Box(
                            modifier = Modifier
                                .size(64.dp)
                                .clip(RoundedCornerShape(16.dp))
                                .background(MaterialTheme.colorScheme.surface),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                imageVector = AmitiaIcons.Image,
                                contentDescription = null,
                                tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f),
                                modifier = Modifier.size(AmitiaIconSize.Huge)
                            )
                        }
                        Text(
                            text = "图片预览",
                            style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            AmitiaSectionHeader(title = "含义信息")
            AmitiaTextField(
                value = meaning,
                onValueChange = onMeaningChange,
                label = "含义",
                placeholder = "输入表情包的含义",
                leadingIcon = AmitiaIcons.Label
            )
            AmitiaMultilineField(
                value = context,
                onValueChange = onContextChange,
                label = "补充语境",
                placeholder = "可选的使用场景描述",
                minLines = 2,
                maxLines = 4,
                charLimit = 200
            )

            AmitiaSectionHeader(title = "归属")
            AmitiaTextField(
                value = groupName,
                onValueChange = onGroupNameChange,
                label = "所属分组",
                placeholder = "选择或输入分组名",
                leadingIcon = AmitiaIcons.Folder
            )
            AmitiaSwitchRow(
                title = "全角色共享",
                subtitle = "所有角色都可以使用此表情包",
                checked = sharedAll,
                onCheckedChange = onSharedAllChange,
                leadingIcon = AmitiaIcons.People
            )

            AmitiaSectionHeader(title = "状态")
            AmitiaSwitchRow(
                title = "启用",
                subtitle = "控制此表情包是否可用",
                checked = enabled,
                onCheckedChange = onEnabledChange,
                leadingIcon = AmitiaIcons.ToggleOn
            )

            AmitiaSectionHeader(title = "来源信息")
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(12.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(
                    modifier = Modifier.padding(AmitiaSpacing.Base),
                    verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
                ) {
                    InfoRow(label = "来源", value = source)
                    InfoRow(label = "导入时间", value = importedAt)
                }
            }

            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            PrimaryButton(
                text = "保存",
                onClick = onSave,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurface,
            fontWeight = FontWeight.Medium
        )
    }
}

@Preview(name = "Emoji Edit - Light", showBackground = true)
@Composable
private fun EmojiDetailEditLightPreview() {
    AmitiaTheme(darkTheme = false) {
        EmojiDetailEditContent(
            meaning = "开心",
            onMeaningChange = {},
            context = "用于表达愉快心情",
            onContextChange = {},
            groupName = "日常表情",
            onGroupNameChange = {},
            sharedAll = true,
            onSharedAllChange = {},
            enabled = true,
            onEnabledChange = {},
            source = "手动导入",
            importedAt = "今天 14:30",
            onBack = {},
            onSave = {}
        )
    }
}

@Preview(name = "Emoji Edit - Dark", showBackground = true)
@Composable
private fun EmojiDetailEditDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        EmojiDetailEditContent(
            meaning = "",
            onMeaningChange = {},
            context = "",
            onContextChange = {},
            groupName = "",
            onGroupNameChange = {},
            sharedAll = false,
            onSharedAllChange = {},
            enabled = true,
            onEnabledChange = {},
            source = "批量导入",
            importedAt = "昨天",
            onBack = {},
            onSave = {}
        )
    }
}
