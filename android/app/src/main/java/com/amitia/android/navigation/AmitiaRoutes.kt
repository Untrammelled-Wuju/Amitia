package com.amitia.android.navigation

object AmitiaRoutes {

    object Bootstrap {
        const val SPLASH = "bootstrap/splash"
        const val STARTUP = "bootstrap/startup"
        const val RECOVERY = "bootstrap/recovery"
        const val MIGRATION = "bootstrap/migration"
    }

    object Onboarding {
        const val WELCOME = "onboarding/welcome"
        const val ENVIRONMENT_CHECK = "onboarding/environment-check"
        const val MODE_SELECT = "onboarding/mode-select"
        const val LOCAL_INSTALL = "onboarding/local-install"
        const val REMOTE_CONNECT = "onboarding/remote-connect"
        const val ACCOUNT_ENTRY = "onboarding/account-entry"
        const val REGISTER = "onboarding/register"
        const val LOGIN = "onboarding/login"
        const val PERMISSIONS = "onboarding/permissions"
        const val MODEL_TEXT = "onboarding/model-text"
        const val MODEL_VISION = "onboarding/model-vision"
        const val MODEL_VOICE = "onboarding/model-voice"
        const val MODEL_VECTOR = "onboarding/model-vector"
        const val CHARACTER_APPEARANCE = "onboarding/character-appearance"
        const val CHARACTER_NAME = "onboarding/character-name"
        const val CHARACTER_IDENTITY = "onboarding/character-identity"
        const val CHARACTER_PERSONALITY = "onboarding/character-personality"
        const val INITIAL_MEMORY_1 = "onboarding/initial-memory-1"
        const val INITIAL_MEMORY_2 = "onboarding/initial-memory-2"
        const val INITIAL_MEMORY_3 = "onboarding/initial-memory-3"
        const val SETUP_SUMMARY = "onboarding/setup-summary"
        const val CHARACTER_COMPLETE = "onboarding/character-complete"
        const val ENTER_AMITIA = "onboarding/enter-amitia"
        const val DATA_IMPORT = "onboarding/data-import"

        val ordered: List<String> = listOf(
            WELCOME,
            ENVIRONMENT_CHECK,
            MODE_SELECT,
            LOCAL_INSTALL,
            REMOTE_CONNECT,
            ACCOUNT_ENTRY,
            REGISTER,
            LOGIN,
            PERMISSIONS,
            MODEL_TEXT,
            MODEL_VISION,
            MODEL_VOICE,
            MODEL_VECTOR,
            CHARACTER_APPEARANCE,
            CHARACTER_NAME,
            CHARACTER_IDENTITY,
            CHARACTER_PERSONALITY,
            INITIAL_MEMORY_1,
            INITIAL_MEMORY_2,
            INITIAL_MEMORY_3,
            SETUP_SUMMARY,
            CHARACTER_COMPLETE,
            ENTER_AMITIA,
            DATA_IMPORT
        )
    }

    object Main {
        const val TODAY = "main/today"
        const val CHAT = "main/chat"
        const val CHARACTER = "main/character"
        const val MEMORY = "main/memory"
        const val MORE = "main/more"

        const val TODAY_DETAIL = "main/today_detail"
        const val ACTIVITY_FEED = "main/activity_feed"
        const val NOTIFICATION_CENTER = "main/notification_center"
        const val GLOBAL_SEARCH = "main/global_search"
        const val RUNTIME_ISSUES = "main/runtime_issues"

        const val CHAT_CONVERSATION = "main/chat_conversation/{characterId}"
        const val NEW_CONVERSATION = "main/new_conversation"
        const val CHAT_DETAIL = "main/chat_detail/{conversationId}"
        const val IMAGE_PREVIEW = "main/image_preview/{imageId}"
        const val FILE_DETAIL = "main/file_detail/{fileId}"
        const val MESSAGE_DETAIL = "main/message_detail/{messageId}"
        const val MESSAGE_SEARCH = "main/message_search"
        const val MEDIA_GALLERY = "main/media_gallery/{conversationId}"
        const val TOOL_EXECUTION = "main/tool_execution/{executionId}"
        const val CONTEXT_MANAGEMENT = "main/context_management/{conversationId}"
        const val MEMORY_REFERENCE = "main/memory_reference/{messageId}"
        const val CONVERSATION_EXPORT = "main/conversation_export/{conversationId}"
        const val CONVERSATION_SETTINGS = "main/conversation_settings/{conversationId}"
        const val PROMPT_TRACE = "main/prompt_trace/{messageId}"

