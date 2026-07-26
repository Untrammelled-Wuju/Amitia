package com.amitia.android.navigation

object AmitiaRoutes {

    const val STARTUP = "startup"
    const val ONBOARDING = "onboarding"
    const val AUTH = "auth"

    const val HOME = "home"
    const val CHAT = "chat"
    const val CHARACTER = "character"
    const val CAPABILITY = "capability"
    const val SETTINGS = "settings"

    const val CHAT_CONVERSATION = "chat_conversation/{characterId}"
    const val CHARACTER_DETAIL = "character_detail/{characterId}"
    const val CHARACTER_EDIT = "character_edit/{characterId}"
    const val CHARACTER_CREATE = "character_create"
    const val MEMORY_DETAIL = "memory_detail/{memoryId}"
    const val MEMORY_EDIT = "memory_edit/{memoryId}"
    const val MEMORY_CREATE = "memory_create"
    const val MEMORY_NEW = "memory_create/new"
    const val RUNTIME_MANAGEMENT = "runtime_management"
    const val MODELS = "models"
    const val CHANNELS = "channels"

    fun chatConversation(characterId: String): String = "chat_conversation/$characterId"
    fun characterDetail(characterId: String): String = "character_detail/$characterId"
    fun characterEdit(characterId: String): String = "character_edit/$characterId"
    fun memoryDetail(memoryId: String): String = "memory_detail/$memoryId"
    fun memoryEdit(memoryId: String): String = "memory_edit/$memoryId"

    val bottomRoutes: Set<String> = setOf(HOME, CHAT, CHARACTER, CAPABILITY, SETTINGS)
}
