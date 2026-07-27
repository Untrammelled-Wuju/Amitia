package com.amitia.android.navigation

import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.amitia.android.diagnostics.DiagnosticsActivity
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.feature.channels.ChannelsScreen
import com.amitia.feature.character.CharacterDetailScreen
import com.amitia.feature.character.CharacterEditScreen
import com.amitia.feature.character.CharacterScreen
import com.amitia.feature.chat.ChatScreen
import com.amitia.feature.home.HomeScreen
import com.amitia.feature.memory.MemoryDetailScreen
import com.amitia.feature.memory.MemoryEditScreen
import com.amitia.feature.memory.MemoryScreen
import com.amitia.feature.models.ModelsScreen
import com.amitia.feature.runtime.RuntimeScreen
import com.amitia.feature.settings.SettingsScreen

@Composable
fun AmitiaMainNavHost(
    navController: NavHostController,
    modifier: Modifier = Modifier,
    navEventBus: NavEventBus? = null
) {
    if (navEventBus != null) {
        LaunchedEffect(navEventBus) {
            navEventBus.events.collect { event ->
                when (event) {
                    is NavEvent.OpenChat -> {
                        navController.navigate(AmitiaRoutes.Main.chatConversation(event.characterId)) {
                            launchSingleTop = true
                        }
                    }
                    is NavEvent.OpenCharacter -> {
                        navController.navigate(AmitiaRoutes.Main.characterDetail(event.characterId)) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.OpenRuntime -> {
                        navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.OpenHome -> {
                        navController.navigate(AmitiaRoutes.Main.TODAY) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.ClearNotifications -> Unit
                }
            }
        }
    }

    NavHost(
        navController = navController,
        startDestination = AmitiaRoutes.Main.TODAY,
        modifier = modifier
    ) {
        composable(AmitiaRoutes.Main.TODAY) {
            HomeScreen(
                onOpenRuntime = { navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) },
                onOpenChat = { characterId ->
                    navController.navigate(AmitiaRoutes.Main.chatConversation(characterId))
                },
                onOpenCharacter = { characterId ->
                    navController.navigate(AmitiaRoutes.Main.characterDetail(characterId))
                }
            )
        }

        composable(AmitiaRoutes.Main.CHAT) {
            ChatScreen(
                onOpenCharacter = { id ->
                    navController.navigate(AmitiaRoutes.Main.characterDetail(id))
                },
                onBack = { navController.popBackStack() }
            )
        }

        composable(
            route = AmitiaRoutes.Main.CHAT_CONVERSATION,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
            ChatScreen(
                characterId = characterId,
                onOpenCharacter = { id ->
                    navController.navigate(AmitiaRoutes.Main.characterDetail(id))
                },
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.Main.CHARACTER) {
            CharacterScreen(
                onOpenDetail = { id ->
                    navController.navigate(AmitiaRoutes.Main.characterDetail(id))
                },
                onCreate = {
                    navController.navigate(AmitiaRoutes.Main.CHARACTER_CREATE)
                }
            )
        }

        composable(
            route = AmitiaRoutes.Main.CHARACTER_DETAIL,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
            CharacterDetailScreen(
                characterId = characterId,
                onEdit = { navController.navigate(AmitiaRoutes.Main.characterEdit(characterId)) },
                onChat = {
                    navController.navigate(AmitiaRoutes.Main.chatConversation(characterId))
                },
                onMemory = { _ ->
                    navController.navigate(AmitiaRoutes.Main.MEMORY_NEW)
                },
                onBack = { navController.popBackStack() }
            )
        }

        composable(
            route = AmitiaRoutes.Main.CHARACTER_EDIT,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
            CharacterEditScreen(
                characterId = characterId,
                onSaved = { navController.popBackStack() },
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.Main.CHARACTER_CREATE) {
            CharacterEditScreen(
                characterId = null,
                onSaved = { navController.popBackStack() },
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.Main.MEMORY) {
            MemoryScreen(
                onOpenDetail = { id ->
                    navController.navigate(AmitiaRoutes.Main.memoryDetail(id))
                },
                onCreate = {
                    navController.navigate(AmitiaRoutes.Main.MEMORY_NEW)
                }
            )
        }

        composable(AmitiaRoutes.Main.MEMORY_NEW) {
            MemoryEditScreen(
                memoryId = null,
                onSaved = { navController.popBackStack() },
                onBack = { navController.popBackStack() }
            )
        }

        composable(
            route = AmitiaRoutes.Main.MEMORY_DETAIL,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_MEMORY_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val memoryId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_MEMORY_ID).orEmpty()
            MemoryDetailScreen(
                memoryId = memoryId,
                onEdit = { navController.navigate(AmitiaRoutes.Main.memoryEdit(memoryId)) },
                onBack = { navController.popBackStack() }
            )
        }

        composable(
            route = AmitiaRoutes.Main.MEMORY_EDIT,
            arguments = listOf(navArgument(AmitiaRoutes.KEY_MEMORY_ID) { type = NavType.StringType })
        ) { backStackEntry ->
            val memoryId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_MEMORY_ID).orEmpty()
            MemoryEditScreen(
                memoryId = memoryId,
                onSaved = { navController.popBackStack() },
                onBack = { navController.popBackStack() }
            )
        }

        composable(AmitiaRoutes.Main.MORE) {
            val context = LocalContext.current
            MoreScreen(
                onOpenSettings = { navController.navigate(AmitiaRoutes.Main.SETTINGS) },
                onOpenModels = { navController.navigate(AmitiaRoutes.Main.MODELS) },
                onOpenChannels = { navController.navigate(AmitiaRoutes.Main.CHANNELS) },
                onOpenRuntime = { navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) },
                onOpenDiagnostics = {
                    context.startActivity(Intent(context, DiagnosticsActivity::class.java))
                }
            )
        }

        composable(AmitiaRoutes.Main.SETTINGS) {
            val context = LocalContext.current
            SettingsScreen(
                onOpenRuntime = { navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) },
                onLogout = {
                    val host = context.findActivity()
                    host?.let {
                        it.startActivity(Intent(it, com.amitia.android.bootstrap.BootstrapActivity::class.java))
                        it.finish()
                    }
                }
            )
        }

        composable(AmitiaRoutes.Main.MODELS) {
            ModelsScreen(onBack = { navController.popBackStack() })
        }

        composable(AmitiaRoutes.Main.CHANNELS) {
            ChannelsScreen(onBack = { navController.popBackStack() })
        }

        composable(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) {
            RuntimeScreen(onBack = { navController.popBackStack() })
        }

        composable(AmitiaRoutes.Main.CAPABILITY) {
            CapabilityTabScreen(
                onOpenModels = { navController.navigate(AmitiaRoutes.Main.MODELS) },
                onOpenChannels = { navController.navigate(AmitiaRoutes.Main.CHANNELS) },
                onOpenMemory = { navController.navigate(AmitiaRoutes.Main.MEMORY_NEW) },
                onOpenRuntime = { navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) }
            )
        }
    }
}