        const val CHARACTER_DETAIL = "main/character_detail/{characterId}"
        const val CHARACTER_EDIT = "main/character_edit/{characterId}"
        const val CHARACTER_CREATE = "main/character_create"
        const val CHARACTER_WIZARD = "main/character_wizard"
        const val PET_ASSET = "main/pet_asset/{characterId}"
        const val PET_ACTION_LIBRARY = "main/pet_action_library/{characterId}"
        const val ACTION_GENERATION = "main/action_generation/{characterId}"
        const val ACTION_REVIEW = "main/action_review/{characterId}/{actionId}"

        const val MEMORY_DETAIL = "main/memory_detail/{memoryId}"
        const val MEMORY_EDIT = "main/memory_edit/{memoryId}"
        const val MEMORY_NEW = "main/memory_new"
        const val MEMORY_TIMELINE = "main/memory_timeline"
        const val MEMORY_SEARCH = "main/memory_search"
        const val LONG_TERM_MEMORY = "main/long_term_memory"
        const val EPISODIC_MEMORY = "main/episodic_memory"
        const val WORLD_BOOK_LIST = "main/world_book_list"
        const val WORLD_BOOK_DETAIL = "main/world_book_detail/{bookId}"
        const val WORLD_BOOK_ENTRY_EDIT = "main/world_book_entry_edit/{bookId}/{entryId}"
        const val MEMORY_GRAPH = "main/memory_graph"
        const val PENDING_MEMORY = "main/pending_memory"
        const val MEMORY_CONFLICT = "main/memory_conflict"
        const val MEMORY_IMPORT = "main/memory_import"
        const val IMPORT_FIELD_MAPPING = "main/import_field_mapping"
        const val MEMORY_EXPORT = "main/memory_export"
        const val MEMORY_SETTINGS = "main/memory_settings"

        const val EMOJI_CENTER = "main/emoji_center"
        const val EMOJI_GROUP_DETAIL = "main/emoji_group_detail/{groupId}"
        const val EMOJI_BATCH_IMPORT = "main/emoji_batch_import"
        const val EMOJI_DETAIL_EDIT = "main/emoji_detail_edit/{groupId}"
        const val EMOJI_SCOPE = "main/emoji_scope/{groupId}"
        const val EMOJI_SEND_STRATEGY = "main/emoji_send_strategy/{groupId}"
        const val EMOJI_IMPORT_RESULT = "main/emoji_import_result"

        const val SCHEDULE = "main/schedule"
        const val SCHEDULE_CALENDAR = "main/schedule_calendar"
        const val SCHEDULE_DETAIL = "main/schedule_detail/{scheduleId}"
        const val SCHEDULE_EDIT = "main/schedule_edit"
        const val LIFE_TEMPLATE = "main/life_template"
        const val PROACTIVE_TIME_WINDOW = "main/proactive_time_window"
        const val STATUS_RULE = "main/status_rule"
        const val QUIET_HOURS = "main/quiet_hours"

        const val CHANNEL_CENTER = "main/channel_center"
        const val CHANNEL_DETAIL = "main/channel_detail/{channelId}"
        const val CHANNEL_WEB_DETAIL = "main/channel_web_detail"
        const val CHANNEL_WECHAT_DETAIL = "main/channel_wechat_detail"
        const val CHANNEL_QQ_DETAIL = "main/channel_qq_detail"
        const val CHANNEL_API_DETAIL = "main/channel_api_detail"
        const val CHANNEL_NEW = "main/channel_new"
        const val CHANNEL_EDIT = "main/channel_edit/{channelId}"
        const val CHANNEL_DIAGNOSTICS = "main/channel_diagnostics/{channelId}"
        const val CHANNEL_NOTIFICATION = "main/channel_notification/{channelId}"

