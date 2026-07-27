package com.amitia.android.navigation

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import androidx.activity.ComponentActivity
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationRail
import androidx.compose.material3.NavigationRailItem
import androidx.compose.material3.NavigationRailItemDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.windowsizeclass.ExperimentalMaterial3WindowSizeClassApi
import androidx.compose.material3.windowsizeclass.WindowWidthSizeClass
import androidx.compose.material3.windowsizeclass.calculateWindowSizeClass
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.currentBackStackEntryAsState
import com.amitia.core.designsystem.AmitiaGlassSurface
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaNavDimensions
import com.amitia.core.designsystem.AmitiaSpacing
import com.amitia.core.designsystem.GlassLevel

data class AmitiaNavDestination(
    val route: String,
    val label: String,
    val selectedIcon: ImageVector,
    val unselectedIcon: ImageVector
)

val amitiaPrimaryDestinations: List<AmitiaNavDestination> = listOf(
    AmitiaNavDestination(
        route = AmitiaRoutes.Main.TODAY,
        label = "今日",
        selectedIcon = AmitiaIcons.Home,
        unselectedIcon = AmitiaIcons.HomeOutlined
    ),
    AmitiaNavDestination(
        route = AmitiaRoutes.Main.CHAT,
        label = "对话",
        selectedIcon = AmitiaIcons.Chat,
        unselectedIcon = AmitiaIcons.ChatOutlined
    ),
    AmitiaNavDestination(
        route = AmitiaRoutes.Main.CHARACTER,
        label = "角色",
        selectedIcon = AmitiaIcons.Person,
        unselectedIcon = AmitiaIcons.PersonOutlined
    ),
    AmitiaNavDestination(
        route = AmitiaRoutes.Main.MEMORY,
        label = "记忆",
        selectedIcon = AmitiaIcons.Psychology,
        unselectedIcon = AmitiaIcons.Psychology
    ),
    AmitiaNavDestination(
        route = AmitiaRoutes.Main.MORE,
        label = "更多",
        selectedIcon = AmitiaIcons.MoreHoriz,
        unselectedIcon = AmitiaIcons.MoreHoriz
    )
)

enum class AmitiaNavKind { Bottom, Rail, Side }

fun Context.findActivity(): Activity? {
    var context = this
    while (context is ContextWrapper) {
        if (context is Activity) return context
        context = context.baseContext
    }
    return null
}

@OptIn(ExperimentalMaterial3WindowSizeClassApi::class)
@Composable
fun rememberAmitiaNavKind(): AmitiaNavKind {
    val context = LocalContext.current
    val activity = remember(context) { context.findActivity() as? ComponentActivity }
    val windowSizeClass = if (activity != null) {
        calculateWindowSizeClass(activity)
    } else {
        null
    }
    return remember(windowSizeClass) {
        when (windowSizeClass?.widthSizeClass) {
            WindowWidthSizeClass.Medium -> AmitiaNavKind.Rail
            WindowWidthSizeClass.Expanded -> AmitiaNavKind.Side
            else -> AmitiaNavKind.Bottom
        }
    }
}

@Composable
fun AmitiaAdaptiveNavigationContainer(
    navController: NavHostController,
    modifier: Modifier = Modifier,
    content: @Composable (PaddingValues) -> Unit
) {
    val kind = rememberAmitiaNavKind()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route
    val showNavigation = currentRoute in AmitiaRoutes.primaryRoutes

    when (kind) {
        AmitiaNavKind.Bottom -> {
            Box(modifier.fillMaxSize()) {
                val bottomInset = if (showNavigation) {
                    AmitiaNavDimensions.BottomNavHeight +
                        AmitiaNavDimensions.BottomNavSidePadding * 2 +
                        AmitiaNavDimensions.BottomNavTopOffset
                } else {
                    AmitiaSpacing.None
                }
                content(PaddingValues(bottom = bottomInset))
                if (showNavigation) {
                    AmitiaBottomNavigation(
                        navController = navController,
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .fillMaxWidth()
                    )
                }
            }
        }

        AmitiaNavKind.Rail -> {
            Row(modifier.fillMaxSize()) {
                if (showNavigation) {
                    AmitiaNavigationRail(navController)
                }
                Box(Modifier.weight(1f).fillMaxHeight()) {
                    content(PaddingValues(AmitiaSpacing.None))
                }
            }
        }

        AmitiaNavKind.Side -> {
            Row(modifier.fillMaxSize()) {
                if (showNavigation) {
                    AmitiaSideNavigation(navController)
                }
                Box(Modifier.weight(1f).fillMaxHeight()) {
                    content(PaddingValues(AmitiaSpacing.None))
                }
            }
        }
    }
}

