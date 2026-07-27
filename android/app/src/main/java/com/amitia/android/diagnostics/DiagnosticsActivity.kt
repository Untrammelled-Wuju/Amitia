package com.amitia.android.diagnostics

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.VerticalDivider
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.amitia.android.navigation.AmitiaRoutes
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaTheme
import com.amitia.core.designsystem.AmitiaSpacing
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class DiagnosticsActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            AmitiaTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    DiagnosticsNavHost()
                }
            }
        }
    }
}

private data class DiagnosticsItem(
    val route: String,
    val title: String,
    val icon: ImageVector
)

private val diagnosticsItems: List<DiagnosticsItem> = listOf(
    DiagnosticsItem(AmitiaRoutes.Diagnostics.OVERVIEW, "概览", AmitiaIcons.Dashboard),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.SERVICES, "服务", AmitiaIcons.Hub),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.DATABASES, "数据库", AmitiaIcons.Storage),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.TASK_RUNTIME, "任务运行时", AmitiaIcons.Schedule),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.TRUSTED_SERVICE_RUNTIME, "可信服务运行时", AmitiaIcons.VerifiedUser),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.WASM_RUNTIME, "WASM 运行时", AmitiaIcons.Code),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.HOOKS, "钩子", AmitiaIcons.Webhook),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.EVENTS, "事件", AmitiaIcons.Event),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.SCHEDULES, "调度", AmitiaIcons.AccessTime),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.UI_CONTRIBUTIONS, "UI 贡献", AmitiaIcons.Widgets),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.RESTRICTED_WEB_UI, "受限 Web UI", AmitiaIcons.Lock),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.UPDATES, "更新", AmitiaIcons.Update),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.MIGRATIONS, "迁移", AmitiaIcons.Sync),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.AUDIT, "审计", AmitiaIcons.History),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.PERFORMANCE, "性能", AmitiaIcons.Speed),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.LOGS, "日志", AmitiaIcons.Terminal),
    DiagnosticsItem(AmitiaRoutes.Diagnostics.FEATURE_FLAGS, "特性开关", AmitiaIcons.Flag)
)

@Composable
private fun DiagnosticsNavHost() {
    val navController = rememberNavController()
    Row(modifier = Modifier.fillMaxSize()) {
        DiagnosticsSidebar(
            navController = navController,
            modifier = Modifier
                .width(280.dp)
                .fillMaxHeight()
        )
        VerticalDivider()
        Box(
            modifier = Modifier
                .weight(1f)
                .fillMaxHeight()
        ) {
            NavHost(
                navController = navController,
                startDestination = AmitiaRoutes.Diagnostics.OVERVIEW
            ) {
                diagnosticsItems.forEach { item ->
                    composable(item.route) {
                        DiagnosticsContentScreen(title = item.title)
                    }
                }
            }
        }
    }
}

@Composable
private fun DiagnosticsSidebar(
    navController: NavHostController,
    modifier: Modifier = Modifier
) {
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route
    LazyColumn(
        modifier = modifier,
        contentPadding = androidx.compose.foundation.layout.PaddingValues(vertical = AmitiaSpacing.Base)
    ) {
        item {
            Text(
                text = "高级控制台",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(horizontal = AmitiaSpacing.Base, vertical = AmitiaSpacing.Sm)
            )
        }
        items(diagnosticsItems) { item ->
            val selected = currentRoute == item.route
            DiagnosticsSidebarItem(
                item = item,
                selected = selected,
                onClick = {
                    navController.navigate(item.route) {
                        launchSingleTop = true
                        restoreState = true
                        popUpTo(AmitiaRoutes.Diagnostics.OVERVIEW) {
                            saveState = true
                        }
                    }
                }
            )
        }
    }
}

@Composable
private fun DiagnosticsSidebarItem(
    item: DiagnosticsItem,
    selected: Boolean,
    onClick: () -> Unit
) {
    val background = if (selected) {
        MaterialTheme.colorScheme.primaryContainer
    } else {
        Color.Transparent
    }
    val tint = if (selected) {
        MaterialTheme.colorScheme.primary
    } else {
        MaterialTheme.colorScheme.onSurfaceVariant
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(48.dp)
            .padding(horizontal = AmitiaSpacing.Base)
            .clip(MaterialTheme.shapes.medium)
            .background(background)
            .clickable(onClick = onClick)
            .padding(horizontal = AmitiaSpacing.Md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Icon(
            imageVector = item.icon,
            contentDescription = item.title,
            tint = tint,
            modifier = Modifier.size(20.dp)
        )
        Text(
            text = item.title,
            style = MaterialTheme.typography.bodyMedium,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface
        )
    }
}

@Composable
private fun DiagnosticsContentScreen(title: String) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(AmitiaSpacing.Xxl),
        verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Base)
    ) {
        Text(
            text = title,
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
        Text(
            text = "此面板为 $title 的诊断视图占位，后续将接入实际运行时数据与控制接口。",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
