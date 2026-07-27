package com.amitia.feature.settings.feedback

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaChipSelector
import com.amitia.core.designsystem.component.AmitiaChipItem
import com.amitia.core.designsystem.component.SuccessBanner
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun FeedbackScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val feedback = state.feedback
    var issueType by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var includeLogs by remember { mutableStateOf(true) }
    var contact by remember { mutableStateOf("") }
    var submitted by remember { mutableStateOf(false) }

    FeedbackScreenContent(
        issueType = issueType,
        description = description,
        includeLogs = includeLogs,
        contact = contact,
        submitted = submitted,
        onIssueTypeChange = { issueType = it },
        onDescriptionChange = { description = it },
        onIncludeLogsChange = { includeLogs = it },
        onContactChange = { contact = it },
        onSubmit = { submitted = true },
        onBack = onBack
    )
}

@Composable
private fun FeedbackScreenContent(
    issueType: String,
    description: String,
    includeLogs: Boolean,
    contact: String,
    submitted: Boolean,
    onIssueTypeChange: (String) -> Unit,
    onDescriptionChange: (String) -> Unit,
    onIncludeLogsChange: (Boolean) -> Unit,
    onContactChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onBack: () -> Unit
) {
    val issueTypes = remember {
        listOf("问题报告", "功能建议", "体验反馈", "其他").map {
            AmitiaChipItem(it, it == issueType)
        }
    }

    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "反馈", onBack = onBack) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            if (submitted) {
                SuccessBanner(message = "反馈已提交，感谢你的支持")
            }
            AmitiaSection(title = "问题类型") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        AmitiaChipSelector(
                            items = issueTypes,
                            onToggle = { index ->
                                onIssueTypeChange(issueTypes[index].label)
                            },
                            multiSelect = false
                        )
                    }
                }
            }
            AmitiaSection(title = "详细描述") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        AmitiaMultilineField(
                            value = description,
                            onValueChange = onDescriptionChange,
                            label = "描述",
                            placeholder = "请详细描述你遇到的问题或建议...",
                            charLimit = 500,
                            minLines = 4
                        )
                    }
                }
            }
            AmitiaSection(title = "截图") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                        SecondaryButton(
                            text = "添加截图",
                            onClick = {},
                            modifier = Modifier.fillMaxWidth(),
                            leadingIcon = AmitiaIcons.Image
                        )
                    }
                }
            }
            AmitiaSection(title = "附加信息") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        AmitiaSwitchRow(
                            title = "附带脱敏日志",
                            subtitle = "帮助开发者诊断问题",
                            checked = includeLogs,
                            onCheckedChange = onIncludeLogsChange,
                            leadingIcon = AmitiaIcons.BugReport
                        )
                        com.amitia.core.designsystem.component.AmitiaInsetDivider(
                            leadingInset = 56.dp + AmitiaSpacing.Base
                        )
                        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
                            AmitiaTextField(
                                value = contact,
                                onValueChange = onContactChange,
                                label = "联系方式（可选）",
                                placeholder = "邮箱或用户名",
                                leadingIcon = AmitiaIcons.Email
                            )
                        }
                    }
                }
            }
            PrimaryButton(
                text = "提交反馈",
                onClick = onSubmit,
                modifier = Modifier.fillMaxWidth(),
                enabled = issueType.isNotBlank() && description.isNotBlank(),
                leadingIcon = AmitiaIcons.Send
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Preview(name = "反馈页 - 亮色", showBackground = true)
@Composable
private fun FeedbackScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        FeedbackScreenContent(
            issueType = "",
            description = "",
            includeLogs = true,
            contact = "",
            submitted = false,
            onIssueTypeChange = {},
            onDescriptionChange = {},
            onIncludeLogsChange = {},
            onContactChange = {},
            onSubmit = {},
            onBack = {}
        )
    }
}

@Preview(name = "反馈页 - 暗色", showBackground = true)
@Composable
private fun FeedbackScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        FeedbackScreenContent(
            issueType = "问题报告",
            description = "应用在某些情况下会闪退",
            includeLogs = true,
            contact = "user@example.com",
            submitted = true,
            onIssueTypeChange = {},
            onDescriptionChange = {},
            onIncludeLogsChange = {},
            onContactChange = {},
            onSubmit = {},
            onBack = {}
        )
    }
}
