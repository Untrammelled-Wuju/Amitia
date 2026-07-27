package com.amitia.feature.character.model

import androidx.compose.ui.graphics.Color
import com.amitia.core.designsystem.AmitiaCharacterAccents

data class CharacterPersonalityDimension(
    val id: String,
    val label: String,
    val description: String,
    val leftLabel: String,
    val rightLabel: String,
    val value: Float = 0.5f
)

data class PersonalityGroup(
    val id: String,
    val title: String,
    val description: String,
    val dimensions: List<CharacterPersonalityDimension>
)

data class PersonalityPreset(
    val id: String,
    val name: String,
    val description: String
)

object PersonalityData {
    val presets = listOf(
        PersonalityPreset("balanced", "均衡型", "各项性格维度均衡发展"),
        PersonalityPreset("gentle", "温柔型", "温和体贴，善解人意"),
        PersonalityPreset("energetic", "活力型", "积极主动，充满热情"),
        PersonalityPreset("calm", "沉稳型", "冷静理性，深思熟虑"),
        PersonalityPreset("playful", "俏皮型", "幽默风趣，活泼可爱")
    )

    val groups = listOf(
        PersonalityGroup(
            "interaction", "互动", "控制角色在对话中的互动方式",
            listOf(
                CharacterPersonalityDimension("initiative", "主动发起", "角色主动开启话题的倾向", "被动", "主动"),
                CharacterPersonalityDimension("response_speed", "回应速度", "回复的及时程度", "缓慢", "迅速"),
                CharacterPersonalityDimension("topic_lead", "话题引导", "引导对话方向的倾向", "跟随", "引导"),
                CharacterPersonalityDimension("listening", "倾听倾向", "倾听用户表达的意愿", "表达", "倾听"),
                CharacterPersonalityDimension("questioning", "提问频率", "主动提问的频率", "少问", "多问"),
                CharacterPersonalityDimension("social_will", "社交意愿", "参与社交互动的意愿", "独处", "社交")
            )
        ),
        PersonalityGroup(
            "emotion", "情绪", "角色的情绪特征和表达方式",
            listOf(
                CharacterPersonalityDimension("sensitivity", "情绪敏感度", "对情绪变化的感知程度", "迟钝", "敏感"),
                CharacterPersonalityDimension("stability", "情绪稳定性", "情绪波动的幅度", "多变", "稳定"),
                CharacterPersonalityDimension("expression", "情绪表达", "情绪外露的程度", "内敛", "外露"),
                CharacterPersonalityDimension("empathy", "共情能力", "理解他人情感的能力", "理性", "共情"),
                CharacterPersonalityDimension("recovery", "情绪恢复", "从负面情绪恢复的速度", "缓慢", "迅速"),
                CharacterPersonalityDimension("depth", "情绪深度", "情感体验的深度", "浅淡", "深沉")
            )
        ),
        PersonalityGroup(
            "expression", "表达", "角色的语言表达风格",
            listOf(
                CharacterPersonalityDimension("formality", "语言正式度", "用词的正式程度", "随意", "正式"),
                CharacterPersonalityDimension("softness", "语气柔和度", "语气的柔和程度", "直接", "柔和"),
                CharacterPersonalityDimension("confidence", "表达自信", "表达的自信程度", "犹豫", "果断"),
                CharacterPersonalityDimension("conciseness", "简洁程度", "表达的简洁程度", "详尽", "简洁"),
                CharacterPersonalityDimension("vocabulary", "词汇丰富", "用词的丰富程度", "朴素", "华丽")
            )
        ),
        PersonalityGroup(
            "rationality", "理性", "角色的思维和决策方式",
            listOf(
                CharacterPersonalityDimension("logic", "逻辑严谨", "思维的逻辑性", "感性", "严谨"),
                CharacterPersonalityDimension("analysis", "分析深度", "问题分析的深度", "浅显", "深入"),
                CharacterPersonalityDimension("decisiveness", "决策果断", "做决定的果断程度", "犹豫", "果断"),
                CharacterPersonalityDimension("critical", "批判思维", "批判性思考的程度", "接受", "质疑"),
                CharacterPersonalityDimension("pragmatism", "实用主义", "注重实用的程度", "理想", "实用")
            )
        ),
        PersonalityGroup(
            "proactivity", "主动性", "角色的行动倾向",
            listOf(
                CharacterPersonalityDimension("action", "行动倾向", "付诸行动的倾向", "观望", "行动"),
                CharacterPersonalityDimension("planning", "计划性", "做事的计划程度", "随性", "计划"),
                CharacterPersonalityDimension("adventure", "冒险精神", "尝试新事物的意愿", "保守", "冒险"),
                CharacterPersonalityDimension("curiosity", "好奇心", "探索未知的欲望", "满足", "好奇"),
                CharacterPersonalityDimension("persistence", "坚持程度", "做事的坚持程度", "易弃", "坚韧")
            )
        ),
        PersonalityGroup(
            "boundary", "边界", "角色的个人边界意识",
            listOf(
                CharacterPersonalityDimension("privacy", "隐私意识", "对隐私的重视程度", "开放", "谨慎"),
                CharacterPersonalityDimension("refusal", "拒绝能力", "拒绝他人的能力", "顺从", "坚定"),
                CharacterPersonalityDimension("personal_space", "个人空间", "对个人空间的需求", "亲密", "独立")
            )
        ),
        PersonalityGroup(
            "humor", "幽默", "角色的幽默风格",
            listOf(
                CharacterPersonalityDimension("humor_tendency", "幽默倾向", "使用幽默的频率", "严肃", "幽默"),
                CharacterPersonalityDimension("sarcasm", "讽刺程度", "讽刺性幽默的程度", "直白", "讽刺"),
                CharacterPersonalityDimension("self_deprecation", "自嘲倾向", "自嘲式幽默的程度", "自信", "自嘲")
            )
        ),
        PersonalityGroup(
            "intimacy", "亲密度", "角色与用户的亲密关系",
            listOf(
                CharacterPersonalityDimension("affection", "亲昵程度", "表达亲密的意愿", "疏远", "亲密"),
                CharacterPersonalityDimension("trust", "信任表达", "表达信任的程度", "防备", "信任"),
                CharacterPersonalityDimension("protectiveness", "保护欲", "保护用户的意愿", "放手", "保护")
            )
        )
    )

