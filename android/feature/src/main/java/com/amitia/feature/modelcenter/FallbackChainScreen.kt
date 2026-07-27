package com.amitia.feature.modelcenter

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.SecondaryButton

@Composable
fun FallbackChainScreen(
    onBack: () -> Unit,
    viewModel: ModelCenterViewModel = hiltViewModel()
) {
    val state by viewModel.fallbackState.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { viewModel.loadFallbackChains() }
    FallbackChainContent(state = state, onBack = onBack, onRetry = viewModel::loadFallbackChains)
}

@Composable
fun FallbackChainContent(
    state: ScreenState<List<FallbackChainUiModel>>,
    onBack: () -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "回退链", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { InlineLoading(message = "正在加载回退链...") }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.RestartAlt,
                title = "暂无回退链",
                description = "请先配置主模型",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(state.data, key = { it.id }) { chain ->
                    FallbackChainCard(chain = chain)
                }
            }
            is ScreenState.Partial -> {}
        }
    }
}

@Composable
private fun FallbackChainCard(chain: FallbackChainUiModel) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
            ) {
                Box(
                    modifier = Modifier.size(40.dp).clip(CircleShape)
                        .background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.RestartAlt,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(20.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = chain.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        text = "主模型: ${chain.primaryModelName}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                AmitiaSwitchRow(
                    title = "",
                    checked = chain.enabled,
                    onCheckedChange = {}
                )
            }
            AmitiaSection(title = "回退顺序") {
                Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                    ) {
                        Box(modifier = Modifier.size(24.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary), contentAlignment = Alignment.Center) {
                            Text("1", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onPrimary)
                        }
                        Text(chain.primaryModelName, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurface)
                        Surface(shape = RoundedCornerShape(8.dp), color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)) {
                            Text("主模型", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onTertiaryContainer, modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp))
                        }
                    }
                    chain.fallbackModels.forEach { fallback ->
                        Row(
                            modifier = Modifier.fillMaxWidth().padding(start = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                        ) {
                            Box(modifier = Modifier.width(2.dp).size(height = 16.dp, width = 2.dp).clip(RoundedCornerShape(1.dp)).background(MaterialTheme.colorScheme.outlineVariant))
                            Box(modifier = Modifier.size(24.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant), contentAlignment = Alignment.Center) {
                                Text("${fallback.order + 1}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Column(modifier = Modifier.weight(1f)) {
                                Text(fallback.modelName, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurface)
                                Text(
                                    text = "触发: ${fallback.triggerErrors.joinToString(", ")}",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f),
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            }
                            Icon(AmitiaIcons.DragHandle, contentDescription = "拖拽排序", tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.5f), modifier = Modifier.size(20.dp))
                        }
                    }
                }
            }
            SecondaryButton(text = "编辑回退链", onClick = {}, modifier = Modifier.fillMaxWidth(), leadingIcon = AmitiaIcons.Edit)
        }
    }
}

@Preview(name = "FallbackChain - Light", showBackground = true)
@Composable
private fun FallbackChainLightPreview() {
    AmitiaTheme(darkTheme = false) {
        FallbackChainContent(
            state = ScreenState.Content(listOf(
                FallbackChainUiModel(
                    id = "1", name = "默认回退链",
                    primaryModelId = "1", primaryModelName = "GPT-4o",
                    fallbackModels = listOf(
                        FallbackModelUiModel("2", "Claude 3.5", listOf("超时", "限流"), 1),
                        FallbackModelUiModel("3", "GPT-3.5", listOf("认证失败"), 2)
                    )
                )
            )),
            onBack = {}, onRetry = {}
        )
    }
}

@Preview(name = "FallbackChain - Dark", showBackground = true)
@Composable
private fun FallbackChainDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        FallbackChainContent(
            state = ScreenState.Content(listOf(
                FallbackChainUiModel(id = "1", name = "默认回退链", primaryModelId = "1", primaryModelName = "GPT-4o")
            )),
            onBack = {}, onRetry = {}
        )
    }
}
