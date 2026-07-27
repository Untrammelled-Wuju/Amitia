package com.amitia.feature.channels

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaColors
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaEmptyState
import com.amitia.core.designsystem.component.AmitiaLoadingIndicator
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.model.ChannelDto

@Composable
fun ChannelsScreen(
    onBack: () -> Unit,
    viewModel: ChannelsViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "渠道",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Medium
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    IconButton(onClick = viewModel::load) {
                        Icon(Icons.Outlined.Refresh, contentDescription = "刷新")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onSurfaceVariant
                )
            )
        }
    ) { padding ->
        when {
            state.loading && state.channels.isEmpty() -> Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) { AmitiaLoadingIndicator() }
            state.channels.isEmpty() -> AmitiaEmptyState(
                icon = AmitiaIcons.Hub,
                title = "尚无渠道",
                description = "通过 Onboarding 或后端配置接入微信/QQ/Web",
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
            )
            else -> LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                if (state.status != null) {
                    item {
                        Text(
                            text = "已绑定 ${state.status!!.totalBound} · 活跃 ${state.status!!.totalActive}",
                            style = MaterialTheme.typography.labelMedium,
                            color = AmitiaColors.OnSurfaceMuted,
                            modifier = Modifier.padding(vertical = 4.dp)
                        )
                    }
                }
                items(state.channels, key = { it.id }) { channel ->
                    ChannelRow(
                        channel = channel,
                        onBind = { viewModel.bind(channel.type, channel.config) },
                        onUnbind = { viewModel.unbind(channel.type, channel.config) }
                    )
                }
            }
        }
    }
}

@Composable
private fun ChannelRow(
    channel: ChannelDto,
    onBind: () -> Unit,
    onUnbind: () -> Unit
) {
    val statusColor = when (channel.status?.lowercase()) {
        "connected", "online", "active" -> AmitiaColors.StateRunning
        "disconnected", "offline", "inactive" -> AmitiaColors.StateIdle
        "error", "failed" -> AmitiaColors.StateFailed
        else -> AmitiaColors.StateIdle
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        color = MaterialTheme.colorScheme.surface
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                AmitiaStatusDot(color = statusColor)
                Spacer(modifier = Modifier.padding(8.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = channel.name,
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurface,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        text = "${channel.type} · ${channel.status ?: "未知"}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                if (channel.enabled) {
                    TextButton(onClick = onUnbind) {
                        Text(text = "解绑", color = MaterialTheme.colorScheme.error)
                    }
                } else {
                    TextButton(onClick = onBind) {
                        Text(text = "绑定")
                    }
                }
            }
            if (channel.lastActiveAt != null) {
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = "最后活跃: ${channel.lastActiveAt}",
                    style = MaterialTheme.typography.labelSmall,
                    color = AmitiaColors.OnSurfaceMuted
                )
            }
        }
    }
}