@Composable
fun AmitiaNavigationRail(
    navController: NavHostController,
    modifier: Modifier = Modifier
) {
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = backStackEntry?.destination
    NavigationRail(
        modifier = modifier
            .width(AmitiaNavDimensions.NavRailWidth)
            .fillMaxHeight(),
        containerColor = MaterialTheme.colorScheme.surface,
        header = {}
    ) {
        Column(
            modifier = Modifier.fillMaxHeight(),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            amitiaPrimaryDestinations.forEach { item ->
                val selected = currentDestination?.hierarchy?.any { it.route == item.route } == true
                NavigationRailItem(
                    selected = selected,
                    onClick = {
                        navController.navigate(item.route) {
                            popUpTo(navController.graph.findStartDestination().id) {
                                saveState = true
                            }
                            launchSingleTop = true
                            restoreState = true
                        }
                    },
                    icon = {
                        Icon(
                            imageVector = if (selected) item.selectedIcon else item.unselectedIcon,
                            contentDescription = item.label
                        )
                    },
                    label = {
                        Text(item.label, style = MaterialTheme.typography.labelSmall)
                    },
                    colors = NavigationRailItemDefaults.colors(
                        selectedIconColor = MaterialTheme.colorScheme.onPrimary,
                        selectedTextColor = MaterialTheme.colorScheme.primary,
                        indicatorColor = MaterialTheme.colorScheme.primaryContainer,
                        unselectedIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
                        unselectedTextColor = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                )
            }
        }
    }
}

@Composable
fun AmitiaSideNavigation(
    navController: NavHostController,
    modifier: Modifier = Modifier
) {
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = backStackEntry?.destination
    AmitiaGlassSurface(
        level = GlassLevel.Navigation,
        modifier = modifier
            .width(AmitiaNavDimensions.SideNavMaxWidth)
            .fillMaxHeight()
    ) {
        Column(
            modifier = Modifier
                .fillMaxHeight()
                .padding(vertical = AmitiaSpacing.Lg),
            verticalArrangement = Arrangement.spacedBy(AmitiaSpacing.Xs)
        ) {
            Spacer(Modifier.height(AmitiaSpacing.Base))
            amitiaPrimaryDestinations.forEach { item ->
                val selected = currentDestination?.hierarchy?.any { it.route == item.route } == true
                AmitiaSideNavItem(
                    item = item,
                    selected = selected,
                    onClick = {
                        navController.navigate(item.route) {
                            popUpTo(navController.graph.findStartDestination().id) {
                                saveState = true
                            }
                            launchSingleTop = true
                            restoreState = true
                        }
                    }
                )
            }
        }
    }
}

@Composable
private fun AmitiaSideNavItem(
    item: AmitiaNavDestination,
    selected: Boolean,
    onClick: () -> Unit
) {
    val background = if (selected) {
        MaterialTheme.colorScheme.primaryContainer
    } else {
        Color.Transparent
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(AmitiaNavDimensions.BottomNavHeight)
            .padding(horizontal = AmitiaSpacing.Base)
            .clip(MaterialTheme.shapes.large)
            .background(background)
            .clickable(onClick = onClick)
            .padding(horizontal = AmitiaSpacing.Base),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(AmitiaSpacing.Md)
    ) {
        Icon(
            imageVector = if (selected) item.selectedIcon else item.unselectedIcon,
            contentDescription = item.label,
            tint = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = item.label,
            style = MaterialTheme.typography.labelLarge,
            color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
        )
    }
}
