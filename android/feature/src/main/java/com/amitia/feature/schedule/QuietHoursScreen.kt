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
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaSwitchRow
import com.amitia.core.designsystem.component.AmitiaTextField
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.PrimaryButton

@Composable
fun QuietHoursScreen(
    onBack: () -> Unit,
    viewModel: QuietHoursViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    QuietHoursContent(
        state = state,
        onBack = onBack,
        onToggle = viewModel::toggle,
        onUpdate = viewModel::update,
        onRetry = viewModel::load
    )
}

@Composable
fun QuietHoursContent(
    state: ScreenState<QuietHoursData>,
    onBack: () -> Unit,
    onToggle: (String) -> Unit,
    onUpdate: (String, ((QuietHoursConfig) -> QuietHoursConfig)) -> Unit,
    onRetry: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "安静时段", onBack = onBack)
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
                icon = AmitiaIcons.NotificationsOff,
                title = "暂无安静时段",
                description = "点击下方按钮添加第一个安静时段",
                modifier = Modifier.fillMaxSize(),
                primaryAction = { PrimaryButton(text = "新建安静时段", onClick = {}, leadingIcon = AmitiaIcons.Add) }
            )
            is ScreenState.Content -> QuietHoursBody(data = state.data, onToggle = onToggle, onUpdate = onUpdate)
            is ScreenState.Partial -> QuietHoursBody(data = state.data, onToggle = onToggle, onUpdate = onUpdate)
        }
    }
}

@Composable
private fun QuietHoursBody(
    data: QuietHoursData,
    onToggle: (String) -> Unit,
    onUpdate: (String, ((QuietHoursConfig) -> QuietHoursConfig)) -> Unit
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(
            horizontal = AmitiaSpacing.Base,
            vertical = AmitiaSpacing.Sm
        ),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
    ) {
        item { QuietHoursHintCard() }
        item { AmitiaSectionHeader(title = "已配置时段") }
        items(data.items.size) { index ->
            val item = data.items[index]
            QuietHoursCard(
                config = item,
                onToggle = { onToggle(item.id) },
                onUpdate = { transform -> onUpdate(item.id, transform) }
            )
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Sm)) }
        item {
            PrimaryButton(
                text = "新建安静时段",
                onClick = {},
                modifier = Modifier.fillMaxWidth(),
                leadingIcon = AmitiaIcons.Add
            )
        }
        item { Spacer(modifier = Modifier.height(AmitiaSpacing.Lg)) }
    }
}

@Composable
private fun QuietHoursHintCard() {
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
                imageVector = AmitiaIcons.NotificationsOff,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.tertiary,
                modifier = Modifier.size(20.dp)
            )
            Text(
                text = "安静时段内将降低主动消息频率，可按需放行紧急提醒",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onTertiaryContainer
            )
        }
    }
}

@Composable
private fun QuietHoursCard(
    config: QuietHoursConfig,
    onToggle: () -> Unit,
    onUpdate: ((QuietHoursConfig) -> QuietHoursConfig) -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = if (config.enabled) MaterialTheme.colorScheme.surface
        else MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.3f)
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Bedtime,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.size(20.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = config.name,
                        style = MaterialTheme.typography.titleSmall,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "${config.startTime} - ${config.endTime}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    AmitiaTextField(
                        value = config.startTime,
                        onValueChange = { v -> onUpdate { it.copy(startTime = v) } },
                        label = "开始",
                        placeholder = "23:00"
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    AmitiaTextField(
                        value = config.endTime,
                        onValueChange = { v -> onUpdate { it.copy(endTime = v) } },
                        label = "结束",
                        placeholder = "07:00"
                    )
                }
            }
            AmitiaSwitchRow(
                title = "允许紧急提醒",
                checked = config.allowEmergency,
                onCheckedChange = { v -> onUpdate { it.copy(allowEmergency = v) } },
                subtitle = "紧急消息可突破安静时段",
                leadingIcon = AmitiaIcons.Bolt
            )
            AmitiaSwitchRow(
                title = "系统通知例外",
                checked = config.systemNotificationException,
                onCheckedChange = { v -> onUpdate { it.copy(systemNotificationException = v) } },
                subtitle = "系统级通知不受安静时段限制",
                leadingIcon = AmitiaIcons.Notifications
            )
            AllowedRolesRow(
                roles = config.allowedRoles,
                onAddRole = { role -> onUpdate { it.copy(allowedRoles = (it.allowedRoles + role).distinct()) } }
            )
            AmitiaSwitchRow(
                title = "启用该时段",
                checked = config.enabled,
                onCheckedChange = { onToggle() },
                leadingIcon = AmitiaIcons.ToggleOn
            )
        }
    }
}

@Composable
private fun AllowedRolesRow(roles: List<String>, onAddRole: (String) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
        Text(
            text = "允许的角色",
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs),
            verticalAlignment = Alignment.CenterVertically
        ) {
            if (roles.isEmpty()) {
                Surface(
                    shape = RoundedCornerShape(6.dp),
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {
                    Text(
                        text = "未指定（默认全部静音）",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            roles.forEach { role ->
                Surface(
                    shape = RoundedCornerShape(6.dp),
                    color = MaterialTheme.colorScheme.primaryContainer
                ) {
                    Text(
                        text = role,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp)
                    )
                }
            }
            Surface(
                modifier = Modifier.clickable { onAddRole("新角色") },
                shape = RoundedCornerShape(6.dp),
                color = MaterialTheme.colorScheme.surfaceVariant
            ) {
                Row(
                    modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(2.dp)
                ) {
                    Icon(
                        imageVector = AmitiaIcons.Add,
                        contentDescription = "添加角色",
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(12.dp)
                    )
                    Text(
                        text = "添加",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Preview(name = "QuietHours - Light", showBackground = true)
@Composable
private fun QuietHoursLightPreview() {
    AmitiaTheme(darkTheme = false) {
        QuietHoursContent(
            state = ScreenState.Content(QuietHoursData(ScheduleMockData.quietHours)),
            onBack = {}, onToggle = {}, onUpdate = { _, _ -> }, onRetry = {}
        )
    }
}

@Preview(name = "QuietHours - Dark", showBackground = true)
@Composable
private fun QuietHoursDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        QuietHoursContent(
            state = ScreenState.Empty(),
            onBack = {}, onToggle = {}, onUpdate = { _, _ -> }, onRetry = {}
        )
    }
}