@Composable
private fun MoreScreen(
    onOpenSettings: () -> Unit,
    onOpenModels: () -> Unit,
    onOpenChannels: () -> Unit,
    onOpenRuntime: () -> Unit,
    onOpenDiagnostics: () -> Unit
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        Text(
            text = "更多",
            style = MaterialTheme.typography.headlineMedium,
            color = MaterialTheme.colorScheme.onBackground
        )
        MoreEntry(
            icon = AmitiaIcons.SettingsOutlined,
            title = "设置",
            subtitle = "账号、外观、隐私、通知与数据",
            onClick = onOpenSettings
        )
        MoreEntry(
            icon = AmitiaIcons.TuneOutlined,
            title = "模型配置",
            subtitle = "管理 LLM、Embedding、TTS、ASR、视觉、图像生成模型",
            onClick = onOpenModels
        )
        MoreEntry(
            icon = AmitiaIcons.Hub,
            title = "渠道状态",
            subtitle = "微信、QQ、Web 等接入渠道的连接与绑定状态",
            onClick = onOpenChannels
        )
        MoreEntry(
            icon = AmitiaIcons.Sync,
            title = "Runtime 管理",
            subtitle = "本地核心 / 远程模式 / 服务状态 / 诊断导出",
            onClick = onOpenRuntime
        )
        MoreEntry(
            icon = AmitiaIcons.Terminal,
            title = "高级控制台",
            subtitle = "服务、数据库、运行时、钩子、事件、调度、审计与日志",
            onClick = onOpenDiagnostics
        )
    }
}

@Composable
private fun MoreEntry(
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
                contentDescription = title,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(22.dp)
            )
        },
        title = title,
        subtitle = subtitle
    )
}
