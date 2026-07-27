package com.amitia.feature.channel

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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaPasswordField
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.DangerButton
import com.amitia.core.designsystem.component.LoadingButton

@Composable
fun ChannelEditScreen(
    channelId: String?,
    onBack: () -> Unit,
    viewModel: ChannelEditViewModel = hiltViewModel()
) {
    val form by viewModel.form.collectAsStateWithLifecycle()
    val saving by viewModel.saving.collectAsStateWithLifecycle()
    ChannelEditContent(
        form = form,
        saving = saving,
        isEdit = channelId != null,
        onBack = onBack,
        onUpdate = viewModel::update,
        onSave = { viewModel.save(onBack) }
    )
}

@Composable
fun ChannelEditContent(
    form: ChannelEditForm,
    saving: Boolean,
    isEdit: Boolean,
    onBack: () -> Unit,
    onUpdate: (((ChannelEditForm) -> ChannelEditForm)) -> Unit,
    onSave: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = if (isEdit) "编辑渠道" else "编辑渠道", onBack = onBack)
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "基础信息")
            AmitiaTextField(
                value = form.name,
                onValueChange = { v -> onUpdate { it.copy(name = v) } },
                label = "渠道名称",
                placeholder = "例如：生产 API",
                leadingIcon = AmitiaIcons.Label
            )
            AmitiaSectionHeader(title = "渠道类型")
            ChannelType.entries.forEach { type ->
                AmitiaSelectionRow(
                    title = type.label,
                    selected = form.type == type,
                    onSelect = { onUpdate { it.copy(type = type) } },
                    leadingIcon = channelIcon(type)
                )
            }
            AmitiaSectionHeader(title = "接入配置")
            AmitiaTextField(
                value = form.baseUrl,
                onValueChange = { v -> onUpdate { it.copy(baseUrl = v) } },
                label = "Base URL",
                placeholder = "https://api.example.com/v1",
                leadingIcon = AmitiaIcons.Link
            )
            AmitiaPasswordField(
                value = form.apiKey,
                onValueChange = { v -> onUpdate { it.copy(apiKey = v) } },
                label = "凭据",
                placeholder = "API Key 或 Token"
            )
            AmitiaSectionHeader(title = "运行策略")
            AmitiaTextField(
                value = form.retryPolicy,
                onValueChange = { v -> onUpdate { it.copy(retryPolicy = v) } },
                label = "重试策略",
                placeholder = "指数退避",
                leadingIcon = AmitiaIcons.Restore
            )
            AmitiaSwitchRow(
                title = "启用渠道",
                checked = form.enabled,
                onCheckedChange = { v -> onUpdate { it.copy(enabled = v) } },
                subtitle = "关闭后该渠道停止接收与投递",
                leadingIcon = AmitiaIcons.ToggleOn
            )
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            LoadingButton(
                text = "保存修改",
                onClick = onSave,
                loading = saving,
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Check
            )
            if (isEdit) {
                DangerButton(
                    text = "删除渠道",
                    onClick = onBack,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Delete
                )
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
        }
    }
}

@Preview(name = "ChannelEdit - Light", showBackground = true)
@Composable
private fun ChannelEditLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ChannelEditContent(
            form = ChannelEditForm(name = "生产 API", type = ChannelType.Api, baseUrl = "https://api.example.com/v1", enabled = true),
            saving = false,
            isEdit = true,
            onBack = {}, onUpdate = {}, onSave = {}
        )
    }
}

@Preview(name = "ChannelEdit - Dark", showBackground = true)
@Composable
private fun ChannelEditDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ChannelEditContent(
            form = ChannelEditForm(),
            saving = true,
            isEdit = false,
            onBack = {}, onUpdate = {}, onSave = {}
        )
    }
}
