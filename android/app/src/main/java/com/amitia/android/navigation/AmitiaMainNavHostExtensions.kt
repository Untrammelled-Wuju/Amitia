package com.amitia.android.navigation

import android.content.Intent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.amitia.android.diagnostics.DiagnosticsActivity
import com.amitia.feature.channels.ChannelsScreen
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
import com.amitia.feature.models.ModelsScreen
import com.amitia.feature.runtime.RuntimeScreen
import com.amitia.feature.settings.SettingsScreen
import com.amitia.feature.settings.about.AboutScreen
import com.amitia.feature.settings.accessibility.AccessibilityScreen
import com.amitia.feature.settings.account.AccountScreen
import com.amitia.feature.settings.appearance.AppearanceScreen
import com.amitia.feature.settings.applock.AppLockScreen
import com.amitia.feature.settings.autostart.AutostartBatteryScreen
import com.amitia.feature.settings.backup.BackupRestoreScreen
import com.amitia.feature.settings.crashrecovery.CrashRecoveryScreen
import com.amitia.feature.settings.data.DataStorageScreen
import com.amitia.feature.settings.developer.DeveloperOptionsScreen
import com.amitia.feature.settings.feedback.FeedbackScreen
import com.amitia.feature.settings.importexport.ImportExportScreen
import com.amitia.feature.settings.language.LanguageScreen
import com.amitia.feature.settings.licenses.OpenSourceLicensesScreen
import com.amitia.feature.settings.localruntime.LocalRuntimeScreen
import com.amitia.feature.settings.more.MoreScreen
import com.amitia.feature.settings.network.NetworkProxyScreen
import com.amitia.feature.settings.notification.NotificationSettingsScreen
import com.amitia.feature.settings.permission.PermissionManagementScreen
import com.amitia.feature.settings.privacy.PrivacyScreen
import com.amitia.feature.settings.runmode.RunModeScreen
import com.amitia.feature.settings.security.SecurityScreen
import com.amitia.feature.settings.update.UpdateScreen
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

