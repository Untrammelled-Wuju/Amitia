package com.amitia.feature.character.detail

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Check
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaMultilineField
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.feature.character.CharacterDetailViewModel

@Composable
fun CharacterBasicSettingsTab(
    characterId: String,
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.overviewState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadOverview(characterId) }
        )
        is ScreenState.Content -> {
            val data = (state as ScreenState.Content).data
            BasicSettingsContent(
                initialName = data.name,
                initialIdentity = data.identity,
                modifier = Modifier.padding(contentPadding)
            )
        }
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun BasicSettingsContent(
    initialName: String,
    initialIdentity: String,
    modifier: Modifier = Modifier
) {
    var name by remember { mutableStateOf(initialName) }
    var identity by remember { mutableStateOf(initialIdentity) }
    var description by remember { mutableStateOf("") }
    var callUser by remember { mutableStateOf("") }
    var background by remember { mutableStateOf("") }
    var languageStyle by remember { mutableStateOf("") }
    var showChanges by remember { mutableStateOf(false) }

    val hasChanges = name != initialName || identity != initialIdentity ||
        description.isNotBlank() || callUser.isNotBlank() ||
        background.isNotBlank() || languageStyle.isNotBlank()

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        AmitiaSection(title = "基本信息") {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                AmitiaTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = "名字",
                    placeholder = "角色名称"
                )
                AmitiaTextField(
                    value = identity,
                    onValueChange = { identity = it },
                    label = "身份",
                    placeholder = "如：温柔知性的陪伴助手"
                )
                AmitiaTextField(
                    value = callUser,
                    onValueChange = { callUser = it },
                    label = "称呼用户的方式",
                    placeholder = "如：主人、朋友、你"
                )
            }
        }
        AmitiaSection(title = "详细设定") {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                AmitiaMultilineField(
                    value = description,
                    onValueChange = { description = it },
                    label = "简介",
                    placeholder = "简要描述角色",
                    charLimit = 300
                )
                AmitiaMultilineField(
                    value = background,
                    onValueChange = { background = it },
                    label = "背景设定",
                    placeholder = "角色的背景故事",
                    minLines = 4,
                    charLimit = 1000
                )
                AmitiaMultilineField(
                    value = languageStyle,
                    onValueChange = { languageStyle = it },
                    label = "语言风格",
                    placeholder = "角色的说话风格和习惯",
                    minLines = 2,
                    charLimit = 500
                )
            }
        }
        if (showChanges && hasChanges) {
            ChangeSummaryCard(
                changes = buildList {
                    if (name != initialName) add("名字：$initialName -> $name")
                    if (identity != initialIdentity) add("身份：$initialIdentity -> $identity")
                    if (description.isNotBlank()) add("简介：已填写")
                    if (callUser.isNotBlank()) add("称呼用户：$callUser")
                    if (background.isNotBlank()) add("背景设定：已填写")
                    if (languageStyle.isNotBlank()) add("语言风格：已填写")
                }
            )
        }
        Spacer(modifier = Modifier.height(8.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            SecondaryButton(
                text = "预览变更",
                onClick = { showChanges = !showChanges },
                modifier = Modifier.weight(1f)
            )
            PrimaryButton(
                text = "保存",
                onClick = {},
                leadingIcon = Icons.Outlined.Check,
                enabled = hasChanges,
                modifier = Modifier.weight(1f)
            )
        }
    }
}

@Composable
private fun ChangeSummaryCard(changes: List<String>) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "变更摘要",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onTertiaryContainer,
                fontWeight = FontWeight.Medium
            )
            Spacer(modifier = Modifier.height(8.dp))
            changes.forEach { change ->
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    modifier = Modifier.padding(vertical = 2.dp)
                ) {
                    Text(
                        text = "·",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.tertiary
                    )
                    Text(
                        text = change,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onTertiaryContainer
                    )
                }
            }
        }
    }
}

@Preview(name = "Basic Settings - Light", showBackground = true)
@Composable
private fun CharacterBasicSettingsLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            BasicSettingsContent(
                initialName = "艾米",
                initialIdentity = "温柔知性的陪伴助手"
            )
        }
    }
}

@Preview(name = "Basic Settings - Dark", showBackground = true)
@Composable
private fun CharacterBasicSettingsDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            BasicSettingsContent(
                initialName = "艾米",
                initialIdentity = "温柔知性的陪伴助手"
            )
        }
    }
}
