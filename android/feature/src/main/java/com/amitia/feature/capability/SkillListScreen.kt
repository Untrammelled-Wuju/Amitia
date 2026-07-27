package com.amitia.feature.capability

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
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
import com.amitia.core.designsystem.component.AmitiaSegmentedTabs
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

private enum class SkillTab(val label: String) {
    Installed("已安装"), System("系统"), User("用户导入"), Updates("更新")
}

@Composable
fun SkillListScreen(
    onBack: () -> Unit,
    onOpenSkillDetail: (String) -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val skills by viewModel.skills.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    var query by remember { mutableStateOf("") }
    var tab by remember { mutableStateOf(0) }
    val filtered = when (SkillTab.entries[tab]) {
        SkillTab.Installed -> skills
        SkillTab.System -> skills.filter { it.source == SkillSource.System }
        SkillTab.User -> skills.filter { it.source == SkillSource.User }
        SkillTab.Updates -> skills.filter { it.updateAvailable }
    }.filter { query.isBlank() || it.name.contains(query, true) || it.description.contains(query, true) }
    SkillListContent(
        skills = filtered,
        loading = loading,
        tabIndex = tab,
        query = query,
        onBack = onBack,
        onOpenSkillDetail = onOpenSkillDetail,
        onTabSelected = { tab = it },
        onQueryChange = { query = it },
        onClearQuery = { query = "" }
    )
}

@Composable
fun SkillListContent(
    skills: List<SkillInfo>,
    loading: Boolean,
    tabIndex: Int,
    query: String,
    onBack: () -> Unit,
    onOpenSkillDetail: (String) -> Unit,
    onTabSelected: (Int) -> Unit,
    onQueryChange: (String) -> Unit,
    onClearQuery: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "Skill", onBack = onBack)
        AmitiaSegmentedTabs(
            tabs = SkillTab.entries.map { it.label },
            selectedIndex = tabIndex,
            onSelected = onTabSelected,
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base)
        )
        androidx.compose.foundation.layout.Column(
            modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
        ) {
            com.amitia.core.designsystem.component.AmitiaSearchField(
                value = query,
                onValueChange = onQueryChange,
                onClear = onClearQuery,
                placeholder = "搜索 Skill"
            )
        }
        when {
            loading -> Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载 Skill...")
            }
            skills.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.LightbulbOutlined,
                title = "暂无 Skill",
                description = "导入或安装后将在此展示",
                modifier = Modifier.fillMaxSize()
            )
            else -> LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                items(skills, key = { it.id }) { skill ->
                    SkillRow(skill = skill, onClick = { onOpenSkillDetail(skill.id) })
                }
            }
        }
    }
}

@Composable
private fun SkillRow(skill: SkillInfo, onClick: () -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface,
        onClick = onClick
    ) {
        androidx.compose.foundation.layout.Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(CircleShape)
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
                androidx.compose.foundation.layout.Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
                ) {
                    Text(
                        text = skill.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        maxLines = 1, overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "v${skill.version}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    text = skill.description,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2, overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = skill.source.label,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary
                )
            }
            if (skill.updateAvailable) {
                Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.tertiaryContainer) {
                    Text(
                        text = "更新",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onTertiaryContainer,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
        }
    }
}

@Preview(name = "Skill List - Light", showBackground = true)
@Composable
private fun SkillListLightPreview() {
    AmitiaTheme(darkTheme = false) {
        SkillListContent(
            skills = listOf(
                SkillInfo("s1", "意图识别", "识别对话意图", SkillSource.System, "1.0.0", "text", "intent", roles = listOf("艾米")),
                SkillInfo("s3", "代码解释", "解释代码", SkillSource.Community, "2.1.0", "code", "text", updateAvailable = true)
            ),
            loading = false, tabIndex = 0, query = "",
            onBack = {}, onOpenSkillDetail = {}, onTabSelected = {}, onQueryChange = {}, onClearQuery = {}
        )
    }
}

@Preview(name = "Skill List - Dark", showBackground = true)
@Composable
private fun SkillListDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        SkillListContent(
            skills = emptyList(), loading = false, tabIndex = 0, query = "",
            onBack = {}, onOpenSkillDetail = {}, onTabSelected = {}, onQueryChange = {}, onClearQuery = {}
        )
    }
}
