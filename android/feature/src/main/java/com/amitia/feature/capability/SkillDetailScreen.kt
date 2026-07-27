package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.background
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
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
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading
import com.amitia.core.designsystem.component.PrimaryButton
import com.amitia.core.designsystem.component.SettingsRow

@Composable
fun SkillDetailScreen(
    skillId: String,
    onBack: () -> Unit,
    onTest: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val skills by viewModel.skills.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    SkillDetailContent(
        skill = skills.firstOrNull { it.id == skillId },
        loading = loading,
        onBack = onBack,
        onTest = onTest
    )
}

@Composable
fun SkillDetailContent(
    skill: SkillInfo?,
    loading: Boolean,
    onBack: () -> Unit,
    onTest: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = skill?.name ?: "Skill 详情", onBack = onBack)
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载详情...")
            }
            skill == null -> AmitiaEmptyState(
                icon = AmitiaIcons.LightbulbOutlined,
                title = "未找到 Skill",
                description = "该 Skill 可能已被移除",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "basic") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Row(
                            modifier = Modifier.padding(AmitiaSpacing.Base),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
                        ) {
                            Box(
                                modifier = Modifier.size(48.dp).clip(CircleShape)
                                    .background(MaterialTheme.colorScheme.primaryContainer),
                                contentAlignment = Alignment.Center
                            ) {
                                Icon(
                                    imageVector = AmitiaIcons.Lightbulb,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.onPrimaryContainer,
                                    modifier = Modifier.size(AmitiaIconSize.Nav)
                                )
                            }
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    text = skill.name,
                                    style = MaterialTheme.typography.titleMedium,
                                    color = MaterialTheme.colorScheme.onSurface
                                )
                                Text(
                                    text = "v${skill.version} · ${skill.source.label}",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
                item(key = "desc_header") { AmitiaSectionHeader(title = "描述") }
                item(key = "desc") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.surface
                    ) {
                        Text(
                            text = skill.description,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurface,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
                item(key = "io_header") { AmitiaSectionHeader(title = "输入输出") }
                item(key = "input") {
                    SchemaRow(label = "输入", schema = skill.inputSchema)
                }
                item(key = "output") {
                    SchemaRow(label = "输出", schema = skill.outputSchema)
                }
                if (skill.declaredMcp.isNotEmpty()) {
                    item(key = "mcp_header") { AmitiaSectionHeader(title = "声明的 MCP") }
                    items(skill.declaredMcp, key = { it }) { mcp ->
                        SettingsRow(
                            title = mcp,
                            subtitle = "依赖的 MCP 服务",
                            leadingIcon = AmitiaIcons.Hub
                        )
                    }
                }
                if (skill.requiredPermissions.isNotEmpty()) {
                    item(key = "perm_header") { AmitiaSectionHeader(title = "所需权限") }
                    items(skill.requiredPermissions, key = { it }) { perm ->
                        SettingsRow(
                            title = perm,
                            leadingIcon = AmitiaIcons.Lock
                        )
                    }
                }
                if (skill.roles.isNotEmpty()) {
                    item(key = "roles_header") { AmitiaSectionHeader(title = "使用角色") }
                    item(key = "roles") {
                        Surface(
                            modifier = Modifier.fillMaxWidth(),
                            shape = MaterialTheme.shapes.medium,
                            color = MaterialTheme.colorScheme.surface
                        ) {
                            Row(
                                modifier = Modifier.padding(AmitiaSpacing.Base),
                                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                            ) {
                                skill.roles.forEach { role ->
                                    Surface(
                                        shape = MaterialTheme.shapes.small,
                                        color = MaterialTheme.colorScheme.primaryContainer
                                    ) {
                                        Text(
                                            text = role,
                                            style = MaterialTheme.typography.labelMedium,
                                            color = MaterialTheme.colorScheme.onPrimaryContainer,
                                            modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
                item(key = "test") {
                    PrimaryButton(
                        text = "测试 Skill",
                        onClick = onTest,
                        leadingIcon = AmitiaIcons.Science,
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            }
        }
    }
}

@Composable
private fun SchemaRow(label: String, schema: String) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceVariant
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium
            )
            Text(
                text = schema,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
    }
}

@Preview(name = "Skill Detail - Light", showBackground = true)
@Composable
private fun SkillDetailLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SkillDetailContent(
            skill = SkillInfo(
                "s2", "情绪分析", "分析用户情绪倾向", SkillSource.User, "0.8.0",
                "text:string", "emotion:string",
                declaredMcp = listOf("emotion-api"),
                requiredPermissions = listOf("网络访问"),
                roles = listOf("艾米", "助手")
            ),
            loading = false, onBack = {}, onTest = {}
        )
    }
}

@Preview(name = "Skill Detail - Dark", showBackground = true)
@Composable
private fun SkillDetailDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SkillDetailContent(
            skill = null, loading = true, onBack = {}, onTest = {}
        )
    }
}
