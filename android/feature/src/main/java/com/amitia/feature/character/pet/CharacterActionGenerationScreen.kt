package com.amitia.feature.character.pet

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.AutoAwesome
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
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
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaSlider
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun CharacterActionGenerationScreen(
    onBack: () -> Unit,
    onReview: () -> Unit
) {
    var actionName by remember { mutableStateOf("") }
    var selectedCategory by remember { mutableStateOf("基础") }
    var frameCount by remember { mutableStateOf(8) }
    var fps by remember { mutableStateOf(12) }
    var transparentBg by remember { mutableStateOf(true) }
    var generateVariants by remember { mutableStateOf(false) }
    var isGenerating by remember { mutableStateOf(false) }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "动作生成",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            GenerationInfoCard()
            ActionNameCard(
                actionName = actionName,
                onNameChange = { actionName = it },
                selectedCategory = selectedCategory,
                onCategorySelect = { selectedCategory = it }
            )
            FrameConfigCard(
                frameCount = frameCount,
                onFrameCountChange = { frameCount = it },
                fps = fps,
                onFpsChange = { fps = it }
            )
            AdvancedOptionsCard(
                transparentBg = transparentBg,
                onTransparentBgChange = { transparentBg = it },
                generateVariants = generateVariants,
                onGenerateVariantsChange = { generateVariants = it }
            )
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "取消",
                    onClick = onBack,
                    modifier = Modifier.weight(1f)
                )
                LoadingButton(
                    text = "开始生成",
                    onClick = {
                        isGenerating = true
                        onReview()
                    },
                    loading = isGenerating,
                    leadingIcon = Icons.Outlined.AutoAwesome,
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun GenerationInfoCard() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.tertiary.copy(alpha = 0.2f)),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Outlined.AutoAwesome,
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.tertiary,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "动作生成",
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onTertiaryContainer
                )
                Text(
                    text = "配置参数后生成桌宠动作帧序列",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onTertiaryContainer.copy(alpha = 0.8f)
                )
            }
        }
    }
}

@Composable
private fun ActionNameCard(
    actionName: String,
    onNameChange: (String) -> Unit,
    selectedCategory: String,
    onCategorySelect: (String) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "动作信息",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            AmitiaTextField(
                value = actionName,
                onValueChange = onNameChange,
                label = "动作名称",
                placeholder = "如：挥手"
            )
            Spacer(modifier = Modifier.height(12.dp))
            Text(
                text = "动作分类",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            listOf("基础", "情绪", "动作", "互动反馈", "系统反馈").forEach { category ->
                AmitiaSelectionRow(
                    title = category,
                    selected = selectedCategory == category,
                    onSelect = { onCategorySelect(category) }
                )
            }
        }
    }
}

@Composable
private fun FrameConfigCard(
    frameCount: Int,
    onFrameCountChange: (Int) -> Unit,
    fps: Int,
    onFpsChange: (Int) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "帧配置",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            AmitiaSlider(
                value = frameCount.toFloat(),
                onValueChange = { onFrameCountChange(it.toInt()) },
                valueRange = 4f..12f,
                steps = 7,
                label = "帧数",
                valueFormatter = { "${it.toInt()} 帧" }
            )
            Spacer(modifier = Modifier.height(8.dp))
            AmitiaSlider(
                value = fps.toFloat(),
                onValueChange = { onFpsChange(it.toInt()) },
                valueRange = 6f..24f,
                steps = 5,
                label = "帧率",
                valueFormatter = { "${it.toInt()} FPS" }
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "预计时长：${String.format("%.1f", frameCount.toFloat() / fps)} 秒",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun AdvancedOptionsCard(
    transparentBg: Boolean,
    onTransparentBgChange: (Boolean) -> Unit,
    generateVariants: Boolean,
    onGenerateVariantsChange: (Boolean) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "高级选项",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(12.dp))
            AmitiaSwitchRow(
                title = "透明背景",
                subtitle = "生成透明背景的动作帧",
                checked = transparentBg,
                onCheckedChange = onTransparentBgChange
            )
            Spacer(modifier = Modifier.height(4.dp))
            AmitiaSwitchRow(
                title = "生成变体",
                subtitle = "为每个动作生成多个变体供选择",
                checked = generateVariants,
                onCheckedChange = onGenerateVariantsChange
            )
        }
    }
}

@Preview(name = "ActionGeneration - Light", showBackground = true)
@Composable
private fun CharacterActionGenerationLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            CharacterActionGenerationScreen(
                onBack = {},
                onReview = {}
            )
        }
    }
}

@Preview(name = "ActionGeneration - Dark", showBackground = true)
@Composable
private fun CharacterActionGenerationDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            CharacterActionGenerationScreen(
                onBack = {},
                onReview = {}
            )
        }
    }
}
