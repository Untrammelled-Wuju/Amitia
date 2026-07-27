package com.amitia.feature.creativeworkshop

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.ScreenState
import com.amitia.core.designsystem.component.AmitiaIconButton
import com.amitia.core.designsystem.component.AmitiaSectionHeader
import com.amitia.core.designsystem.component.LoadingSkeleton

@Composable
fun PermissionScreen(
    projectId: String,
    onBack: () -> Unit,
    viewModel: CreativeWorkshopViewModel = hiltViewModel()
) {
    LaunchedEffect(projectId) { viewModel.loadPermissions(projectId) }
    val state by viewModel.state.collectAsStateWithLifecycle()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("权限声明", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Medium) },
                navigationIcon = { AmitiaIconButton(icon = AmitiaIcons.ArrowBack, contentDescription = "返回", onClick = onBack) },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground
                )
            )
        }
    ) { padding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding).padding(horizontal = AmitiaSpacing.Base),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)
        ) {
            item {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = MaterialTheme.shapes.large,
                    color = MaterialTheme.colorScheme.tertiaryContainer.copy(alpha = 0.3f)
                ) {
                    Row(modifier = Modifier.padding(AmitiaSpacing.Base), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                        Icon(AmitiaIcons.Info, contentDescription = null, tint = MaterialTheme.colorScheme.tertiary, modifier = Modifier.size(20.dp))
                        Text(
                            "遵循最小权限原则，仅声明必要权限",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onTertiaryContainer
                        )
                    }
                }
            }
            when (val ps = state.permissions) {
                is ScreenState.Loading -> item { LoadingSkeleton(lineCount = 4) }
                is ScreenState.Content -> {
                    items(ps.data) { permission ->
                        PermissionItemCard(
                            permission = permission,
                            onToggle = { viewModel.togglePermission(permission.id) }
                        )
                    }
                }
                is ScreenState.Error -> item {
                    Text("加载失败", color = MaterialTheme.colorScheme.error)
                }
                else -> Unit
            }
            item { Spacer(modifier = Modifier.height(AmitiaSpacing.Xxl)) }
        }
    }
}

@Composable
private fun PermissionItemCard(
    permission: CwPermission,
    onToggle: () -> Unit
) {
    val riskColor = when (permission.risk) {
        CwPermissionRisk.Low -> MaterialTheme.colorScheme.tertiary
        CwPermissionRisk.Medium -> MaterialTheme.colorScheme.secondary
        CwPermissionRisk.High -> MaterialTheme.colorScheme.error
    }
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = MaterialTheme.colorScheme.surface,
        onClick = onToggle
    ) {
        Column(modifier = Modifier.padding(AmitiaSpacing.Base), verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Box(
                    modifier = Modifier.size(32.dp).clip(CircleShape).background(
                        if (permission.granted) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.surfaceVariant
                    ),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        if (permission.granted) AmitiaIcons.CheckCircle else AmitiaIcons.Lock,
                        contentDescription = null,
                        tint = if (permission.granted) MaterialTheme.colorScheme.onTertiaryContainer else MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.size(18.dp)
                    )
                }
                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)) {
                        Text(permission.name, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onSurface)
                        if (permission.required) {
                            Surface(shape = MaterialTheme.shapes.small, color = MaterialTheme.colorScheme.errorContainer) {
                                Text("必需", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onErrorContainer, modifier = Modifier.padding(horizontal = AmitiaSpacing.Xs, vertical = 2.dp))
                            }
                        }
                    }
                    Text(permission.description, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Sm)) {
                Surface(shape = MaterialTheme.shapes.small, color = riskColor.copy(alpha = 0.15f)) {
                    Text("风险: ${permission.risk.label}", style = MaterialTheme.typography.labelSmall, color = riskColor, modifier = Modifier.padding(horizontal = AmitiaSpacing.Sm, vertical = 2.dp))
                }
                Text("用途: ${permission.usage}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Preview(name = "Permission - Light", showBackground = true)
@Composable
private fun PermissionScreenLightPreview() {
    AmitiaTheme(darkTheme = false) {
        PermissionScreen(projectId = "1", onBack = {})
    }
}

@Preview(name = "Permission - Dark", showBackground = true)
@Composable
private fun PermissionScreenDarkPreview() {
    AmitiaTheme(darkTheme = true) {
        PermissionScreen(projectId = "1", onBack = {})
    }
}