        const val MODEL_CENTER = "main/model_center"
        const val PROVIDER_LIST = "main/provider_list"
        const val PROVIDER_EDIT = "main/provider_edit/{providerId}"
        const val TEXT_MODEL_LIST = "main/text_model_list"
        const val VISION_MODEL_LIST = "main/vision_model_list"
        const val VOICE_MODEL_LIST = "main/voice_model_list"
        const val VECTOR_MODEL_LIST = "main/vector_model_list"
        const val MODEL_DETAIL = "main/model_detail/{modelId}"
        const val MODEL_ROUTING = "main/model_routing"
        const val FALLBACK_CHAIN = "main/fallback_chain"
        const val MODEL_TEST = "main/model_test/{modelId}"
        const val MODEL_USAGE = "main/model_usage"
        const val MODEL_DIAGNOSTICS = "main/model_diagnostics"

        const val VOICE_CENTER = "main/voice_center"
        const val VOICE_LIBRARY = "main/voice_library"
        const val VOICE_CLONE = "main/voice_clone"
        const val TTS_SETTINGS = "main/tts_settings"
        const val STT_SETTINGS = "main/stt_settings"
        const val AUDIO_DIAGNOSTICS = "main/audio_diagnostics"

        const val CAPABILITY_CENTER = "main/capability_center"
        const val EXTENSION_CENTER = "main/extension_center"
        const val SYSTEM_PLUGIN_LIST = "main/system_plugin_list"
        const val PUBLIC_PLUGIN_LIST = "main/public_plugin_list"
        const val PLUGIN_DETAIL = "main/plugin_detail/{pluginId}"
        const val EXTENSION_IMPORT = "main/extension_import"
        const val EXTENSION_PERMISSION = "main/extension_permission/{extensionId}"
        const val SKILL_LIST = "main/skill_list"
        const val SKILL_DETAIL = "main/skill_detail/{skillId}"
        const val MCP_LIST = "main/mcp_list"
        const val MCP_NEW = "main/mcp_new"
        const val MCP_DETAIL = "main/mcp_detail/{mcpId}"
        const val CAPABILITY_TEST = "main/capability_test"
        const val EXTENSION_UPDATE = "main/extension_update"
        const val EXTENSION_LOG = "main/extension_log"

        const val COMPUTER_USE = "main/computer_use"
        const val CU_PERMISSION_MODE = "main/cu_permission_mode"
        const val CU_SYSTEM_PERMISSION = "main/cu_system_permission"
        const val CU_SESSION = "main/cu_session"
        const val CU_APPROVAL_QUEUE = "main/cu_approval_queue"
        const val CU_HISTORY = "main/cu_history"
        const val CU_SECURITY_RULE = "main/cu_security_rule"

        const val SETTINGS = "main/settings"
        const val SETTINGS_ACCOUNT = "main/settings_account"
        const val SETTINGS_APPEARANCE = "main/settings_appearance"
        const val SETTINGS_NOTIFICATION = "main/settings_notification"
        const val SETTINGS_PRIVACY = "main/settings_privacy"
        const val SETTINGS_SECURITY = "main/settings_security"
        const val SETTINGS_APP_LOCK = "main/settings_app_lock"
        const val SETTINGS_DATA = "main/settings_data"
        const val SETTINGS_BACKUP = "main/settings_backup"
        const val SETTINGS_IMPORT_EXPORT = "main/settings_import_export"
        const val SETTINGS_RUN_MODE = "main/settings_run_mode"
        const val SETTINGS_LOCAL_RUNTIME = "main/settings_local_runtime"
        const val SETTINGS_AUTO_START = "main/settings_auto_start"
        const val SETTINGS_NETWORK = "main/settings_network"
        const val SETTINGS_PERMISSIONS = "main/settings_permissions"
        const val SETTINGS_LANGUAGE = "main/settings_language"
        const val SETTINGS_ACCESSIBILITY = "main/settings_accessibility"
        const val SETTINGS_UPDATE = "main/settings_update"
        const val SETTINGS_ABOUT = "main/settings_about"
        const val SETTINGS_LICENSE = "main/settings_license"
        const val SETTINGS_FEEDBACK = "main/settings_feedback"
        const val SETTINGS_CRASH = "main/settings_crash"
        const val SETTINGS_DEVELOPER = "main/settings_developer"