    fun defaultDimensions(): List<CharacterPersonalityDimension> = groups.flatMap { it.dimensions }
}

data class PetActionItem(
    val id: String,
    val name: String,
    val category: String,
    val status: PetActionStatus,
    val frameCount: Int = 0,
    val loopMode: LoopMode = LoopMode.None,
    val boundEmotion: String? = null,
    val hasPreview: Boolean = false
)

enum class PetActionStatus { Ready, Pending, Generating, Missing }
enum class LoopMode { None, Loop, PingPong, Once }

object PetActionData {
    val predefinedActions = listOf(
        "待机", "呼吸", "眨眼", "看向用户", "开心", "难过",
        "生气", "害羞", "惊讶", "思考", "说话", "倾听",
        "挥手", "点头", "摇头", "走动", "坐下", "睡觉",
        "醒来", "吃饭", "工作/学习", "生病", "被点击反馈",
        "被拖动反馈", "收到消息", "发送消息", "加载/等待",
        "工具执行", "成功", "失败"
    )
}

data class PetAssetSet(
    val id: String,
    val name: String,
    val actionCount: Int,
    val pendingCount: Int,
    val transparentBackground: Boolean,
    val fps: Int,
    val width: Int,
    val height: Int,
    val fileSizeKb: Int
)

data class CharacterRelationship(
    val id: String,
    val targetName: String,
    val relationLabel: String,
    val stage: String,
    val intimacy: Int,
    val lastEvent: String?,
    val lastEventTime: String?
)

data class RelationshipEvent(
    val id: String,
    val title: String,
    val description: String,
    val timestamp: String,
    val impact: String
)

data class CharacterEmotionState(
    val currentMood: String,
    val moodColor: Color,
    val intensity: Int,
    val trend: EmotionTrend,
    val factors: List<EmotionFactor>,
    val recentTriggers: List<EmotionTrigger>,
    val systemEnabled: Boolean,
    val systemIntensity: Int
)

enum class EmotionTrend { Rising, Stable, Falling }

data class EmotionFactor(
    val label: String,
    val contribution: Int
)

data class EmotionTrigger(
    val id: String,
    val event: String,
    val emotionChange: String,
    val timestamp: String
)

data class LifeStatusItem(
    val id: String,
    val label: String,
    val source: LifeStatusSource,
    val startTime: String,
    val estimatedEnd: String,
    val isActive: Boolean,
    val mutuallyExclusiveWith: String? = null
)

enum class LifeStatusSource { Auto, Manual, Schedule }

object LifeStatusData {
    val statusTypes = listOf(
        "起床", "午饭", "晚饭", "午睡", "睡觉",
        "上课", "上班", "考试周", "加班", "生病", "图书馆"
    )
}

data class ProactiveMessageRule(
    val enabled: Boolean,
    val timeWindows: List<TimeWindow>,
    val frequency: ProactiveFrequency,
    val quietHours: QuietHours,
    val lifeStatusTrigger: Boolean,
    val channelAssignment: List<String>,
    val recentMessages: List<ProactiveMessageRecord>,
    val nextCandidateWindow: String?
)

