package com.amitia.feature.settings.permission

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.AmitiaSection
import com.amitia.core.designsystem.component.AmitiaContentSurface
import com.amitia.core.designsystem.component.AmitiaInsetDivider
import com.amitia.core.designsystem.component.AmitiaPageScaffold
import com.amitia.core.designsystem.component.AmitiaStatusDot
import com.amitia.core.designsystem.component.TertiaryButton
import com.amitia.feature.settings.PermissionInfo
import com.amitia.feature.settings.PermissionItem
import com.amitia.feature.settings.SettingsCenterViewModel

@Composable
fun PermissionManagementScreen(
    onBack: () -> Unit,
    viewModel: SettingsCenterViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val permissions = state.permissions

    PermissionManagementScreenContent(
        permissions = permissions,
        onBack = onBack
    )
}

@Composable
private fun PermissionManagementScreenContent(
    permissions: PermissionInfo,
    onBack: () -> Unit
) {
    AmitiaPageScaffold(
        topBar = { AmitiaTopBar(title = "权限管理", onBack = onBack) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            Spacer(modifier = Modifier.height(AmitiaSpacing.Sm))
            AmitiaSection(title = "系统权限") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        permissions.systemPermissions.forEachIndexed { index, perm ->
                            PermissionRow(item = perm)
                            if (index < permissions.systemPermissions.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            AmitiaSection(title = "角色权限") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        permissions.characterPermissions.forEachIndexed { index, perm ->
                            PermissionRow(item = perm)
                            if (index < permissions.characterPermissions.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            AmitiaSection(title = "扩展权限") {
                AmitiaContentSurface(modifier = Modifier.fillMaxWidth()) {
                    Column {
                        permissions.extensionPermissions.forEachIndexed { index, perm ->
                            PermissionRow(item = perm)
                            if (index < permissions.extensionPermissions.lastIndex) {
                                AmitiaInsetDivider(leadingInset = AmitiaSpacing.Base)
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl))
        }
    }
}

@Composable
private fun PermissionRow(item: PermissionItem) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = item.name,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            Text(
                text = item.description,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis
            )
        }
        AmitiaStatusDot(
            color = if (item.granted) AmitiaStateColors.Running else AmitiaStateColors.Failed
        )
        Text(
            text = if (item.granted) "已授权" else "未授权",
            style = MaterialTheme.typography.labelMedium,
            color = if (item.granted) AmitiaStateColors.Running else AmitiaStateColors.Failed
        )
        if (!item.granted) {
            TertiaryButton(text = "授权", onClick = {})
        }
    }
}

@Preview(name = "权限管理页 - 亮色", showBackground = true)
@Composable
private fun PermissionManagementScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PermissionManagementScreenContent(
            permissions = PermissionInfo(),
            onBack = {}
        )
    }
}

@Preview(name = "权限管理页 - 暗色", showBackground = true)
@Composable
private fun PermissionManagementScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PermissionManagementScreenContent(
            permissions = PermissionInfo(),
            onBack = {}
        )
    }
}