        const val CREATIVE_WORKSHOP = "main/creative_workshop"
        const val CW_NEW_PROJECT = "main/cw_new_project"
        const val CW_PROJECT_DETAIL = "main/cw_project_detail/{projectId}"
        const val CW_MANIFEST_EDIT = "main/cw_manifest_edit/{projectId}"
        const val CW_SCHEMA_UI_EDIT = "main/cw_schema_ui_edit/{projectId}"
        const val CW_SCHEMA_UI_PREVIEW = "main/cw_schema_ui_preview/{projectId}"
        const val CW_PERMISSION = "main/cw_permission/{projectId}"
        const val CW_BUILD = "main/cw_build/{projectId}"
        const val CW_PUBLISH = "main/cw_publish/{projectId}"
        const val CW_TEST = "main/cw_test/{projectId}"

        const val MODELS = "main/models"
        const val CHANNELS = "main/channels"
        const val RUNTIME_MANAGEMENT = "main/runtime_management"
        const val CAPABILITY = "main/capability"
        const val DIAGNOSTICS = "main/diagnostics"

        fun chatConversation(characterId: String): String = "main/chat_conversation/$characterId"
        fun chatDetail(conversationId: String): String = "main/chat_detail/$conversationId"
        fun characterDetail(characterId: String): String = "main/character_detail/$characterId"
        fun characterEdit(characterId: String): String = "main/character_edit/$characterId"
        fun petAsset(characterId: String): String = "main/pet_asset/$characterId"
        fun petActionLibrary(characterId: String): String = "main/pet_action_library/$characterId"
        fun actionGeneration(characterId: String): String = "main/action_generation/$characterId"
        fun actionReview(characterId: String, actionId: String): String = "main/action_review/$characterId/$actionId"
        fun memoryDetail(memoryId: String): String = "main/memory_detail/$memoryId"
        fun memoryEdit(memoryId: String): String = "main/memory_edit/$memoryId"
        fun mediaGallery(conversationId: String): String = "main/media_gallery/$conversationId"
        fun toolExecution(executionId: String): String = "main/tool_execution/$executionId"
        fun contextManagement(conversationId: String): String = "main/context_management/$conversationId"
        fun memoryReference(messageId: String): String = "main/memory_reference/$messageId"
        fun conversationExport(conversationId: String): String = "main/conversation_export/$conversationId"
        fun conversationSettings(conversationId: String): String = "main/conversation_settings/$conversationId"
        fun promptTrace(messageId: String): String = "main/prompt_trace/$messageId"
        fun imagePreview(imageId: String): String = "main/image_preview/$imageId"
        fun fileDetail(fileId: String): String = "main/file_detail/$fileId"
        fun messageDetail(messageId: String): String = "main/message_detail/$messageId"
        fun worldBookDetail(bookId: String): String = "main/world_book_detail/$bookId"
        fun worldBookEntryEdit(bookId: String, entryId: String): String = "main/world_book_entry_edit/$bookId/$entryId"
        fun scheduleDetail(scheduleId: String): String = "main/schedule_detail/$scheduleId"
        fun channelDetail(channelId: String): String = "main/channel_detail/$channelId"
        fun channelEdit(channelId: String): String = "main/channel_edit/$channelId"
        fun channelDiagnostics(channelId: String): String = "main/channel_diagnostics/$channelId"
        fun channelNotification(channelId: String): String = "main/channel_notification/$channelId"
        fun providerEdit(providerId: String): String = "main/provider_edit/$providerId"
        fun modelDetail(modelId: String): String = "main/model_detail/$modelId"
        fun modelTest(modelId: String): String = "main/model_test/$modelId"
        fun pluginDetail(pluginId: String): String = "main/plugin_detail/$pluginId"
        fun extensionPermission(extensionId: String): String = "main/extension_permission/$extensionId"
        fun skillDetail(skillId: String): String = "main/skill_detail/$skillId"
        fun mcpDetail(mcpId: String): String = "main/mcp_detail/$mcpId"
        fun cwProjectDetail(projectId: String): String = "main/cw_project_detail/$projectId"
        fun cwManifestEdit(projectId: String): String = "main/cw_manifest_edit/$projectId"
        fun cwSchemaUiEdit(projectId: String): String = "main/cw_schema_ui_edit/$projectId"
        fun cwSchemaUiPreview(projectId: String): String = "main/cw_schema_ui_preview/$projectId"
        fun cwPermission(projectId: String): String = "main/cw_permission/$projectId"
        fun cwBuild(projectId: String): String = "main/cw_build/$projectId"
        fun cwPublish(projectId: String): String = "main/cw_publish/$projectId"
        fun cwTest(projectId: String): String = "main/cw_test/$projectId"
    }

