package com.amitia.feature.chat

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.LoadingSkeleton
import com.amitia.core.designsystem.component.MessageStatusIndicator

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ToolExecutionScreen(
    toolId: String,
    onBack: () -> Unit,
    viewModel: ChatExtendViewModel = hiltViewModel()
) {
    LaunchedEffect(toolId) { viewModel.loadToolExecution(toolId) }
    val state by viewModel.toolExecutionState.collectAsStateWithLifecycle()

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        TopAppBar(
            title = { Text(text = "工具执行详情", style = MaterialTheme.typography.titleLarge) },
            navigationIcon = {
                AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack)
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background)
        )
        when (val s = state) {
            is ScreenState.Loading -> LoadingSkeleton(lineCount = 5, lineHeight = 48)
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.ErrorOutline,
                title = s.error.title,
                description = s.error.message,
                onRetry = { viewModel.loadToolExecution(toolId) }
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Build,
                title = "工具执行不存在",
                description = "该执行记录可能已被清除"
            )
            is ScreenState.Content, is ScreenState.Partial -> {
                val tool = (s as ScreenState.Content<ToolExecutionDetail>).data
                ToolDetailContent(tool = tool)
            }
        }
    }
}

@Composable
private fun ToolDetailContent(tool: ToolExecutionDetail) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        item(key = "tool_header") {
            AmitiaContentSurface {
                Row(
                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                ) {
                    Box(
                        modifier = Modifier.size(48.dp).clip(CircleShape)
                            .background(MaterialTheme.colorScheme.primaryContainer),
                        contentAlignment = Alignment.Center
                    ) {
                        Icon(
                            imageVector = AmitiaIcons.Terminal,
                            contentDescription = null,
                            tint = MaterialTheme.colorScheme.onPrimaryContainer,
                            modifier = Modifier.size(AmitiaIconSize.Medium)
                        )
                    }
                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = tool.toolName,
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Text(
                            text = tool.purpose,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                    MessageStatusIndicator(status = tool.status)
                }
            }
        }
        item(key = "tool_info") {
            AmitiaSection(title = "执行信息") {
                AmitiaContentSurface {
                    Column {
                        InfoRow(label = "耗时", value = tool.duration)
                        InfoRow(label = "需审批", value = if (tool.requiresApproval) "是" else "否")
                        InfoRow(label = "已审批", value = if (tool.approved) "是" else "否")
                    }
                }
            }
        }
        item(key = "tool_input") {
            AmitiaSection(title = "输入参数") {
                AmitiaContentSurface {
                    Text(
                        text = tool.inputSummary,
                        style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
        }
        item(key = "tool_output") {
            AmitiaSection(title = "执行结果") {
                AmitiaContentSurface {
                    Text(
                        text = tool.outputSummary,
                        style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                        color = MaterialTheme.colorScheme.onSurface,
                        modifier = Modifier.padding(AmitiaSpacing.Base)
                    )
                }
            }
        }
        if (tool.errorMessage != null) {
            item(key = "tool_error") {
                AmitiaSection(title = "错误信息") {
                    AmitiaContentSurface {
                        Text(
                            text = tool.errorMessage,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
            }
        }
        if (tool.sensitiveFields.isNotEmpty()) {
            item(key = "sensitive_fields") {
                AmitiaSection(title = "敏感字段") {
                    AmitiaContentSurface {
                        Column {
                            tool.sensitiveFields.forEach { field ->
                                Row(
                                    modifier = Modifier.fillMaxWidth().padding(AmitiaSpacing.Base),
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Icon(
                                        imageVector = AmitiaIcons.Lock,
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.error,
                                        modifier = Modifier.size(AmitiaIconSize.Medium)
                                    )
                                    Spacer(modifier = Modifier.size(AmitiaSpacing.Sm))
                                    Text(
                                        text = field,
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurface
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.weight(1f))
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

@Preview(name = "Tool Execution - Light", showBackground = true)
@Composable
private fun ToolExecutionLightPreview() {
    AmitiaTheme(darkTheme = false) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            ToolDetailContent(
                tool = ToolExecutionDetail(
                    id = "t1",
                    toolName = "weather_query",
                    purpose = "查询用户所在城市天气",
                    inputSummary = "city: 上海",
                    outputSummary = "上海今日晴，28°C",
                    status = com.amitia.core.designsystem.component.MessageStatus.Sent,
                    duration = "0.3s",
                    approved = true,
                    requiresApproval = false,
                    sensitiveFields = listOf("location")
                )
            )
        }
    }
}

@Preview(name = "Tool Execution - Dark", showBackground = true)
@Composable
private fun ToolExecutionDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            InfoRow(label = "耗时", value = "0.3s")
        }
    }
}
