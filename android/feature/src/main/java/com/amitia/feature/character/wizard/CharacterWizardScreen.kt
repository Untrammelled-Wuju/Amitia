package com.amitia.feature.character.wizard

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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.ArrowForward
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.Save
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.TertiaryButton

enum class WizardStep(
    val title: String,
    val description: String
) {
    Appearance("形象", "设置角色头像和立绘"),
    Name("名字与身份", "填写角色基本信息"),
    Personality("性格", "配置 36 维性格"),
    Voice("声音", "绑定语音模型"),
    Model("模型", "绑定文本与视觉模型"),
    Memory("初始记忆", "设定初始记忆与世界书"),
    Proactive("主动消息", "配置主动消息规则"),
    Channel("渠道", "选择启用的渠道"),
    Permission("权限", "设置角色权限范围"),
    Preview("预览与创建", "确认信息并创建角色")
}

@Composable
fun CharacterWizardScreen(
    onBack: () -> Unit,
    onCreated: (String) -> Unit
) {
    var currentStep by rememberSaveable { mutableIntStateOf(0) }
    var draftName by rememberSaveable { mutableStateOf("") }
    var draftIdentity by rememberSaveable { mutableStateOf("") }
    var draftDescription by rememberSaveable { mutableStateOf("") }
    var showSaveToast by remember { mutableStateOf(false) }

    val steps = WizardStep.entries
    val progress = (currentStep + 1f) / steps.size
    val isLastStep = currentStep == steps.lastIndex

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(
                            text = "创建角色向导",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = "步骤 ${currentStep + 1} / ${steps.size} · ${steps[currentStep].title}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    IconButton(onClick = { showSaveToast = true }) {
                        Icon(Icons.Outlined.Save, contentDescription = "保存草稿")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        },
        bottomBar = {
            WizardBottomBar(
                isFirst = currentStep == 0,
                isLast = isLastStep,
                onPrevious = { if (currentStep > 0) currentStep-- },
                onNext = {
                    if (isLastStep) onCreated("new_character")
                    else currentStep++
                },
                onCreate = { onCreated("new_character") }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
        ) {
            LinearProgressIndicator(
                progress = { progress },
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant
            )
            StepIndicatorRow(
                steps = steps,
                currentStep = currentStep,
                onStepClick = { currentStep = it }
            )
            Box(
                modifier = Modifier
                    .weight(1f)
                    .verticalScroll(rememberScrollState())
                    .padding(20.dp)
            ) {
                WizardStepContent(
                    step = steps[currentStep],
                    draftName = draftName,
                    onDraftNameChange = { draftName = it },
                    draftIdentity = draftIdentity,
                    onDraftIdentityChange = { draftIdentity = it },
                    draftDescription = draftDescription,
                    onDraftDescriptionChange = { draftDescription = it }
                )
            }
        }
        if (showSaveToast) {
            Surface(
                modifier = Modifier
                    .padding(16.dp)
                    .clip(RoundedCornerShape(12.dp)),
                color = MaterialTheme.colorScheme.tertiaryContainer
            ) {
                Text(
                    text = "草稿已保存，下次可继续编辑",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onTertiaryContainer,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
                )
            }
        }
    }
}

@Composable
private fun StepIndicatorRow(
    steps: List<WizardStep>,
    currentStep: Int,
    onStepClick: (Int) -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        steps.forEachIndexed { index, step ->
            val isCompleted = index < currentStep
            val isCurrent = index == currentStep
            Box(
                modifier = Modifier
                    .weight(1f)
                    .height(4.dp)
                    .clip(CircleShape)
                    .background(
                        when {
                            isCurrent -> MaterialTheme.colorScheme.primary
                            isCompleted -> MaterialTheme.colorScheme.primary.copy(alpha = 0.5f)
                            else -> MaterialTheme.colorScheme.surfaceVariant
                        }
                    )
            )
        }
    }
}

@Composable
private fun WizardBottomBar(
    isFirst: Boolean,
    isLast: Boolean,
    onPrevious: () -> Unit,
    onNext: () -> Unit,
    onCreate: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(20.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            if (!isFirst) {
                SecondaryButton(
                    text = "上一步",
                    onClick = onPrevious,
                    leadingIcon = Icons.Outlined.ArrowBack,
                    modifier = Modifier.weight(1f)
                )
            }
            if (isLast) {
                PrimaryButton(
                    text = "创建角色",
                    onClick = onCreate,
                    leadingIcon = Icons.Outlined.Check,
                    modifier = Modifier.weight(1f)
                )
            } else {
                PrimaryButton(
                    text = "下一步",
                    onClick = onNext,
                    leadingIcon = Icons.Outlined.ArrowForward,
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Preview(name = "Wizard - Light", showBackground = true)
@Composable
private fun CharacterWizardLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CharacterWizardScreen(onBack = {}, onCreated = {})
    }
}

@Preview(name = "Wizard - Dark", showBackground = true)
@Composable
private fun CharacterWizardDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CharacterWizardScreen(onBack = {}, onCreated = {})
    }
}
