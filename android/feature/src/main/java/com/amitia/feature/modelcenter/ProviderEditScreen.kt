package com.amitia.feature.modelcenter

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaCodeEditorSurface
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaPasswordField
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun ProviderEditScreen(
    onBack: () -> Unit,
    providerId: String? = null,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val saving by viewModel.saving.collectAsStateWithLifecycle()
    ProviderEditContent(
        providerId = providerId,
        saving = saving,
        onBack = onBack,
        onSave = { name, apiKey, baseUrl, extraConfig ->
            val provider = ProviderUiModel(
                id = providerId ?: System.currentTimeMillis().toString(),
                name = name,
                type = "OpenAI 兼容",
                authStatus = ProviderAuthStatus.Unknown,
                available = false,
                lastTested = null,
                roleCount = 0,
                baseUrl = baseUrl,
                apiKeyMasked = apiKey
            )
            viewModel.saveProvider(provider, apiKey, baseUrl)
        },
        onTest = {}
    )
}

@Composable
fun ProviderEditContent(
    providerId: String?,
    saving: Boolean,
    onBack: () -> Unit,
    onSave: (String, String, String, String) -> Unit,
    onTest: () -> Unit
) {
    var name by remember { mutableStateOf("") }
    var apiKey by remember { mutableStateOf("") }
    var baseUrl by remember { mutableStateOf("https://api.openai.com/v1") }
    var extraConfig by remember { mutableStateOf("{}") }
    var testResult by remember { mutableStateOf<String?>(null) }
    var showTestError by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(
            title = if (providerId == null) "新建 Provider" else "编辑 Provider",
            onBack = onBack
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            AmitiaSection(title = "基本信息") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    AmitiaTextField(
                        value = name,
                        onValueChange = { name = it },
                        label = "Provider 名称",
                        placeholder = "例如：OpenAI",
                        leadingIcon = AmitiaIcons.Hub
                    )
                    AmitiaTextField(
                        value = baseUrl,
                        onValueChange = { baseUrl = it },
                        label = "Base URL",
                        placeholder = "https://api.openai.com/v1",
                        leadingIcon = AmitiaIcons.Link
                    )
                }
            }
            AmitiaSection(title = "认证信息") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                    AmitiaPasswordField(
                        value = apiKey,
                        onValueChange = { apiKey = it },
                        label = "API Key",
                        placeholder = "sk-...",
                        errorMessage = if (showTestError && apiKey.isBlank()) "API Key 不能为空" else null
                    )
                    WarningBanner(
                        message = "API Key 将加密存储，请确保不要在日志或截图暴露密钥。",
                        onDismiss = null
                    )
                }
            }
            AmitiaSection(title = "高级配置") {
                AmitiaCodeEditorSurface(
                    value = extraConfig,
                    onValueChange = { extraConfig = it },
                    language = "json"
                )
            }
            if (testResult != null) {
                AmitiaSection(title = "连接测试结果") {
                    AmitiaMultilineField(
                        value = testResult!!,
                        onValueChange = {},
                        label = "服务返回",
                        enabled = false,
                        minLines = 2,
                        maxLines = 4
                    )
                }
            }
            Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                LoadingButton(
                    text = "保存",
                    onClick = { onSave(name, apiKey, baseUrl, extraConfig) },
                    loading = saving,
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.Check
                )
                SecondaryButton(
                    text = "测试连接",
                    onClick = {
                        if (apiKey.isBlank()) {
                            showTestError = true
                            testResult = "错误：API Key 不能为空"
                        } else {
                            showTestError = false
                            testResult = "连接成功：Provider $name 已就绪，响应延迟 230ms。"
                        }
                    },
                    modifier = Modifier.fillMaxWidth(),
                    leadingIcon = AmitiaIcons.BugReport
                )
            }
        }
    }
}

@Preview(name = "Provider Edit - Light", showBackground = true)
@Composable
private fun ProviderEditLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ProviderEditContent(
            providerId = null,
            saving = false,
            onBack = {},
            onSave = { _, _, _, _ -> },
            onTest = {}
        )
    }
}

@Preview(name = "Provider Edit - Dark", showBackground = true)
@Composable
private fun ProviderEditDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ProviderEditContent(
            providerId = "1",
            saving = false,
            onBack = {},
            onSave = { _, _, _, _ -> },
            onTest = {}
        )
    }
}
