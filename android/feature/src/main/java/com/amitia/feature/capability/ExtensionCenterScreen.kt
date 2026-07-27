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
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIconSize
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.AmitiaTopBar
import com.amitia.core.designsystem.component.InlineLoading

@Composable
fun ExtensionCenterScreen(
    onOpenSystemPlugins: () -> Unit,
    onOpenPublicPlugins: () -> Unit,
    onOpenInstalled: () -> Unit,
    onOpenUpdates: () -> Unit,
    onOpenImport: () -> Unit,
    viewModel: CapabilityViewModel = hiltViewModel()
) {
    val plugins by viewModel.plugins.collectAsStateWithLifecycle()
    val updates by viewModel.updates.collectAsStateWithLifecycle()
    val loading by viewModel.loading.collectAsStateWithLifecycle()
    ExtensionCenterContent(
        systemPluginCount = plugins.count { it.isSystem },
        publicPluginCount = plugins.count { !it.isSystem },
        installedCount = plugins.size,
        updateCount = updates.size,
        loading = loading,
        onOpenSystemPlugins = onOpenSystemPlugins,
        onOpenPublicPlugins = onOpenPublicPlugins,
        onOpenInstalled = onOpenInstalled,
        onOpenUpdates = onOpenUpdates,
        onOpenImport = onOpenImport
    )
}

@Composable
fun ExtensionCenterContent(
    systemPluginCount: Int,
    publicPluginCount: Int,
    installedCount: Int,
    updateCount: Int,
    loading: Boolean,
    onOpenSystemPlugins: () -> Unit,
    onOpenPublicPlugins: () -> Unit,
    onOpenInstalled: () -> Unit,
    onOpenUpdates: () -> Unit,
    onOpenImport: () -> Unit
) {
    Column(modifier = Modifier.fillMaxSize()) {
        AmitiaTopBar(title = "扩展中心")
        if (loading) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                InlineLoading(message = "加载扩展...")
            }
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm),
                verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
            ) {
                item(key = "hint") {
                    Surface(
                        modifier = Modifier.fillMaxWidth(),
                        shape = MaterialTheme.shapes.medium,
                        color = MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Text(
                            text = "扩展中心仅负责发现和管理扩展，不负责角色分配",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(AmitiaSpacing.Base)
                        )
                    }
                }
                item(key = "sections") {
                    Column(verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        ExtensionSectionRow(AmitiaIcons.Security, "系统插件", "$systemPluginCount 个内置插件", onOpenSystemPlugins)
                        ExtensionSectionRow(AmitiaIcons.Public, "公共插件", "$publicPluginCount 个社区插件", onOpenPublicPlugins)
                        ExtensionSectionRow(AmitiaIcons.Extension, "已安装扩展", "$installedCount 个已安装", onOpenInstalled)
                        ExtensionSectionRow(AmitiaIcons.Update, "更新", "$updateCount 个可更新", onOpenUpdates)
                    }
                }
                item(key = "import_header") {
                    AmitiaSectionHeader(title = "扩展包")
                }
                item(key = "import") {
                    ExtensionSectionRow(AmitiaIcons.UploadFile, "导入扩展包", "支持 .amitiax 格式", onOpenImport)
                }
            }
        }
    }
}

@Composable
private fun ExtensionSectionRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit
) {
    AmitiaEntryCard(
        onClick = onClick,
        leading = {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(AmitiaIconSize.Medium)
            )
        },
        title = title,
        subtitle = subtitle
    )
}

@Preview(name = "Extension Center - Light", showBackground = true)
@Composable
private fun ExtensionCenterLightPreview() {
    AmitiaTheme(darkTheme = false) {
        ExtensionCenterContent(
            systemPluginCount = 2,
            publicPluginCount = 2,
            installedCount = 4,
            updateCount = 1,
            loading = false,
            onOpenSystemPlugins = {},
            onOpenPublicPlugins = {},
            onOpenInstalled = {},
            onOpenUpdates = {},
            onOpenImport = {}
        )
    }
}

@Preview(name = "Extension Center - Dark", showBackground = true)
@Composable
private fun ExtensionCenterDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        ExtensionCenterContent(
            systemPluginCount = 0,
            publicPluginCount = 0,
            installedCount = 0,
            updateCount = 0,
            loading = false,
            onOpenSystemPlugins = {},
            onOpenPublicPlugins = {},
            onOpenInstalled = {},
            onOpenUpdates = {},
            onOpenImport = {}
        )
    }
}
