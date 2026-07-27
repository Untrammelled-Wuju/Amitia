package com.amitia.feature.schedule

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaErrorState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.TertiaryButton

@Composable
fun LifeTemplateScreen(
    onBack: () -> Unit,
    onEditTemplate: (String) -> Unit,
    viewModel: LifeTemplateViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    LifeTemplateContent(
        state = state,
        onBack = onBack,
        onToggle = viewModel::toggleTemplate,
        onDuplicate = viewModel::duplicateTemplate,
        onEdit = onEditTemplate,
        onRetry = viewModel::load
    )
}

@Composable
fun LifeTemplateContent(
    state: ScreenState<LifeTemplateData>,
    onBack: () -> Unit,
    onToggle: (String) -> Unit,
    onDuplicate: (String) -> Unit,
    onEdit: (String) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "生活模板", onBack = onBack)
        when (state) {
            is ScreenState.Loading -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center
            ) { AmitiaLoadingIndicator() }
            is ScreenState.Error -> AmitiaErrorState(
                icon = AmitiaIcons.CloudOff,
                title = state.error.title,
                description = state.error.message,
                onRetry = onRetry,
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Empty -> AmitiaEmptyState(
                icon = AmitiaIcons.Category,
                title = "暂无生活模板",
                description = "模板加载完成后将在此显示",
                modifier = Modifier.fillMaxSize()
            )
            is ScreenState.Content -> TemplateBody(
                data = state.data,
                onToggle = onToggle,
                onDuplicate = onDuplicate,
                onEdit = onEdit
            )
            is ScreenState.Partial -> TemplateBody(
                data = state.data,
                onToggle = onToggle,
                onDuplicate = onDuplicate,
                onEdit = onEdit
            )
        }
    }
}

@Composable
private fun TemplateBody(
    data: LifeTemplateData,
    onToggle: (String) -> Unit,
    onDuplicate: (String) -> Unit,
    onEdit: (String) -> Unit
) {
    val grouped = data.templates.groupBy { it.category }
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { TemplateHint() }
        LifeTemplateCategory.entries.forEach { category ->
            val items = grouped[category].orEmpty()
            if (items.isNotEmpty()) {
                item { AmitiaSectionHeader(title = category.label) }
                items(items.size) { index ->
                    val template = items[index]
                    TemplateRow(
                        template = template,
                        onToggle = { onToggle(template.id) },
                        onDuplicate = { onDuplicate(template.id) },
                        onEdit = { onEdit(template.id) }
                    )
                }
            }
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Lg)) }
    }
}

@Composable
private fun TemplateHint() {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
    ) {
        Row(
            modifier = Modifier.padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Icon(
                imageVector = AmitiaIcons.Info,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.tertiary,
                modifier = Modifier.size(18.dp)
            )
            Text(
                text = "模板默认全部关闭，可复制后编辑为个人模板",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onTertiaryContainer
            )
        }
    }
}

@Composable
private fun TemplateRow(
    template: LifeTemplate,
    onToggle: () -> Unit,
    onDuplicate: () -> Unit,
    onEdit: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = if (template.enabled) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.5f)
        else MaterialTheme.colorScheme.surface
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { onEdit() }
                .padding(AmitiaSpacing.Base),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
        ) {
            Box(
                modifier = Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(
                        if (template.enabled) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.surfaceVariant
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = templateIcon(template),
                    contentDescription = null,
                    tint = if (template.enabled) MaterialTheme.colorScheme.onPrimary
                    else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp)
                )
            }
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = template.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = "${template.description} · 默认 ${template.defaultTime}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis
                )
            }
            TertiaryButton(text = "复制", onClick = onDuplicate, leadingIcon = AmitiaIcons.ContentCopy)
            androidx.compose.material3.Switch(
                checked = template.enabled,
                onCheckedChange = { onToggle() },
                colors = androidx.compose.material3.SwitchDefaults.colors(
                    checkedThumbColor = MaterialTheme.colorScheme.onPrimary,
                    checkedTrackColor = MaterialTheme.colorScheme.primary,
                    uncheckedThumbColor = MaterialTheme.colorScheme.surface,
                    uncheckedTrackColor = MaterialTheme.colorScheme.surfaceVariant
                )
            )
        }
    }
}

private fun templateIcon(template: LifeTemplate) = when (template.name) {
    "起床" -> AmitiaIcons.WbSunny
    "午饭", "晚饭" -> AmitiaIcons.Restaurant
    "午睡", "睡觉" -> AmitiaIcons.Bedtime
    "上课", "图书馆", "考试周" -> AmitiaIcons.School
    "上班", "加班" -> AmitiaIcons.Work
    "生病" -> AmitiaIcons.MedicalServices
    else -> AmitiaIcons.Category
}

@Preview(name = "LifeTemplate - Light", showBackground = true)
@Composable
private fun LifeTemplateLightPreview() {
    AmitiaTheme(darkTheme = false) {
        LifeTemplateContent(
            state = ScreenState.Content(LifeTemplateData(ScheduleMockData.lifeTemplates)),
            onBack = {}, onToggle = {}, onDuplicate = {}, onEdit = {}, onRetry = {}
        )
    }
}

@Preview(name = "LifeTemplate - Dark", showBackground = true)
@Composable
private fun LifeTemplateDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        LifeTemplateContent(
            state = ScreenState.Empty(),
            onBack = {}, onToggle = {}, onDuplicate = {}, onEdit = {}, onRetry = {}
        )
    }
}
