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
import androidx.navigation.NavGraphBuilder
import com.amitia.android.diagnostics.DiagnosticsActivity
import com.amitia.core.designsystem.AmitiaIcons
import com.amitia.core.designsystem.component.AmitiaEntryCard
import com.amitia.feature.chat.ChatDetailScreen
import com.amitia.feature.chat.ChatScreen
import com.amitia.feature.chat.ContextManagementScreen
import com.amitia.feature.chat.ConversationListScreen
import com.amitia.feature.chat.ConversationSettingsScreen
import com.amitia.feature.chat.ExportScreen
import com.amitia.feature.chat.FileDetailScreen
import com.amitia.feature.chat.ImagePreviewScreen
import com.amitia.feature.chat.MediaGalleryScreen
import com.amitia.feature.chat.MemoryReferenceScreen
import com.amitia.feature.chat.MessageDetailScreen
import com.amitia.feature.chat.MessageSearchScreen
import com.amitia.feature.chat.NewConversationScreen
import com.amitia.feature.chat.PromptTraceScreen
import com.amitia.feature.chat.ToolExecutionScreen
import com.amitia.feature.character.CharacterDetailScreen
import com.amitia.feature.character.CharacterEditScreen
import com.amitia.feature.character.CharacterScreen
import com.amitia.feature.character.list.CharacterCreateEntryScreen
import com.amitia.feature.character.list.CharacterListScreen
import com.amitia.feature.character.pet.CharacterActionGenerationScreen
import com.amitia.feature.character.pet.CharacterActionResultReviewScreen
import com.amitia.feature.character.pet.CharacterPetActionLibraryScreen
import com.amitia.feature.character.pet.CharacterPetAssetScreen
import com.amitia.feature.character.wizard.CharacterWizardScreen
import com.amitia.feature.channels.ChannelsScreen
import com.amitia.feature.channel.ChannelCreateScreen
import com.amitia.feature.channel.ChannelDiagnosticScreen
import com.amitia.feature.channel.ChannelEditScreen
import com.amitia.feature.channel.ChannelHomeScreen
import com.amitia.feature.channel.ChannelNotificationSettingsScreen
import com.amitia.feature.channel.ApiChannelDetailScreen
import com.amitia.feature.channel.QQChannelDetailScreen
import com.amitia.feature.channel.WebChannelDetailScreen
import com.amitia.feature.channel.WeChatChannelDetailScreen
import com.amitia.feature.computeruse.ApprovalQueueScreen
import com.amitia.feature.computeruse.ComputerUseHomeScreen
import com.amitia.feature.computeruse.OperationHistoryScreen
import com.amitia.feature.computeruse.PermissionModeScreen
import com.amitia.feature.computeruse.RunningSessionScreen
import com.amitia.feature.computeruse.SafetyRulesScreen
import com.amitia.feature.computeruse.SystemPermissionScreen
import com.amitia.feature.creativeworkshop.BuildScreen
import com.amitia.feature.creativeworkshop.CreativeWorkshopScreen
import com.amitia.feature.creativeworkshop.ManifestEditScreen
import com.amitia.feature.creativeworkshop.NewProjectScreen
import com.amitia.feature.creativeworkshop.PermissionScreen
import com.amitia.feature.creativeworkshop.ProjectDetailScreen
import com.amitia.feature.creativeworkshop.ProjectTestScreen
import com.amitia.feature.creativeworkshop.PublishScreen
import com.amitia.feature.creativeworkshop.SchemaUiEditScreen
import com.amitia.feature.creativeworkshop.SchemaUiPreviewScreen
import com.amitia.feature.emoji.EmojiBatchImportScreen
import com.amitia.feature.emoji.EmojiCenterScreen
import com.amitia.feature.emoji.EmojiDetailEditScreen
import com.amitia.feature.emoji.EmojiGroupDetailScreen
import com.amitia.feature.emoji.EmojiImportResultScreen
import com.amitia.feature.emoji.EmojiScopeScreen
import com.amitia.feature.emoji.EmojiSendStrategyScreen
import com.amitia.feature.home.HomeScreen
import com.amitia.feature.memory.EpisodicMemoryScreen
import com.amitia.feature.memory.ImportFieldMappingScreen
import com.amitia.feature.memory.LongTermMemoryScreen
import com.amitia.feature.memory.MemoryConflictScreen
import com.amitia.feature.memory.MemoryDetailScreen
import com.amitia.feature.memory.MemoryEditScreen
import com.amitia.feature.memory.MemoryExportScreen
import com.amitia.feature.memory.MemoryGraphScreen
import com.amitia.feature.memory.MemoryHomeScreen
import com.amitia.feature.memory.MemoryImportScreen
import com.amitia.feature.memory.MemoryScreen
import com.amitia.feature.memory.MemorySearchScreen
import com.amitia.feature.memory.MemorySettingsScreen
import com.amitia.feature.memory.MemoryTimelineScreen
import com.amitia.feature.memory.PendingMemoryScreen
import com.amitia.feature.memory.WorldBookDetailScreen
import com.amitia.feature.memory.WorldBookEntryEditScreen
import com.amitia.feature.memory.WorldBookListScreen
import com.amitia.feature.modelcenter.FallbackChainScreen
import com.amitia.feature.modelcenter.ModelCenterHomeScreen
import com.amitia.feature.modelcenter.ModelDetailScreen
import com.amitia.feature.modelcenter.ModelDiagnosticScreen
import com.amitia.feature.modelcenter.ModelRoutingScreen
import com.amitia.feature.modelcenter.ModelTestScreen
import com.amitia.feature.modelcenter.ProviderEditScreen
import com.amitia.feature.modelcenter.ProviderListScreen
import com.amitia.feature.modelcenter.TextModelListScreen
import com.amitia.feature.modelcenter.UsageScreen
import com.amitia.feature.modelcenter.VectorModelListScreen
import com.amitia.feature.modelcenter.VisionModelListScreen
import com.amitia.feature.modelcenter.VoiceModelListScreen
import com.amitia.feature.models.ModelsScreen
import com.amitia.feature.runtime.RuntimeScreen
import com.amitia.feature.schedule.LifeTemplateScreen
import com.amitia.feature.schedule.ProactiveMessageWindowScreen
import com.amitia.feature.schedule.QuietHoursScreen
import com.amitia.feature.schedule.ScheduleCalendarScreen
import com.amitia.feature.schedule.ScheduleDetailScreen
import com.amitia.feature.schedule.ScheduleEditScreen
import com.amitia.feature.schedule.ScheduleHomeScreen
import com.amitia.feature.schedule.StateRuleScreen
import com.amitia.feature.settings.SettingsScreen
import com.amitia.feature.settings.about.AboutScreen
import com.amitia.feature.settings.accessibility.AccessibilityScreen
import com.amitia.feature.settings.account.AccountScreen
import com.amitia.feature.settings.appearance.AppearanceScreen
import com.amitia.feature.settings.applock.AppLockScreen
import com.amitia.feature.settings.autostart.AutostartBatteryScreen
import com.amitia.feature.settings.backup.BackupRestoreScreen
import com.amitia.feature.settings.center.SettingsCenterScreen
import com.amitia.feature.settings.crashrecovery.CrashRecoveryScreen
import com.amitia.feature.settings.data.DataStorageScreen
import com.amitia.feature.settings.developer.DeveloperOptionsScreen
import com.amitia.feature.settings.feedback.FeedbackScreen
import com.amitia.feature.settings.importexport.ImportExportScreen
import com.amitia.feature.settings.language.LanguageScreen
import com.amitia.feature.settings.licenses.OpenSourceLicensesScreen
import com.amitia.feature.settings.localruntime.LocalRuntimeScreen
import com.amitia.feature.settings.network.NetworkProxyScreen
import com.amitia.feature.settings.notification.NotificationSettingsScreen
import com.amitia.feature.settings.permission.PermissionManagementScreen
import com.amitia.feature.settings.privacy.PrivacyScreen
import com.amitia.feature.settings.runmode.RunModeScreen
import com.amitia.feature.settings.security.SecurityScreen
import com.amitia.feature.settings.update.UpdateScreen
import com.amitia.feature.today.ActivityFeedScreen
import com.amitia.feature.today.GlobalSearchScreen
import com.amitia.feature.today.NotificationCenterScreen
import com.amitia.feature.today.RuntimeIssuesScreen
import com.amitia.feature.today.TodayDetailScreen
import com.amitia.feature.today.TodayHomeScreen
import com.amitia.feature.voicecenter.AudioDiagnosticScreen as VoiceCenterAudioDiagnosticScreen
import com.amitia.feature.voicecenter.SttSettingsScreen
import com.amitia.feature.voicecenter.TtsSettingsScreen
import com.amitia.feature.voicecenter.VoiceCenterHomeScreen
import com.amitia.feature.voicecenter.VoiceCloneScreen
import com.amitia.feature.voicecenter.VoiceLibraryScreen
import com.amitia.feature.capability.CapabilityHomeScreen
import com.amitia.feature.capability.CapabilityTestScreen
import com.amitia.feature.capability.ExtensionCenterScreen
import com.amitia.feature.capability.ExtensionImportScreen
import com.amitia.feature.capability.ExtensionLogScreen
import com.amitia.feature.capability.ExtensionPermissionReviewScreen
import com.amitia.feature.capability.ExtensionUpdateScreen
import com.amitia.feature.capability.McpCreateScreen
import com.amitia.feature.capability.McpDetailScreen
import com.amitia.feature.capability.McpListScreen
import com.amitia.feature.capability.PluginDetailScreen
import com.amitia.feature.capability.PublicPluginListScreen
import com.amitia.feature.capability.SkillDetailScreen
import com.amitia.feature.capability.SkillListScreen
import com.amitia.feature.capability.SystemPluginListScreen

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
                        navController.navigate(AmitiaRoutes.Main.CHAT) {
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
        startDestination = AmitiaRoutes.Main.CHAT,
        modifier = modifier
    ) {
        addTodayRoutes(navController)
        addChatRoutes(navController)
        addCharacterRoutes(navController)
        addMemoryRoutes(navController)
        addEmojiRoutes(navController)
        addScheduleRoutes(navController)
        addChannelRoutes(navController)
        addModelCenterRoutes(navController)
        addVoiceCenterRoutes(navController)
        addCapabilityRoutes(navController)
        addComputerUseRoutes(navController)
        addSettingsRoutes(navController)
        addCreativeWorkshopRoutes(navController)
        addMoreRoutes(navController)
    }
}

