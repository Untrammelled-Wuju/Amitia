package com.amitia.feature.emoji

data class EmojiGroupItem(
    val id: String,
    val name: String,
    val count: Int,
    val coverPath: String? = null,
    val lastImported: String,
    val isUngrouped: Boolean = false
)

data class EmojiItem(
    val id: String,
    val path: String,
    val meaning: String? = null,
    val context: String? = null,
    val groupId: String? = null,
    val groupName: String? = null,
    val sharedAll: Boolean = true,
    val characterIds: List<String> = emptyList(),
    val enabled: Boolean = true,
    val source: String = "手动导入",
    val importedAt: String = "",
    val needsMeaning: Boolean = false
)

data class EmojiImportItem(
    val id: String,
    val path: String,
    val status: EmojiImportStatus,
    val meaning: String? = null,
    val duplicateOf: String? = null,
    val errorMessage: String? = null
)

enum class EmojiImportStatus {
    Pending, Success, Duplicate, Failed, NeedsMeaning
}

data class EmojiImportResult(
    val successCount: Int = 0,
    val duplicateCount: Int = 0,
    val failedCount: Int = 0,
    val needsMeaningCount: Int = 0,
    val items: List<EmojiImportItem> = emptyList()
)

data class EmojiSendStrategy(
    val aiSendEnabled: Boolean = false,
    val randomProbability: Float = 0.1f,
    val endOnly: Boolean = true,
    val allowInterleave: Boolean = false,
    val defaultReplyInterleave: Boolean = false,
    val cooldownSeconds: Int = 30,
    val maxPerTurn: Int = 1
)

data class EmojiScopeConfig(
    val scopeType: EmojiScopeType = EmojiScopeType.AllCharacters,
    val selectedCharacterIds: List<String> = emptyList()
)

enum class EmojiScopeType(val label: String) {
    AllCharacters("所有角色共享"),
    OnlySpecified("仅指定角色"),
    ExcludeSpecified("排除指定角色")
}

data class CharacterOption(
    val id: String,
    val name: String,
    val avatarPath: String? = null,
    val selected: Boolean = false
)
