package com.amitia.feature.character.detail

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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Psychology
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.RecordVoiceOver
import androidx.compose.material.icons.outlined.AccountTree
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.SecondaryButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.feature.character.CharacterDetailViewModel
import com.amitia.feature.character.model.BindingScope
import com.amitia.feature.character.model.ModelBinding
import com.amitia.feature.character.model.ModelBindingConfig

@Composable
fun CharacterModelBindingTab(
    viewModel: CharacterDetailViewModel,
    contentPadding: PaddingValues
) {
    val state by viewModel.modelBindingState.collectAsStateWithLifecycle()
    when (state) {
        ScreenState.Loading -> DetailLoadingBox()
        is ScreenState.Error -> DetailErrorBox(
            message = (state as ScreenState.Error).error.message,
            onRetry = { viewModel.loadModelBinding() }
        )
        is ScreenState.Content -> ModelBindingContent(
            config = (state as ScreenState.Content).data,
            modifier = Modifier.padding(contentPadding)
        )
        else -> DetailEmptyBox("暂无数据")
    }
}

@Composable
private fun ModelBindingContent(
    config: ModelBindingConfig,
    modifier: Modifier = Modifier
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        item(key = "auto_routing") {
            Surface(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(16.dp),
                color = MaterialTheme.colorScheme.surface
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    AmitiaSwitchRow(
                        title = "智能路由",
                        subtitle = "根据任务类型自动选择最合适的模型",
                        checked = config.autoRouting,
                        onCheckedChange = {},
                        leadingIcon = Icons.Outlined.AccountTree
                    )
                }
            }
        }
        config.textModel?.let { binding ->
            item(key = "text_model") {
                ModelBindingCard(
                    title = "文本模型",
                    icon = Icons.Outlined.Psychology,
                    binding = binding
                )
            }
        }
        config.visionModel?.let { binding ->
            item(key = "vision_model") {
                ModelBindingCard(
                    title = "视觉模型",
                    icon = Icons.Outlined.Visibility,
                    binding = binding
                )
            }
        }
        config.voiceModel?.let { binding ->
            item(key = "voice_model") {
                ModelBindingCard(
                    title = "语音模型",
                    icon = Icons.Outlined.RecordVoiceOver,
                    binding = binding
                )
            }
        }
        config.vectorModel?.let { binding ->
            item(key = "vector_model") {
                ModelBindingCard(
                    title = "向量模型",
                    icon = Icons.Outlined.AccountTree,
                    binding = binding
                )
            }
        }
        item(key = "fallback_chain") {
            FallbackChainCard(config.fallbackChain)
        }
        item(key = "actions") {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                SecondaryButton(
                    text = "测试连接",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
                PrimaryButton(
                    text = "保存绑定",
                    onClick = {},
                    modifier = Modifier.weight(1f)
                )
            }
        }
    }
}

@Composable
private fun ModelBindingCard(
    title: String,
    icon: ImageVector,
    binding: ModelBinding
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(CircleShape)
                    .background(
                        if (binding.isActive) MaterialTheme.colorScheme.primaryContainer
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = if (binding.isActive) MaterialTheme.colorScheme.onPrimaryContainer
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(22.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                Text(
                    text = binding.modelName,
                    style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = binding.provider,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Surface(
                    shape = RoundedCornerShape(8.dp),
                    color = if (binding.isActive) MaterialTheme.colorScheme.tertiaryContainer
                    else MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = if (binding.isActive) "激活" else "未激活",
                        style = MaterialTheme.typography.labelSmall,
                        color = if (binding.isActive) MaterialTheme.colorScheme.onTertiaryContainer
                        else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp)
                    )
                }
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = scopeLabel(binding.scope),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.6f)
                )
            }
        }
    }
}

@Composable
private fun FallbackChainCard(chain: List<String>) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "降级链",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "当主模型不可用时，将按顺序尝试以下模型",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
            )
            Spacer(modifier = Modifier.height(12.dp))
            chain.forEachIndexed { index, modelName ->
                Row(
                    modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Box(
                        modifier = Modifier
                            .size(24.dp)
                            .clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = "${index + 1}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer,
                            fontWeight = FontWeight.Medium
                        )
                    }
                    Text(
                        text = modelName,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }
        }
    }
}

private fun scopeLabel(scope: BindingScope): String = when (scope) {
    BindingScope.InheritGlobal -> "继承全局"
    BindingScope.CharacterSpecific -> "角色专属"
}

@Preview(name = "ModelBinding - Light", showBackground = true)
@Composable
private fun CharacterModelBindingLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ModelBindingContent(
                config = ModelBindingConfig(
                    textModel = ModelBinding("1", "GPT-4o", "OpenAI", BindingScope.CharacterSpecific, true),
                    visionModel = ModelBinding("2", "GPT-4o Vision", "OpenAI", BindingScope.InheritGlobal, true),
                    voiceModel = null,
                    vectorModel = null,
                    autoRouting = true,
                    fallbackChain = listOf("GPT-4o", "Claude-3.5")
                )
            )
        }
    }
}

@Preview(name = "ModelBinding - Dark", showBackground = true)
@Composable
private fun CharacterModelBindingDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(color = MaterialTheme.colorScheme.background) {
            ModelBindingContent(
                config = ModelBindingConfig(
                    textModel = null,
                    visionModel = null,
                    voiceModel = null,
                    vectorModel = null,
                    autoRouting = false,
                    fallbackChain = listOf()
                )
            )
        }
    }
}
