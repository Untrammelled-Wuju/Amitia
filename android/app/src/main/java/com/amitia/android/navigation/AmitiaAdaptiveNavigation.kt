package com.amitia.android.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.LaunchedEffect
import androidx.navigation.NavHostController
import androidx.navigation.compose.currentBackStackEntryAsState
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.AmitiaSpacing
import androidx.compose.ui.graphics.vector.ImageVector

data class AmitiaNavDestination(
    val route: String,
    val label: String,
    val selectedIcon: ImageVector,
    val unselectedIcon: ImageVector
)

val amitiaPrimaryDestinations: List<AmitiaNavDestination> = listOf(
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

val LocalDrawerState = staticCompositionLocalOf<DrawerState> {
    error("DrawerState not provided")
}

class DrawerState {
    var isOpen by mutableStateOf(false)
        private set

    fun open() { isOpen = true }
    fun close() { isOpen = false }
    fun toggle() { isOpen = !isOpen }
}

@Composable
fun rememberDrawerState(): DrawerState = remember { DrawerState() }

@Composable
fun AmitiaAdaptiveNavigationContainer(
    navController: NavHostController,
    modifier: androidx.compose.ui.Modifier = androidx.compose.ui.Modifier,
    content: @Composable (PaddingValues) -> Unit
) {
    val drawerState = rememberDrawerState()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route

    LaunchedEffect(currentRoute) {
        drawerState.close()
    }

    CompositionLocalProvider(LocalDrawerState provides drawerState) {
        Box(modifier.fillMaxSize()) {
            content(PaddingValues(AmitiaSpacing.None))

            AmitiaAppDrawer(
                navController = navController,
                isOpen = drawerState.isOpen,
                onOpen = { drawerState.open() },
                onClose = { drawerState.close() },
                onOpenProfile = { }
            )
        }
    }
}