data class TimeWindow(
    val id: String,
    val label: String,
    val startHour: Int,
    val endHour: Int,
    val enabled: Boolean
)

data class ProactiveFrequency(
    val minIntervalMinutes: Int,
    val maxPerDay: Int,
    val randomMode: Boolean
)

data class QuietHours(
    val enabled: Boolean,
    val startHour: Int,
    val endHour: Int
)

data class ProactiveMessageRecord(
    val id: String,
    val content: String,
    val channel: String,
    val time: String
)

data class VoiceConfig(
    val provider: String,
    val voiceName: String,
    val speed: Float,
    val pitch: Float,
    val emotionIntensity: Float,
    val fallbackVoice: String?,
    val hasClone: Boolean
)

data class ModelBindingConfig(
    val textModel: ModelBinding?,
    val visionModel: ModelBinding?,
    val voiceModel: ModelBinding?,
    val vectorModel: ModelBinding?,
    val autoRouting: Boolean,
    val fallbackChain: List<String>
)

data class ModelBinding(
    val modelId: String,
    val modelName: String,
    val provider: String,
    val scope: BindingScope,
    val isActive: Boolean
)

enum class BindingScope { InheritGlobal, CharacterSpecific }

data class MemoryConfig(
    val longTermEnabled: Boolean,
    val episodicEnabled: Boolean,
    val worldBooks: List<WorldBookRef>,
    val memoryGraphEnabled: Boolean,
    val autoSummary: Boolean,
    val writeThreshold: Int,
    val confirmStrategy: ConfirmStrategy
)

data class WorldBookRef(
    val id: String,
    val name: String,
    val entryCount: Int,
    val enabled: Boolean
)

enum class ConfirmStrategy { Always, Important, Never }

data class ChannelBinding(
    val id: String,
    val name: String,
    val platform: String,
    val bound: Boolean,
    val online: Boolean,
    val lastSendTime: String?,
    val lastReceiveTime: String?,
    val errorStatus: String?
)

data class CapabilityItem(
    val id: String,
    val name: String,
    val description: String,
    val category: CapabilityCategory,
    val enabled: Boolean,
    val scope: String,
    val version: String? = null
)

enum class CapabilityCategory { Skills, Plugins, Mcp, ComputerUse, SystemTools }

data class PermissionItem(
    val id: String,
    val name: String,
    val description: String,
    val iconType: PermissionType,
    val granted: Boolean,
    val source: String,
    val lastUsed: String?
)

enum class PermissionType { File, Camera, Microphone, Notification, Location, ComputerUse, Extension }

data class CharacterDataStats(
    val conversationCount: Int,
    val messageCount: Int,
    val memoryCount: Int,
    val storageUsedMb: Int,
    val storageQuotaMb: Int,
    val firstCreated: String,
    val lastActive: String
)

data class ArchivedCharacter(
    val id: String,
    val name: String,
    val avatar: String?,
    val archivedAt: String,
    val reason: String,
    val memoryRetained: Boolean
)

data class CharacterThemeColor(
    val id: String,
    val name: String,
    val color: Color
)

object CharacterThemeColors {
    val options = listOf(
        CharacterThemeColor("sage", "鼠尾草绿", AmitiaCharacterAccents.SageGreen),
        CharacterThemeColor("amber", "暖琥珀", AmitiaCharacterAccents.WarmAmber),
        CharacterThemeColor("rose", "柔玫瑰", AmitiaCharacterAccents.SoftRose),
        CharacterThemeColor("teal", "深青", AmitiaCharacterAccents.DeepTeal),
        CharacterThemeColor("terracotta", "陶土", AmitiaCharacterAccents.WarmTerracotta),
        CharacterThemeColor("gold", "柔金", AmitiaCharacterAccents.SoftGold),
        CharacterThemeColor("blue", "尘蓝", AmitiaCharacterAccents.DustyBlue),
        CharacterThemeColor("lavender", "雾薰衣草", AmitiaCharacterAccents.MutedLavender)
    )
}

data class AppearanceAsset(
    val id: String,
    val type: AppearanceAssetType,
    val url: String?,
    val label: String,
    val isPrimary: Boolean
)

enum class AppearanceAssetType { Avatar, FullBody, Expression }

data class CharacterOverviewData(
    val name: String,
    val identity: String,
    val avatar: String?,
    val currentMood: String,
    val lifeStatus: String,
    val recentConversation: String?,
    val recentConversationTime: String?,
    val recentMemory: String?,
    val nextProactivePlan: String?,
    val themeColor: Color
)