private fun NavGraphBuilder.addTodayRoutes(navController: NavHostController) {
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

    composable(AmitiaRoutes.Main.TODAY_DETAIL) {
        TodayDetailScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.ACTIVITY_FEED) {
        ActivityFeedScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.NOTIFICATION_CENTER) {
        NotificationCenterScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.GLOBAL_SEARCH) {
        GlobalSearchScreen(
            onBack = { navController.popBackStack() },
            onOpenResult = { _ -> navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.RUNTIME_ISSUES) {
        val context = LocalContext.current
        RuntimeIssuesScreen(
            onBack = { navController.popBackStack() },
            onOpenDiagnostics = {
                context.startActivity(Intent(context, DiagnosticsActivity::class.java))
            }
        )
    }
}

private fun NavGraphBuilder.addChatRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.CHAT) {
        val drawerState = LocalDrawerState.current
        ChatScreen(
            onOpenCharacter = { id ->
                navController.navigate(AmitiaRoutes.Main.characterDetail(id))
            },
            onBack = { navController.popBackStack() },
            onMenu = { drawerState.open() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CHAT_CONVERSATION,
        arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
    ) { backStackEntry ->
        val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
        val drawerState = LocalDrawerState.current
        ChatScreen(
            characterId = characterId,
            onOpenCharacter = { id ->
                navController.navigate(AmitiaRoutes.Main.characterDetail(id))
            },
            onBack = { navController.popBackStack() },
            onMenu = { drawerState.open() }
        )
    }

    composable(AmitiaRoutes.Main.NEW_CONVERSATION) {
        NewConversationScreen(
            onBack = { navController.popBackStack() },
            onCreated = { conversationId ->
                navController.navigate(AmitiaRoutes.Main.chatDetail(conversationId)) {
                    popUpTo(AmitiaRoutes.Main.CHAT) { inclusive = false }
                }
            }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CHAT_DETAIL,
        arguments = listOf(navArgument("conversationId") { type = NavType.StringType })
    ) { backStackEntry ->
        val conversationId = backStackEntry.arguments?.getString("conversationId").orEmpty()
        AmitiaPlaceholderScreen(title = "对话详情 $conversationId", onBack = { navController.popBackStack() })
    }

    composable(
        route = AmitiaRoutes.Main.IMAGE_PREVIEW,
        arguments = listOf(navArgument("imageId") { type = NavType.StringType })
    ) { backStackEntry ->
        val imageId = backStackEntry.arguments?.getString("imageId").orEmpty()
        ImagePreviewScreen(
            imageUrl = imageId,
            title = "图片预览",
            onBack = { navController.popBackStack() },
            onSave = {},
            onShare = {},
            onViewOriginal = {}
        )
    }

    composable(
        route = AmitiaRoutes.Main.FILE_DETAIL,
        arguments = listOf(navArgument("fileId") { type = NavType.StringType })
    ) { backStackEntry ->
        val fileId = backStackEntry.arguments?.getString("fileId").orEmpty()
        FileDetailScreen(
            fileId = fileId,
            onBack = { navController.popBackStack() },
            onDownload = {},
            onShare = {},
            onReUpload = {}
        )
    }

    composable(
        route = AmitiaRoutes.Main.MESSAGE_DETAIL,
        arguments = listOf(navArgument("messageId") { type = NavType.StringType })
    ) { backStackEntry ->
        val messageId = backStackEntry.arguments?.getString("messageId").orEmpty()
        MessageDetailScreen(
            messageId = messageId,
            onBack = { navController.popBackStack() },
            onViewTool = { toolId -> navController.navigate(AmitiaRoutes.Main.toolExecution(toolId)) },
            onViewMemory = { memoryId -> navController.navigate(AmitiaRoutes.Main.memoryDetail(memoryId)) }
        )
    }

    composable(AmitiaRoutes.Main.MESSAGE_SEARCH) {
        MessageSearchScreen(
            onBack = { navController.popBackStack() },
            onOpenMessage = { msgId -> navController.navigate(AmitiaRoutes.Main.messageDetail(msgId)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.MEDIA_GALLERY,
        arguments = listOf(navArgument("conversationId") { type = NavType.StringType })
    ) { backStackEntry ->
        val conversationId = backStackEntry.arguments?.getString("conversationId").orEmpty()
        MediaGalleryScreen(
            conversationId = conversationId,
            onBack = { navController.popBackStack() },
            onOpenImage = { imgId -> navController.navigate(AmitiaRoutes.Main.imagePreview(imgId)) },
            onOpenFile = { fId -> navController.navigate(AmitiaRoutes.Main.fileDetail(fId)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.TOOL_EXECUTION,
        arguments = listOf(navArgument("executionId") { type = NavType.StringType })
    ) { backStackEntry ->
        val executionId = backStackEntry.arguments?.getString("executionId").orEmpty()
        ToolExecutionScreen(
            toolId = executionId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CONTEXT_MANAGEMENT,
        arguments = listOf(navArgument("conversationId") { type = NavType.StringType })
    ) { backStackEntry ->
        val conversationId = backStackEntry.arguments?.getString("conversationId").orEmpty()
        ContextManagementScreen(
            conversationId = conversationId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.MEMORY_REFERENCE,
        arguments = listOf(navArgument("messageId") { type = NavType.StringType })
    ) { backStackEntry ->
        val messageId = backStackEntry.arguments?.getString("messageId").orEmpty()
        MemoryReferenceScreen(
            messageId = messageId,
            onBack = { navController.popBackStack() },
            onOpenMemory = { memoryId -> navController.navigate(AmitiaRoutes.Main.memoryDetail(memoryId)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CONVERSATION_EXPORT,
        arguments = listOf(navArgument("conversationId") { type = NavType.StringType })
    ) { backStackEntry ->
        ExportScreen(
            onBack = { navController.popBackStack() },
            onExport = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CONVERSATION_SETTINGS,
        arguments = listOf(navArgument("conversationId") { type = NavType.StringType })
    ) { backStackEntry ->
        val conversationId = backStackEntry.arguments?.getString("conversationId").orEmpty()
        ConversationSettingsScreen(
            conversationId = conversationId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.PROMPT_TRACE,
        arguments = listOf(navArgument("messageId") { type = NavType.StringType })
    ) { backStackEntry ->
        val messageId = backStackEntry.arguments?.getString("messageId").orEmpty()
        PromptTraceScreen(
            conversationId = messageId,
            onBack = { navController.popBackStack() }
        )
    }
}

private fun NavGraphBuilder.addCharacterRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.CHARACTER) {
        val drawerState = LocalDrawerState.current
        CharacterScreen(
            onOpenDetail = { id ->
                navController.navigate(AmitiaRoutes.Main.characterDetail(id))
            },
            onCreate = {
                navController.navigate(AmitiaRoutes.Main.CHARACTER_CREATE)
            },
            onMenu = { drawerState.open() }
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
        CharacterCreateEntryScreen(
            onBack = { navController.popBackStack() },
            onSelectBlank = {
                navController.navigate(AmitiaRoutes.Main.CHARACTER_WIZARD) {
                    popUpTo(AmitiaRoutes.Main.CHARACTER_CREATE) { inclusive = true }
                }
            },
            onSelectTemplate = {
                navController.navigate(AmitiaRoutes.Main.CHARACTER_WIZARD) {
                    popUpTo(AmitiaRoutes.Main.CHARACTER_CREATE) { inclusive = true }
                }
            },
            onSelectImport = {
                navController.navigate(AmitiaRoutes.Main.CHARACTER_WIZARD) {
                    popUpTo(AmitiaRoutes.Main.CHARACTER_CREATE) { inclusive = true }
                }
            },
            onSelectImage = {
                navController.navigate(AmitiaRoutes.Main.CHARACTER_WIZARD) {
                    popUpTo(AmitiaRoutes.Main.CHARACTER_CREATE) { inclusive = true }
                }
            }
        )
    }

    composable(AmitiaRoutes.Main.CHARACTER_WIZARD) {
        CharacterWizardScreen(
            onBack = { navController.popBackStack() },
            onCreated = { characterId ->
                navController.navigate(AmitiaRoutes.Main.characterDetail(characterId)) {
                    popUpTo(AmitiaRoutes.Main.CHARACTER) { inclusive = false }
                }
            }
        )
    }

    composable(
        route = AmitiaRoutes.Main.PET_ASSET,
        arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
    ) { backStackEntry ->
        val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
        CharacterPetAssetScreen(
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.PET_ACTION_LIBRARY,
        arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
    ) { backStackEntry ->
        val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
        CharacterPetActionLibraryScreen(
            onBack = { navController.popBackStack() },
            onGenerate = {
                navController.navigate(AmitiaRoutes.Main.actionGeneration(characterId))
            }
        )
    }

    composable(
        route = AmitiaRoutes.Main.ACTION_GENERATION,
        arguments = listOf(navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType })
    ) { backStackEntry ->
        val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
        CharacterActionGenerationScreen(
            onBack = { navController.popBackStack() },
            onReview = {
                navController.navigate(AmitiaRoutes.Main.actionReview(characterId, "default"))
            }
        )
    }

    composable(
        route = AmitiaRoutes.Main.ACTION_REVIEW,
        arguments = listOf(
            navArgument(AmitiaRoutes.KEY_CHARACTER_ID) { type = NavType.StringType },
            navArgument("actionId") { type = NavType.StringType }
        )
    ) { backStackEntry ->
        val characterId = backStackEntry.arguments?.getString(AmitiaRoutes.KEY_CHARACTER_ID).orEmpty()
        val actionId = backStackEntry.arguments?.getString("actionId").orEmpty()
        CharacterActionResultReviewScreen(
            onBack = { navController.popBackStack() },
            onApprove = { navController.popBackStack() }
        )
    }
}

private fun NavGraphBuilder.addMemoryRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.MEMORY) {
        val drawerState = LocalDrawerState.current
        MemoryHomeScreen(
            onSearch = { navController.navigate(AmitiaRoutes.Main.MEMORY_SEARCH) },
            onTimeline = { navController.navigate(AmitiaRoutes.Main.MEMORY_TIMELINE) },
            onLongTerm = { navController.navigate(AmitiaRoutes.Main.LONG_TERM_MEMORY) },
            onWorldBook = { navController.navigate(AmitiaRoutes.Main.WORLD_BOOK_LIST) },
            onGraph = { navController.navigate(AmitiaRoutes.Main.MEMORY_GRAPH) },
            onPending = { navController.navigate(AmitiaRoutes.Main.PENDING_MEMORY) },
            onMemoryDetail = { id -> navController.navigate(AmitiaRoutes.Main.memoryDetail(id)) },
            onMenu = { drawerState.open() },
            onCreate = { navController.navigate(AmitiaRoutes.Main.MEMORY_NEW) }
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

    composable(AmitiaRoutes.Main.MEMORY_TIMELINE) {
        MemoryTimelineScreen(
            onBack = { navController.popBackStack() },
            onMemoryDetail = { id -> navController.navigate(AmitiaRoutes.Main.memoryDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.MEMORY_SEARCH) {
        MemorySearchScreen(
            onBack = { navController.popBackStack() },
            onMemoryDetail = { id -> navController.navigate(AmitiaRoutes.Main.memoryDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.LONG_TERM_MEMORY) {
        LongTermMemoryScreen(
            onBack = { navController.popBackStack() },
            onMemoryDetail = { id -> navController.navigate(AmitiaRoutes.Main.memoryDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.EPISODIC_MEMORY) {
        EpisodicMemoryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.WORLD_BOOK_LIST) {
        WorldBookListScreen(
            onBack = { navController.popBackStack() },
            onOpenDetail = { id -> navController.navigate(AmitiaRoutes.Main.worldBookDetail(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.WORLD_BOOK_DETAIL,
        arguments = listOf(navArgument("bookId") { type = NavType.StringType })
    ) { backStackEntry ->
        val bookId = backStackEntry.arguments?.getString("bookId").orEmpty()
        WorldBookDetailScreen(
            bookId = bookId,
            onBack = { navController.popBackStack() },
            onEditEntry = { entryId -> navController.navigate(AmitiaRoutes.Main.worldBookEntryEdit(bookId, entryId)) },
            onAddEntry = { navController.navigate(AmitiaRoutes.Main.worldBookEntryEdit(bookId, "")) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.WORLD_BOOK_ENTRY_EDIT,
        arguments = listOf(
            navArgument("bookId") { type = NavType.StringType },
            navArgument("entryId") { type = NavType.StringType }
        )
    ) { backStackEntry ->
        val entryId = backStackEntry.arguments?.getString("entryId").orEmpty()
        WorldBookEntryEditScreen(
            entryId = entryId.ifEmpty { null },
            onBack = { navController.popBackStack() },
            onSave = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.MEMORY_GRAPH) {
        MemoryGraphScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.PENDING_MEMORY) {
        PendingMemoryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.MEMORY_CONFLICT) {
        MemoryConflictScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.MEMORY_IMPORT) {
        MemoryImportScreen(
            onBack = { navController.popBackStack() },
            onProceedToMapping = { navController.navigate(AmitiaRoutes.Main.IMPORT_FIELD_MAPPING) }
        )
    }

    composable(AmitiaRoutes.Main.IMPORT_FIELD_MAPPING) {
        ImportFieldMappingScreen(
            onBack = { navController.popBackStack() },
            onImport = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.MEMORY_EXPORT) {
        MemoryExportScreen(
            onBack = { navController.popBackStack() },
            onExport = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.MEMORY_SETTINGS) {
        MemorySettingsScreen(onBack = { navController.popBackStack() })
    }
}

private fun NavGraphBuilder.addEmojiRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.EMOJI_CENTER) {
        EmojiCenterScreen(
            onBack = { navController.popBackStack() },
            onOpenGroup = { groupId -> navController.navigate("main/emoji_group_detail/$groupId") },
            onBatchImport = { navController.navigate(AmitiaRoutes.Main.EMOJI_BATCH_IMPORT) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.EMOJI_GROUP_DETAIL,
        arguments = listOf(navArgument("groupId") { type = NavType.StringType })
    ) { backStackEntry ->
        val groupId = backStackEntry.arguments?.getString("groupId").orEmpty()
        EmojiGroupDetailScreen(
            groupId = groupId,
            onBack = { navController.popBackStack() },
            onEditEmoji = { emojiId -> navController.navigate("main/emoji_detail_edit/$emojiId") }
        )
    }

    composable(AmitiaRoutes.Main.EMOJI_BATCH_IMPORT) {
        EmojiBatchImportScreen(
            onBack = { navController.popBackStack() },
            onImport = { navController.navigate(AmitiaRoutes.Main.EMOJI_IMPORT_RESULT) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.EMOJI_DETAIL_EDIT,
        arguments = listOf(navArgument("groupId") { type = NavType.StringType })
    ) { backStackEntry ->
        val groupId = backStackEntry.arguments?.getString("groupId").orEmpty()
        EmojiDetailEditScreen(
            emojiId = groupId,
            onBack = { navController.popBackStack() },
            onSave = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.EMOJI_SCOPE,
        arguments = listOf(navArgument("groupId") { type = NavType.StringType })
    ) { backStackEntry ->
        val groupId = backStackEntry.arguments?.getString("groupId").orEmpty()
        EmojiScopeScreen(
            onBack = { navController.popBackStack() },
            onSave = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.EMOJI_SEND_STRATEGY,
        arguments = listOf(navArgument("groupId") { type = NavType.StringType })
    ) { backStackEntry ->
        val groupId = backStackEntry.arguments?.getString("groupId").orEmpty()
        EmojiSendStrategyScreen(
            onBack = { navController.popBackStack() },
            onSave = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.EMOJI_IMPORT_RESULT) {
        EmojiImportResultScreen(
            onBack = { navController.popBackStack() },
            onComplete = { navController.popBackStack() },
            onFixMeaning = { }
        )
    }
}

private fun NavGraphBuilder.addScheduleRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.SCHEDULE) {
        val drawerState = LocalDrawerState.current
        ScheduleHomeScreen(
            onBack = { navController.popBackStack() },
            onNewSchedule = { navController.navigate(AmitiaRoutes.Main.SCHEDULE_EDIT) },
            onOpenCalendar = { navController.navigate(AmitiaRoutes.Main.SCHEDULE_CALENDAR) },
            onOpenDetail = { id -> navController.navigate(AmitiaRoutes.Main.scheduleDetail(id)) },
            onOpenProactiveWindow = { navController.navigate(AmitiaRoutes.Main.PROACTIVE_TIME_WINDOW) },
            onMenu = { drawerState.open() }
        )
    }

    composable(AmitiaRoutes.Main.SCHEDULE_CALENDAR) {
        ScheduleCalendarScreen(
            onBack = { navController.popBackStack() },
            onOpenDetail = { id -> navController.navigate(AmitiaRoutes.Main.scheduleDetail(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.SCHEDULE_DETAIL,
        arguments = listOf(navArgument("scheduleId") { type = NavType.StringType })
    ) { backStackEntry ->
        val scheduleId = backStackEntry.arguments?.getString("scheduleId").orEmpty()
        ScheduleDetailScreen(
            scheduleId = scheduleId,
            onBack = { navController.popBackStack() },
            onEdit = { id -> navController.navigate(AmitiaRoutes.Main.SCHEDULE_EDIT) }
        )
    }

    composable(AmitiaRoutes.Main.SCHEDULE_EDIT) {
        ScheduleEditScreen(
            scheduleId = null,
            onBack = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.LIFE_TEMPLATE) {
        LifeTemplateScreen(
            onBack = { navController.popBackStack() },
            onEditTemplate = { _ -> }
        )
    }

    composable(AmitiaRoutes.Main.PROACTIVE_TIME_WINDOW) {
        ProactiveMessageWindowScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.STATUS_RULE) {
        StateRuleScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.QUIET_HOURS) {
        QuietHoursScreen(onBack = { navController.popBackStack() })
    }
}

private fun NavGraphBuilder.addChannelRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.CHANNEL_CENTER) {
        ChannelHomeScreen(
            onBack = { navController.popBackStack() },
            onOpenChannel = { _ -> navController.navigate(AmitiaRoutes.Main.CHANNEL_NEW) },
            onCreateChannel = { navController.navigate(AmitiaRoutes.Main.CHANNEL_NEW) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CHANNEL_DETAIL,
        arguments = listOf(navArgument("channelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val channelId = backStackEntry.arguments?.getString("channelId").orEmpty()
        AmitiaPlaceholderScreen(title = "渠道详情 $channelId", onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CHANNEL_WEB_DETAIL) {
        WebChannelDetailScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CHANNEL_WECHAT_DETAIL) {
        WeChatChannelDetailScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CHANNEL_QQ_DETAIL) {
        QQChannelDetailScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CHANNEL_API_DETAIL) {
        ApiChannelDetailScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CHANNEL_NEW) {
        ChannelCreateScreen(
            onBack = { navController.popBackStack() },
            onComplete = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CHANNEL_EDIT,
        arguments = listOf(navArgument("channelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val channelId = backStackEntry.arguments?.getString("channelId").orEmpty()
        ChannelEditScreen(
            channelId = channelId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CHANNEL_DIAGNOSTICS,
        arguments = listOf(navArgument("channelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val channelId = backStackEntry.arguments?.getString("channelId").orEmpty()
        ChannelDiagnosticScreen(onBack = { navController.popBackStack() })
    }

    composable(
        route = AmitiaRoutes.Main.CHANNEL_NOTIFICATION,
        arguments = listOf(navArgument("channelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val channelId = backStackEntry.arguments?.getString("channelId").orEmpty()
        ChannelNotificationSettingsScreen(onBack = { navController.popBackStack() })
    }
}
