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

        const val CHAT_CONVERSATION = "main/chat_conversation/{characterId}"
        const val CHARACTER_DETAIL = "main/character_detail/{characterId}"
        const val CHARACTER_EDIT = "main/character_edit/{characterId}"
        const val CHARACTER_CREATE = "main/character_create"
        const val MEMORY_DETAIL = "main/memory_detail/{memoryId}"
        const val MEMORY_EDIT = "main/memory_edit/{memoryId}"
        const val MEMORY_NEW = "main/memory_new"
        const val SETTINGS = "main/settings"
        const val MODELS = "main/models"
        const val CHANNELS = "main/channels"
        const val RUNTIME_MANAGEMENT = "main/runtime_management"
        const val CAPABILITY = "main/capability"
        const val DIAGNOSTICS = "main/diagnostics"

        fun chatConversation(characterId: String): String = "main/chat_conversation/$characterId"
        fun characterDetail(characterId: String): String = "main/character_detail/$characterId"
        fun characterEdit(characterId: String): String = "main/character_edit/$characterId"
        fun memoryDetail(memoryId: String): String = "main/memory_detail/$memoryId"
        fun memoryEdit(memoryId: String): String = "main/memory_edit/$memoryId"
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
        Main.TODAY,
        Main.CHAT,
        Main.CHARACTER,
        Main.MEMORY,
        Main.MORE
    )
}
