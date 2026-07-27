package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaCodeEditorSurface
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSegmentedTabs
import com.amitia.core.designsystem.component.AmitiaSelectionRow
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.LoadingButton
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.WarningBanner

@Composable
fun CapabilityTestScreen(
    onBack: () -> Unit
) {
    var tabIndex by remember { mutableStateOf(0) }
    var selectedTargetId by remember { mutableStateOf<String?>(null) }
    var input by remember { mutableStateOf("") }
    var running by remember { mutableStateOf(false) }
    var result by remember { mutableStateOf<TestResult?>(null) }
    val targets = sampleTestTargets()
    val selectedTarget = targets.firstOrNull { it.id == selectedTargetId }
    CapabilityTestContent(
        tabIndex = tabIndex,
        targets = targets,
        selectedTargetId = selectedTargetId,
        input = input,
        running = running,
        result = result,
        onBack = onBack,
        onTabSelected = { tabIndex = it; selectedTargetId = null; result = null },
        onTargetSelected = { selectedTargetId = it; result = null },
        onInputChange = { input = it },
        onRun = {
            running = true
            result = null
        }
    )
}

@Composable
fun CapabilityTestContent(
    tabIndex: Int,
    targets: List<CapabilityTestTarget>,
    selectedTargetId: String?,
    input: String,
    running: Boolean,
    result: TestResult?,
    onBack: () -> Unit,
    onTabSelected: (Int) -> Unit,
    onTargetSelected: (String) -> Unit,
    onInputChange: (String) -> Unit,
    onRun: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "能力测试", onBack = onBack)
        AmitiaSegmentedTabs(
            tabs = listOf("Skill", "Plugin", "MCP"),
            selectedIndex = tabIndex,
            onSelected = onTabSelected,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
        )
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            AmitiaSectionHeader(title = "选择测试目标")
            if (targets.isEmpty()) {
                AmitiaEmptyState(
                    icon = AmitiaIcons.Science,
                    title = "暂无可测试目标",
                    modifier = Modifier.fillMaxWidth()
                )
            } else {
                targets.forEach { target ->
                    AmitiaSelectionRow(
                        title = target.name,
                        subtitle = target.description,
                        selected = selectedTargetId == target.id,
                        onSelect = { onTargetSelected(target.id) }
                    )
                }
            }
            if (selectedTargetId != null) {
                AmitiaSectionHeader(title = "测试输入")
                AmitiaCodeEditorSurface(
                    value = input,
                    onValueChange = onInputChange,
                    language = "json",
                    modifier = Modifier.heightIn(min = 120.dp)
                )
                WarningBanner(
                    message = "测试在隔离上下文中执行，结果不会写入角色正式记忆"
                )
                LoadingButton(
                    text = "运行测试",
                    onClick = onRun,
                    loading = running,
                    leadingIcon = AmitiaIcons.PlayArrow,
                    enabled = selectedTargetId != null,
                    modifier = Modifier.fillMaxWidth()
                )
                if (result != null) {
                    AmitiaSectionHeader(title = "测试结果")
                    ResultCard(result = result!!)
                }
            }
        }
    }
}

@Composable
private fun ResultCard(result: TestResult) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = result.target,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Surface(
                    shape = MaterialTheme.shapes.small,
                    color = if (result.success) MaterialTheme.colorScheme.tertiaryContainer
                    else MaterialTheme.colorScheme.errorContainer
                ) {
                    Text(
                        text = if (result.success) "成功" else "失败",
                        style = MaterialTheme.typography.labelSmall,
                        color = if (result.success) MaterialTheme.colorScheme.onTertiaryContainer
                        else MaterialTheme.colorScheme.onErrorContainer,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            Text(
                text = "耗时：${result.duration} · ${result.timestamp}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = "输入",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(top = AmitiaSpacing.Sm)
            )
            Text(
                text = result.input,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 3, overflow = TextOverflow.Ellipsis
            )
            Text(
                text = "输出",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(top = AmitiaSpacing.Xs)
            )
            Text(
                text = result.output,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

private fun sampleTestTargets() = listOf(
    CapabilityTestTarget("s1", "意图识别", CapabilityType.Skill, "识别对话意图"),
    CapabilityTestTarget("pub-1", "天气查询", CapabilityType.Plugin, "查询实时天气"),
    CapabilityTestTarget("m1", "文件搜索服务", CapabilityType.Mcp, "搜索文件")
)

@Preview(name = "Capability Test - Light", showBackground = true)
@Composable
private fun CapabilityTestLightPreview() {
    AmitiaTheme(darkTheme = false) {
        CapabilityTestContent(
            tabIndex = 0,
            targets = sampleTestTargets(),
            selectedTargetId = "s1",
            input = "{\"text\":\"你好\"}",
            running = false,
            result = TestResult("意图识别", "{\"text\":\"你好\"}", "intent: greeting", true, "85ms", "14:30"),
            onBack = {},
            onTabSelected = {},
            onTargetSelected = {},
            onInputChange = {},
            onRun = {}
        )
    }
}

@Preview(name = "Capability Test - Dark", showBackground = true)
@Composable
private fun CapabilityTestDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        CapabilityTestContent(
            tabIndex = 0, targets = emptyList(), selectedTargetId = null,
            input = "", running = false, result = null,
            onBack = {}, onTabSelected = {}, onTargetSelected = {}, onInputChange = {}, onRun = {}
        )
    }
}
