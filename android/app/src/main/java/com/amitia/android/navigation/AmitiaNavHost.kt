package com.amitia.android.navigation

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.amitia.feature.auth.AuthScreen
import com.amitia.feature.auth.AuthViewModel
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
import com.amitia.feature.onboarding.OnboardingScreen
import com.amitia.feature.runtime.RuntimeScreen
import com.amitia.feature.settings.SettingsScreen
import com.amitia.feature.startup.StartupScreen

@Composable
fun AmitiaNavHost(
    navController: NavHostController = rememberNavController(),
    navEventBus: NavEventBus? = null
) {
    if (navEventBus != null) {
        LaunchedEffect(navEventBus) {
            navEventBus.events.collect { event ->
                when (event) {
                    is NavEvent.OpenChat -> {
                        val route = if (event.conversationId != null) {
                            AmitiaRoutes.chatConversation(event.characterId)
                        } else {
                            AmitiaRoutes.chatConversation(event.characterId)
                        }
                        navController.navigate(route) {
                            launchSingleTop = true
                        }
                    }
                    is NavEvent.OpenCharacter -> {
                        navController.navigate(AmitiaRoutes.characterDetail(event.characterId)) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.OpenRuntime -> {
                        navController.navigate(AmitiaRoutes.RUNTIME_MANAGEMENT) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.OpenHome -> {
                        navController.navigate(AmitiaRoutes.HOME) {
                            launchSingleTop = true
                        }
                    }
                    NavEvent.ClearNotifications -> Unit
                }
            }
        }
    }
    Scaffold(
        bottomBar = { AmitiaBottomBar(navController) }
    ) { innerPadding ->
        NavHost(
            navController = navController,
            startDestination = AmitiaRoutes.STARTUP,
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
        ) {
            composable(AmitiaRoutes.STARTUP) {
                StartupScreen(
                    onNavigateOnboarding = {
                        navController.navigate(AmitiaRoutes.ONBOARDING) {
                            popUpTo(AmitiaRoutes.STARTUP) { inclusive = true }
                        }
                    },
                    onNavigateAuth = {
                        navController.navigate(AmitiaRoutes.AUTH) {
                            popUpTo(AmitiaRoutes.STARTUP) { inclusive = true }
                        }
                    },
                    onNavigateHome = {
                        navController.navigate(AmitiaRoutes.HOME) {
                            popUpTo(AmitiaRoutes.STARTUP) { inclusive = true }
                        }
                    }
                )
            }

            composable(AmitiaRoutes.ONBOARDING) {
                OnboardingScreen(
                    onComplete = {
                        navController.navigate(AmitiaRoutes.AUTH) {
                            popUpTo(AmitiaRoutes.ONBOARDING) { inclusive = true }
                        }
                    }
                )
            }

            composable(AmitiaRoutes.AUTH) {
                val viewModel: AuthViewModel = hiltViewModel()
                val state by viewModel.state.collectAsStateWithLifecycle()
                AuthScreen(
                    state = state,
                    onLogin = viewModel::login,
                    onSuccess = {
                        navController.navigate(AmitiaRoutes.HOME) {
                            popUpTo(AmitiaRoutes.AUTH) { inclusive = true }
                        }
                    }
                )
            }

            composable(AmitiaRoutes.HOME) {
                HomeScreen(
                    onOpenRuntime = { navController.navigate(AmitiaRoutes.RUNTIME_MANAGEMENT) },
                    onOpenChat = { characterId ->
                        navController.navigate(AmitiaRoutes.chatConversation(characterId))
                    },
                    onOpenCharacter = { characterId ->
                        navController.navigate(AmitiaRoutes.characterDetail(characterId))
                    }
                )
            }

            composable(AmitiaRoutes.CHAT) {
                ChatScreen(
                    onOpenCharacter = { characterId ->
                        navController.navigate(AmitiaRoutes.characterDetail(characterId))
                    },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(
                route = AmitiaRoutes.CHAT_CONVERSATION,
                arguments = listOf(navArgument("characterId") { type = NavType.StringType })
            ) { backStackEntry ->
                val characterId = backStackEntry.arguments?.getString("characterId").orEmpty()
                ChatScreen(
                    characterId = characterId,
                    onOpenCharacter = { id ->
                        navController.navigate(AmitiaRoutes.characterDetail(id))
                    },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.CHARACTER) {
                CharacterScreen(
                    onOpenDetail = { id ->
                        navController.navigate(AmitiaRoutes.characterDetail(id))
                    },
                    onCreate = {
                        navController.navigate(AmitiaRoutes.CHARACTER_CREATE)
                    }
                )
            }

            composable(
                route = AmitiaRoutes.CHARACTER_DETAIL,
                arguments = listOf(navArgument("characterId") { type = NavType.StringType })
            ) { backStackEntry ->
                val characterId = backStackEntry.arguments?.getString("characterId").orEmpty()
                CharacterDetailScreen(
                    characterId = characterId,
                    onEdit = { navController.navigate(AmitiaRoutes.characterEdit(characterId)) },
                    onChat = {
                        navController.navigate(AmitiaRoutes.chatConversation(characterId))
                    },
                    onMemory = { characterIdArg ->
                        navController.navigate(AmitiaRoutes.MEMORY_CREATE)
                    },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(
                route = AmitiaRoutes.CHARACTER_EDIT,
                arguments = listOf(navArgument("characterId") { type = NavType.StringType })
            ) { backStackEntry ->
                val characterId = backStackEntry.arguments?.getString("characterId").orEmpty()
                CharacterEditScreen(
                    characterId = characterId,
                    onSaved = { navController.popBackStack() },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.CHARACTER_CREATE) {
                CharacterEditScreen(
                    characterId = null,
                    onSaved = { navController.popBackStack() },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.CAPABILITY) {
                CapabilityTabScreen(
                    onOpenModels = { navController.navigate(AmitiaRoutes.MODELS) },
                    onOpenChannels = { navController.navigate(AmitiaRoutes.CHANNELS) },
                    onOpenMemory = { navController.navigate(AmitiaRoutes.MEMORY_CREATE) },
                    onOpenRuntime = { navController.navigate(AmitiaRoutes.RUNTIME_MANAGEMENT) }
                )
            }

            composable(AmitiaRoutes.MEMORY_CREATE) {
                MemoryScreen(
                    onOpenDetail = { id ->
                        navController.navigate(AmitiaRoutes.memoryDetail(id))
                    },
                    onCreate = {
                        navController.navigate(AmitiaRoutes.MEMORY_NEW)
                    }
                )
            }

            composable(AmitiaRoutes.MEMORY_NEW) {
                MemoryEditScreen(
                    memoryId = null,
                    onSaved = { navController.popBackStack() },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(
                route = AmitiaRoutes.MEMORY_DETAIL,
                arguments = listOf(navArgument("memoryId") { type = NavType.StringType })
            ) { backStackEntry ->
                val memoryId = backStackEntry.arguments?.getString("memoryId").orEmpty()
                MemoryDetailScreen(
                    memoryId = memoryId,
                    onEdit = { navController.navigate(AmitiaRoutes.memoryEdit(memoryId)) },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(
                route = AmitiaRoutes.MEMORY_EDIT,
                arguments = listOf(navArgument("memoryId") { type = NavType.StringType })
            ) { backStackEntry ->
                val memoryId = backStackEntry.arguments?.getString("memoryId").orEmpty()
                MemoryEditScreen(
                    memoryId = memoryId,
                    onSaved = { navController.popBackStack() },
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.RUNTIME_MANAGEMENT) {
                RuntimeScreen(
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.MODELS) {
                ModelsScreen(
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.CHANNELS) {
                ChannelsScreen(
                    onBack = { navController.popBackStack() }
                )
            }

            composable(AmitiaRoutes.SETTINGS) {
                SettingsScreen(
                    onOpenRuntime = { navController.navigate(AmitiaRoutes.RUNTIME_MANAGEMENT) },
                    onLogout = {
                        navController.navigate(AmitiaRoutes.AUTH) {
                            popUpTo(0) { inclusive = true }
                        }
                    }
                )
            }
        }
    }
}