    object VoiceCall {
        const val CALL = "voice/call"
        const val INCOMING = "voice/incoming"
        const val CAPTIONS = "voice/captions"
        const val AUDIO_DEVICE = "voice/audio-device"
        const val CALL_HISTORY = "voice/call-history"
        const val CALL_DETAIL = "voice/call-detail/{callId}"

        fun callDetail(callId: String): String = "voice/call-detail/$callId"
    }

    object Secure {
        const val APP_UNLOCK = "secure/app-unlock"
        const val COMPUTER_USE_APPROVAL = "secure/computer-use-approval"
        const val SENSITIVE_PERMISSION = "secure/sensitive-permission"
        const val DESTRUCTIVE_CONFIRMATION = "secure/destructive-confirmation"
    }

    object Diagnostics {
        const val OVERVIEW = "diagnostics/overview"
        const val SERVICES = "diagnostics/services"
        const val DATABASES = "diagnostics/databases"
        const val TASK_RUNTIME = "diagnostics/task-runtime"
        const val TRUSTED_SERVICE_RUNTIME = "diagnostics/trusted-service-runtime"
        const val WASM_RUNTIME = "diagnostics/wasm-runtime"
        const val HOOKS = "diagnostics/hooks"
        const val EVENTS = "diagnostics/events"
        const val SCHEDULES = "diagnostics/schedules"
        const val UI_CONTRIBUTIONS = "diagnostics/ui-contributions"
        const val RESTRICTED_WEB_UI = "diagnostics/restricted-web-ui"
        const val UPDATES = "diagnostics/updates"
        const val MIGRATIONS = "diagnostics/migrations"
        const val AUDIT = "diagnostics/audit"
        const val PERFORMANCE = "diagnostics/performance"
        const val LOGS = "diagnostics/logs"
        const val FEATURE_FLAGS = "diagnostics/feature-flags"

        val ordered: List<String> = listOf(
            OVERVIEW,
            SERVICES,
            DATABASES,
            TASK_RUNTIME,
            TRUSTED_SERVICE_RUNTIME,
            WASM_RUNTIME,
            HOOKS,
            EVENTS,
            SCHEDULES,
            UI_CONTRIBUTIONS,
            RESTRICTED_WEB_UI,
            UPDATES,
            MIGRATIONS,
            AUDIT,
            PERFORMANCE,
            LOGS,
            FEATURE_FLAGS
        )
    }

    const val KEY_CHARACTER_ID = "characterId"
    const val KEY_MEMORY_ID = "memoryId"
    const val KEY_CALL_ID = "callId"

    val primaryRoutes: Set<String> = setOf(
        Main.CHAT,
        Main.CHARACTER,
        Main.MEMORY,
        Main.MORE
    )
}
