package com.amitia.feature.memory

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaNumberField
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun WorldBookEntryEditScreen(
    entryId: String?,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    var title by remember { mutableStateOf("") }
    var keywords by remember { mutableStateOf("") }
    var content by remember { mutableStateOf("") }
    var enabled by remember { mutableStateOf(true) }
    var priority by remember { mutableStateOf("1") }
    var scope by remember { mutableStateOf("艾米") }
    var remark by remember { mutableStateOf("") }

    WorldBookEntryEditContent(
        title = title,
        onTitleChange = { title = it },
        keywords = keywords,
        onKeywordsChange = { keywords = it },
        content = content,
        onContentChange = { content = it },
        enabled = enabled,
        onEnabledChange = { enabled = it },
        priority = priority,
        onPriorityChange = { priority = it },
        scope = scope,
        onScopeChange = { scope = it },
        remark = remark,
        onRemarkChange = { remark = it },
        onBack = onBack,
        onSave = onSave
    )
}

@Composable
fun WorldBookEntryEditContent(
    title: String,
    onTitleChange: (String) -> Unit,
    keywords: String,
    onKeywordsChange: (String) -> Unit,
    content: String,
    onContentChange: (String) -> Unit,
    enabled: Boolean,
    onEnabledChange: (Boolean) -> Unit,
    priority: String,
    onPriorityChange: (String) -> Unit,
    scope: String,
    onScopeChange: (String) -> Unit,
    remark: String,
    onRemarkChange: (String) -> Unit,
    onBack: () -> Unit,
    onSave: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = if (title.isEmpty()) "新建条目" else "编辑条目",
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
            AmitiaTextField(
                value = title,
                onValueChange = onTitleChange,
                label = "标题",
                placeholder = "输入条目标题"
            )
            AmitiaTextField(
                value = keywords,
                onValueChange = onKeywordsChange,
                label = "关键词",
                placeholder = "多个关键词用逗号分隔",
                leadingIcon = AmitiaIcons.Label
            )
            AmitiaMultilineField(
                value = content,
                onValueChange = onContentChange,
                label = "正文",
                placeholder = "输入条目内容",
                minLines = 4,
                maxLines = 8,
                charLimit = 2000
            )
            AmitiaSwitchRow(
                title = "启用",
                subtitle = "控制此条目是否参与触发",
                checked = enabled,
                onCheckedChange = onEnabledChange,
                leadingIcon = AmitiaIcons.ToggleOn
            )
            AmitiaNumberField(
                value = priority,
                onValueChange = onPriorityChange,
                label = "优先级",
                placeholder = "1",
                onIncrement = { onPriorityChange(((priority.toIntOrNull() ?: 0) + 1).toString()) },
                onDecrement = { onPriorityChange(((priority.toIntOrNull() ?: 2) - 1).coerceAtLeast(0).toString()) }
            )
            AmitiaTextField(
                value = scope,
                onValueChange = onScopeChange,
                label = "角色范围",
                placeholder = "指定角色名",
                leadingIcon = AmitiaIcons.Person
            )
            AmitiaMultilineField(
                value = remark,
                onValueChange = onRemarkChange,
                label = "备注",
                placeholder = "可选的补充说明",
                minLines = 2,
                maxLines = 4,
                charLimit = 500
            )
            PrimaryButton(
                text = "保存条目",
                onClick = onSave,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
        }
    }
}

@Preview(name = "Entry Edit - Light", showBackground = true)
@Composable
private fun WorldBookEntryEditLightPreview() {
    AmitiaTheme(darkTheme = false) {
        WorldBookEntryEditContent(
            title = "城市设定",
            onTitleChange = {},
            keywords = "城市, 地点",
            onKeywordsChange = {},
            content = "故事发生在一座沿海城市",
            onContentChange = {},
            enabled = true,
            onEnabledChange = {},
            priority = "1",
            onPriorityChange = {},
            scope = "艾米",
            onScopeChange = {},
            remark = "",
            onRemarkChange = {},
            onBack = {},
            onSave = {}
        )
    }
}

@Preview(name = "Entry Edit - Dark", showBackground = true)
@Composable
private fun WorldBookEntryEditDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        WorldBookEntryEditContent(
            title = "",
            onTitleChange = {},
            keywords = "",
            onKeywordsChange = {},
            content = "",
            onContentChange = {},
            enabled = true,
            onEnabledChange = {},
            priority = "1",
            onPriorityChange = {},
            scope = "",
            onScopeChange = {},
            remark = "",
            onRemarkChange = {},
            onBack = {},
            onSave = {}
        )
    }
}