internal fun NavGraphBuilder.addModelCenterRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.MODEL_CENTER) {
        ModelCenterHomeScreen(
            onBack = { navController.popBackStack() },
            onProviders = { navController.navigate(AmitiaRoutes.Main.PROVIDER_LIST) },
            onTextModels = { navController.navigate(AmitiaRoutes.Main.TEXT_MODEL_LIST) },
            onVisionModels = { navController.navigate(AmitiaRoutes.Main.VISION_MODEL_LIST) },
            onVoiceModels = { navController.navigate(AmitiaRoutes.Main.VOICE_MODEL_LIST) },
            onVectorModels = { navController.navigate(AmitiaRoutes.Main.VECTOR_MODEL_LIST) },
            onRouting = { navController.navigate(AmitiaRoutes.Main.MODEL_ROUTING) },
            onFallback = { navController.navigate(AmitiaRoutes.Main.FALLBACK_CHAIN) },
            onUsage = { navController.navigate(AmitiaRoutes.Main.MODEL_USAGE) },
            onDiagnostics = { navController.navigate(AmitiaRoutes.Main.MODEL_DIAGNOSTICS) },
            onModelDetail = { id -> navController.navigate(AmitiaRoutes.Main.modelDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.PROVIDER_LIST) {
        ProviderListScreen(
            onBack = { navController.popBackStack() },
            onAddProvider = { navController.navigate(AmitiaRoutes.Main.providerEdit("new")) },
            onEditProvider = { id -> navController.navigate(AmitiaRoutes.Main.providerEdit(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.PROVIDER_EDIT,
        arguments = listOf(navArgument("providerId") { type = NavType.StringType })
    ) { backStackEntry ->
        val providerId = backStackEntry.arguments?.getString("providerId").orEmpty()
        ProviderEditScreen(
            onBack = { navController.popBackStack() },
            providerId = if (providerId == "new") null else providerId
        )
    }

    composable(AmitiaRoutes.Main.TEXT_MODEL_LIST) {
        TextModelListScreen(
            onBack = { navController.popBackStack() },
            onModelDetail = { id -> navController.navigate(AmitiaRoutes.Main.modelDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.VISION_MODEL_LIST) {
        VisionModelListScreen(
            onBack = { navController.popBackStack() },
            onModelDetail = { id -> navController.navigate(AmitiaRoutes.Main.modelDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.VOICE_MODEL_LIST) {
        VoiceModelListScreen(
            onBack = { navController.popBackStack() },
            onModelDetail = { id -> navController.navigate(AmitiaRoutes.Main.modelDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.VECTOR_MODEL_LIST) {
        VectorModelListScreen(
            onBack = { navController.popBackStack() },
            onModelDetail = { id -> navController.navigate(AmitiaRoutes.Main.modelDetail(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.MODEL_DETAIL,
        arguments = listOf(navArgument("modelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val modelId = backStackEntry.arguments?.getString("modelId").orEmpty()
        ModelDetailScreen(
            onBack = { navController.popBackStack() },
            modelId = modelId
        )
    }

    composable(AmitiaRoutes.Main.MODEL_ROUTING) {
        ModelRoutingScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.FALLBACK_CHAIN) {
        FallbackChainScreen(onBack = { navController.popBackStack() })
    }

    composable(
        route = AmitiaRoutes.Main.MODEL_TEST,
        arguments = listOf(navArgument("modelId") { type = NavType.StringType })
    ) { backStackEntry ->
        val modelId = backStackEntry.arguments?.getString("modelId").orEmpty()
        ModelTestScreen(
            modelId = modelId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.MODEL_USAGE) {
        UsageScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.MODEL_DIAGNOSTICS) {
        ModelDiagnosticScreen(onBack = { navController.popBackStack() })
    }
}

internal fun NavGraphBuilder.addVoiceCenterRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.VOICE_CENTER) {
        VoiceCenterHomeScreen(
            onBack = { navController.popBackStack() },
            onTtsSettings = { navController.navigate(AmitiaRoutes.Main.TTS_SETTINGS) },
            onSttSettings = { navController.navigate(AmitiaRoutes.Main.STT_SETTINGS) },
            onVoiceLibrary = { navController.navigate(AmitiaRoutes.Main.VOICE_LIBRARY) },
            onVoiceClone = { navController.navigate(AmitiaRoutes.Main.VOICE_CLONE) },
            onRealtimeVoice = { navController.popBackStack() },
            onAudioDiagnostics = { navController.navigate(AmitiaRoutes.Main.AUDIO_DIAGNOSTICS) }
        )
    }

    composable(AmitiaRoutes.Main.VOICE_LIBRARY) {
        VoiceLibraryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.VOICE_CLONE) {
        VoiceCloneScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.TTS_SETTINGS) {
        TtsSettingsScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.STT_SETTINGS) {
        SttSettingsScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.AUDIO_DIAGNOSTICS) {
        VoiceCenterAudioDiagnosticScreen(onBack = { navController.popBackStack() })
    }
}

internal fun NavGraphBuilder.addCapabilityRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.CAPABILITY_CENTER) {
        CapabilityHomeScreen(
            onOpenSkills = { navController.navigate(AmitiaRoutes.Main.SKILL_LIST) },
            onOpenPlugins = { navController.navigate(AmitiaRoutes.Main.PUBLIC_PLUGIN_LIST) },
            onOpenMcp = { navController.navigate(AmitiaRoutes.Main.MCP_LIST) },
            onOpenComputerUse = { navController.navigate(AmitiaRoutes.Main.COMPUTER_USE) },
            onOpenExtensionCenter = { navController.navigate(AmitiaRoutes.Main.EXTENSION_CENTER) }
        )
    }

    composable(AmitiaRoutes.Main.EXTENSION_CENTER) {
        ExtensionCenterScreen(
            onOpenSystemPlugins = { navController.navigate(AmitiaRoutes.Main.SYSTEM_PLUGIN_LIST) },
            onOpenPublicPlugins = { navController.navigate(AmitiaRoutes.Main.PUBLIC_PLUGIN_LIST) },
            onOpenInstalled = { navController.navigate(AmitiaRoutes.Main.PUBLIC_PLUGIN_LIST) },
            onOpenUpdates = { navController.navigate(AmitiaRoutes.Main.EXTENSION_UPDATE) },
            onOpenImport = { navController.navigate(AmitiaRoutes.Main.EXTENSION_IMPORT) }
        )
    }

    composable(AmitiaRoutes.Main.SYSTEM_PLUGIN_LIST) {
        SystemPluginListScreen(
            onBack = { navController.popBackStack() },
            onOpenPluginDetail = { id -> navController.navigate(AmitiaRoutes.Main.pluginDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.PUBLIC_PLUGIN_LIST) {
        PublicPluginListScreen(
            onBack = { navController.popBackStack() },
            onOpenPluginDetail = { id -> navController.navigate(AmitiaRoutes.Main.pluginDetail(id)) },
            onOpenUpdates = { navController.navigate(AmitiaRoutes.Main.EXTENSION_UPDATE) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.PLUGIN_DETAIL,
        arguments = listOf(navArgument("pluginId") { type = NavType.StringType })
    ) { backStackEntry ->
        val pluginId = backStackEntry.arguments?.getString("pluginId").orEmpty()
        PluginDetailScreen(
            pluginId = pluginId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.EXTENSION_IMPORT) {
        ExtensionImportScreen(
            onBack = { navController.popBackStack() },
            onInstalled = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.EXTENSION_PERMISSION,
        arguments = listOf(navArgument("extensionId") { type = NavType.StringType })
    ) { backStackEntry ->
        ExtensionPermissionReviewScreen(
            onBack = { navController.popBackStack() },
            onApprove = { navController.popBackStack() },
            onReject = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.SKILL_LIST) {
        SkillListScreen(
            onBack = { navController.popBackStack() },
            onOpenSkillDetail = { id -> navController.navigate(AmitiaRoutes.Main.skillDetail(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.SKILL_DETAIL,
        arguments = listOf(navArgument("skillId") { type = NavType.StringType })
    ) { backStackEntry ->
        val skillId = backStackEntry.arguments?.getString("skillId").orEmpty()
        SkillDetailScreen(
            skillId = skillId,
            onBack = { navController.popBackStack() },
            onTest = { navController.navigate(AmitiaRoutes.Main.CAPABILITY_TEST) }
        )
    }

    composable(AmitiaRoutes.Main.MCP_LIST) {
        McpListScreen(
            onBack = { navController.popBackStack() },
            onCreate = { navController.navigate(AmitiaRoutes.Main.MCP_NEW) },
            onOpenDetail = { id -> navController.navigate(AmitiaRoutes.Main.mcpDetail(id)) }
        )
    }

    composable(AmitiaRoutes.Main.MCP_NEW) {
        McpCreateScreen(
            onBack = { navController.popBackStack() },
            onCreated = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.MCP_DETAIL,
        arguments = listOf(navArgument("mcpId") { type = NavType.StringType })
    ) { backStackEntry ->
        val mcpId = backStackEntry.arguments?.getString("mcpId").orEmpty()
        McpDetailScreen(
            serverId = mcpId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.CAPABILITY_TEST) {
        CapabilityTestScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.EXTENSION_UPDATE) {
        ExtensionUpdateScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.EXTENSION_LOG) {
        ExtensionLogScreen(onBack = { navController.popBackStack() })
    }
}

internal fun NavGraphBuilder.addComputerUseRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.COMPUTER_USE) {
        ComputerUseHomeScreen(
            onOpenPermissionMode = { navController.navigate(AmitiaRoutes.Main.CU_PERMISSION_MODE) },
            onOpenSystemPermission = { navController.navigate(AmitiaRoutes.Main.CU_SYSTEM_PERMISSION) },
            onOpenSession = { navController.navigate(AmitiaRoutes.Main.CU_SESSION) },
            onOpenApprovalQueue = { navController.navigate(AmitiaRoutes.Main.CU_APPROVAL_QUEUE) },
            onOpenHistory = { navController.navigate(AmitiaRoutes.Main.CU_HISTORY) },
            onOpenSafetyRules = { navController.navigate(AmitiaRoutes.Main.CU_SECURITY_RULE) }
        )
    }

    composable(AmitiaRoutes.Main.CU_PERMISSION_MODE) {
        PermissionModeScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CU_SYSTEM_PERMISSION) {
        SystemPermissionScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CU_SESSION) {
        RunningSessionScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CU_APPROVAL_QUEUE) {
        ApprovalQueueScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CU_HISTORY) {
        OperationHistoryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.CU_SECURITY_RULE) {
        SafetyRulesScreen(onBack = { navController.popBackStack() })
    }
}

internal fun NavGraphBuilder.addSettingsRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.SETTINGS) {
        SettingsScreen(
            onOpenRuntime = { navController.navigate(AmitiaRoutes.Main.RUNTIME_MANAGEMENT) },
            onLogout = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.SETTINGS_ACCOUNT) {
        AccountScreen(
            onBack = { navController.popBackStack() },
            onLogout = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.SETTINGS_APPEARANCE) {
        AppearanceScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_NOTIFICATION) {
        NotificationSettingsScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_PRIVACY) {
        PrivacyScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_SECURITY) {
        SecurityScreen(
            onBack = { navController.popBackStack() },
            onNavigateAppLock = { navController.navigate(AmitiaRoutes.Main.SETTINGS_APP_LOCK) }
        )
    }

    composable(AmitiaRoutes.Main.SETTINGS_APP_LOCK) {
        AppLockScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_DATA) {
        DataStorageScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_BACKUP) {
        BackupRestoreScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_IMPORT_EXPORT) {
        ImportExportScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_RUN_MODE) {
        RunModeScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_LOCAL_RUNTIME) {
        LocalRuntimeScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_AUTO_START) {
        AutostartBatteryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_NETWORK) {
        NetworkProxyScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_PERMISSIONS) {
        PermissionManagementScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_LANGUAGE) {
        LanguageScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_ACCESSIBILITY) {
        AccessibilityScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_UPDATE) {
        UpdateScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_ABOUT) {
        AboutScreen(
            onBack = { navController.popBackStack() },
            onNavigateLicenses = { navController.navigate(AmitiaRoutes.Main.SETTINGS_LICENSE) }
        )
    }

    composable(AmitiaRoutes.Main.SETTINGS_LICENSE) {
        OpenSourceLicensesScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_FEEDBACK) {
        FeedbackScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_CRASH) {
        CrashRecoveryScreen(onBack = { navController.popBackStack() })
    }

    composable(AmitiaRoutes.Main.SETTINGS_DEVELOPER) {
        DeveloperOptionsScreen(onBack = { navController.popBackStack() })
    }
}

internal fun NavGraphBuilder.addCreativeWorkshopRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.CREATIVE_WORKSHOP) {
        CreativeWorkshopScreen(
            onNewProject = { navController.navigate(AmitiaRoutes.Main.CW_NEW_PROJECT) },
            onProjectClick = { id -> navController.navigate(AmitiaRoutes.Main.cwProjectDetail(id)) },
            onImportProject = { navController.popBackStack() }
        )
    }

    composable(AmitiaRoutes.Main.CW_NEW_PROJECT) {
        NewProjectScreen(
            onBack = { navController.popBackStack() },
            onCreate = { _, _, _ -> navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_PROJECT_DETAIL,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        ProjectDetailScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() },
            onEditManifest = { id -> navController.navigate(AmitiaRoutes.Main.cwManifestEdit(id)) },
            onEditSchema = { id -> navController.navigate(AmitiaRoutes.Main.cwSchemaUiEdit(id)) },
            onPermission = { id -> navController.navigate(AmitiaRoutes.Main.cwPermission(id)) },
            onBuild = { id -> navController.navigate(AmitiaRoutes.Main.cwBuild(id)) },
            onTest = { id -> navController.navigate(AmitiaRoutes.Main.cwTest(id)) },
            onPublish = { id -> navController.navigate(AmitiaRoutes.Main.cwPublish(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_MANIFEST_EDIT,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        ManifestEditScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_SCHEMA_UI_EDIT,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        SchemaUiEditScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() },
            onPreview = { id -> navController.navigate(AmitiaRoutes.Main.cwSchemaUiPreview(id)) }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_SCHEMA_UI_PREVIEW,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        SchemaUiPreviewScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_PERMISSION,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        PermissionScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_BUILD,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        BuildScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_PUBLISH,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        PublishScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }

    composable(
        route = AmitiaRoutes.Main.CW_TEST,
        arguments = listOf(navArgument("projectId") { type = NavType.StringType })
    ) { backStackEntry ->
        val projectId = backStackEntry.arguments?.getString("projectId").orEmpty()
        ProjectTestScreen(
            projectId = projectId,
            onBack = { navController.popBackStack() }
        )
    }
}

internal fun NavGraphBuilder.addMoreRoutes(navController: NavHostController) {
    composable(AmitiaRoutes.Main.MORE) {
        MoreScreen(
            onBack = { navController.popBackStack() },
            onNavigate = { route ->
                val targetRoute = when (route) {
                    "schedule" -> AmitiaRoutes.Main.SCHEDULE
                    "channels" -> AmitiaRoutes.Main.CHANNELS
                    "models" -> AmitiaRoutes.Main.MODELS
                    "voice" -> AmitiaRoutes.Main.VOICE_CENTER
                    "extensions" -> AmitiaRoutes.Main.CAPABILITY_CENTER
                    "computer_use" -> AmitiaRoutes.Main.COMPUTER_USE
                    "workshop" -> AmitiaRoutes.Main.CREATIVE_WORKSHOP
                    "data_storage" -> AmitiaRoutes.Main.SETTINGS_DATA
                    "backup" -> AmitiaRoutes.Main.SETTINGS_BACKUP
                    "import_export" -> AmitiaRoutes.Main.SETTINGS_IMPORT_EXPORT
                    "run_mode" -> AmitiaRoutes.Main.SETTINGS_RUN_MODE
                    "local_runtime" -> AmitiaRoutes.Main.SETTINGS_LOCAL_RUNTIME
                    "console" -> AmitiaRoutes.Main.DIAGNOSTICS
                    "diagnostics" -> AmitiaRoutes.Main.DIAGNOSTICS
                    "settings_center" -> AmitiaRoutes.Main.SETTINGS
                    "developer" -> AmitiaRoutes.Main.SETTINGS_DEVELOPER
                    else -> return@MoreScreen
                }
                navController.navigate(targetRoute)
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

    composable(AmitiaRoutes.Main.DIAGNOSTICS) {
        val context = LocalContext.current
        context.startActivity(Intent(context, DiagnosticsActivity::class.java))
        navController.popBackStack()
    }
}
