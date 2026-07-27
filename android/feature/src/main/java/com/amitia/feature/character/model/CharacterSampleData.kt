package com.amitia.feature.character.model

import androidx.compose.ui.graphics.Color
import com.amitia.core.designsystem.AmitiaCharacterAccents
import com.amitia.core.designsystem.AmitiaStateColors
import com.amitia.core.model.CharacterDto

object CharacterSampleData {

    fun sampleCharacters(): List<CharacterDto> = listOf(
        CharacterDto(
            id = "char_001",
            name = "艾米",
            avatar = null,
            description = "温柔知性的陪伴助手",
            personality = "温和、体贴、善于倾听",
            isCurrent = true,
            createdAt = "2025-03-15",
            updatedAt = "2025-07-20"
        ),
        CharacterDto(
            id = "char_002",
            name = "小凛",
            avatar = null,
            description = "活泼开朗的学习伙伴",
            personality = "活泼、积极、充满好奇心",
            isCurrent = false,
            createdAt = "2025-05-10",
            updatedAt = "2025-07-18"
        ),
        CharacterDto(
            id = "char_003",
            name = "知秋",
            avatar = null,
            description = "沉稳睿智的思考者",
            personality = "冷静、理性、深思熟虑",
            isCurrent = false,
            createdAt = "2025-06-01",
            updatedAt = "2025-07-15"
        )
    )

    fun sampleOverview(character: CharacterDto): CharacterOverviewData = CharacterOverviewData(
        name = character.name,
        identity = character.description ?: "未设置身份",
        avatar = character.avatar,
        currentMood = "愉悦",
        lifeStatus = "休息中",
        recentConversation = "今天和你聊了很多有趣的事情，感觉很开心呢。",
        recentConversationTime = "15分钟前",
        recentMemory = "记住了用户喜欢在下午喝咖啡的习惯",
        nextProactivePlan = "候选时间范围：今晚 20:00 - 21:30",
        themeColor = AmitiaCharacterAccents.SageGreen
    )

    fun samplePetActions(): List<PetActionItem> = PetActionData.predefinedActions.mapIndexed { index, name ->
        PetActionItem(
            id = "action_$index",
            name = name,
            category = when {
                index < 4 -> "基础"
                index < 12 -> "情绪"
                index < 19 -> "动作"
                index < 22 -> "互动反馈"
                else -> "系统反馈"
            },
            status = when {
                index < 6 -> PetActionStatus.Ready
                index < 10 -> PetActionStatus.Pending
                index < 12 -> PetActionStatus.Generating
                else -> PetActionStatus.Missing
            },
            frameCount = if (index < 6) (8 + index * 2) else 0,
            loopMode = if (index < 6) LoopMode.Loop else LoopMode.None,
            boundEmotion = when (name) {
                "开心" -> "愉悦"
                "难过" -> "悲伤"
                "生气" -> "愤怒"
                "害羞" -> "害羞"
                "惊讶" -> "惊讶"
                "思考" -> "思考"
                else -> null
            },
            hasPreview = index < 6
        )
    }

    fun samplePetAssetSet(): PetAssetSet = PetAssetSet(
        id = "asset_set_001",
        name = "默认资源集",
        actionCount = 6,
        pendingCount = 4,
        transparentBackground = true,
        fps = 24,
        width = 512,
        height = 512,
        fileSizeKb = 8640
    )

    fun sampleRelationships(): List<CharacterRelationship> = listOf(
        CharacterRelationship("rel_1", "用户", "主人", "信任期", 85, "一起完成了一个项目", "2天前"),
        CharacterRelationship("rel_2", "小凛", "朋友", "熟悉期", 60, "一起讨论了学习计划", "5天前")
    )

    fun sampleRelationshipEvents(): List<RelationshipEvent> = listOf(
        RelationshipEvent("evt_1", "初次相遇", "在引导流程中第一次认识用户", "2025-03-15", "关系建立"),
        RelationshipEvent("evt_2", "第一次深度对话", "聊到了用户的兴趣爱好", "2025-03-20", "亲密度+10"),
        RelationshipEvent("evt_3", "建立信任", "用户分享了个人烦恼并得到帮助", "2025-04-10", "进入信任期"),
        RelationshipEvent("evt_4", "共同成长", "一起完成了一个学习项目", "2025-07-18", "亲密度+5")
    )

    fun sampleEmotionState(): CharacterEmotionState = CharacterEmotionState(
        currentMood = "愉悦",
        moodColor = Color(0xFF7FB28E),
        intensity = 72,
        trend = EmotionTrend.Stable,
        factors = listOf(
            EmotionFactor("最近对话", 35),
            EmotionFactor("生活状态", 25),
            EmotionFactor("用户情绪", 20),
            EmotionFactor("时间因素", 10)
        ),
        recentTriggers = listOf(
            EmotionTrigger("trig_1", "用户夸奖了角色", "愉悦+15", "15分钟前"),
            EmotionTrigger("trig_2", "完成了一个任务", "满足+10", "1小时前"),
            EmotionTrigger("trig_3", "用户暂时离开", "轻微失落-5", "2小时前")
        ),
        systemEnabled = true,
        systemIntensity = 70
    )

    fun sampleLifeStatuses(): List<LifeStatusItem> = listOf(
        LifeStatusItem("ls_1", "起床", LifeStatusSource.Schedule, "07:00", "07:30", false),
        LifeStatusItem("ls_2", "午饭", LifeStatusSource.Schedule, "12:00", "12:45", false),
        LifeStatusItem("ls_3", "午睡", LifeStatusSource.Auto, "13:00", "13:30", true),
        LifeStatusItem("ls_4", "晚饭", LifeStatusSource.Schedule, "18:00", "18:45", false),
        LifeStatusItem("ls_5", "睡觉", LifeStatusSource.Schedule, "23:00", "07:00", false)
    )

    fun sampleProactiveRule(): ProactiveMessageRule = ProactiveMessageRule(
        enabled = true,
        timeWindows = listOf(
            TimeWindow("tw_1", "早晨", 8, 10, true),
            TimeWindow("tw_2", "午后", 13, 15, true),
            TimeWindow("tw_3", "晚间", 19, 22, true)
        ),
        frequency = ProactiveFrequency(minIntervalMinutes = 120, maxPerDay = 5, randomMode = true),
        quietHours = QuietHours(enabled = true, startHour = 23, endHour = 7),
        lifeStatusTrigger = true,
        channelAssignment = listOf("Web", "微信"),
        recentMessages = listOf(
            ProactiveMessageRecord("pm_1", "今天的天气不错，记得出去走走哦", "微信", "昨天 14:30"),
            ProactiveMessageRecord("pm_2", "学习辛苦了，要不要休息一下？", "Web", "前天 16:00")
        ),
        nextCandidateWindow = "候选时间范围：今天 19:00 - 22:00"
    )

    fun sampleVoiceConfig(): VoiceConfig = VoiceConfig(
        provider = "Azure TTS",
        voiceName = "zh-CN-XiaoxiaoNeural",
        speed = 1.0f,
        pitch = 1.0f,
        emotionIntensity = 0.6f,
        fallbackVoice = "zh-CN-YunxiNeural",
        hasClone = false
    )

    fun sampleModelBinding(): ModelBindingConfig = ModelBindingConfig(
        textModel = ModelBinding("model_1", "GPT-4o", "OpenAI", BindingScope.CharacterSpecific, true),
        visionModel = ModelBinding("model_2", "GPT-4o Vision", "OpenAI", BindingScope.InheritGlobal, true),
        voiceModel = ModelBinding("model_3", "Azure TTS", "Azure", BindingScope.InheritGlobal, true),
        vectorModel = null,
        autoRouting = true,
        fallbackChain = listOf("GPT-4o", "Claude-3.5", "本地模型")
    )

    fun sampleMemoryConfig(): MemoryConfig = MemoryConfig(
        longTermEnabled = true,
        episodicEnabled = true,
        worldBooks = listOf(
            WorldBookRef("wb_1", "角色世界观", 128, true),
            WorldBookRef("wb_2", "用户偏好", 56, true),
            WorldBookRef("wb_3", "共同记忆", 32, false)
        ),
        memoryGraphEnabled = true,
        autoSummary = true,
        writeThreshold = 3,
        confirmStrategy = ConfirmStrategy.Important
    )

    fun sampleChannelBindings(): List<ChannelBinding> = listOf(
        ChannelBinding("ch_1", "网页端", "Web", true, true, "10分钟前", "5分钟前", null),
        ChannelBinding("ch_2", "微信", "WeChat", true, true, "1小时前", "30分钟前", null),
        ChannelBinding("ch_3", "QQ", "QQ", false, false, null, null, "未绑定"),
        ChannelBinding("ch_4", "Telegram", "Telegram", true, false, "2天前", "2天前", "连接超时")
    )

    fun sampleCapabilities(): List<CapabilityItem> = listOf(
        CapabilityItem("cap_1", "天气查询", "提供实时天气信息", CapabilityCategory.Skills, true, "角色专属", "1.2.0"),
        CapabilityItem("cap_2", "网页搜索", "搜索引擎查询能力", CapabilityCategory.Skills, true, "继承全局", "2.0.1"),
        CapabilityItem("cap_3", "代码执行", "安全沙箱中执行代码", CapabilityCategory.Plugins, false, "继承全局", "1.0.0"),
        CapabilityItem("cap_4", "文件管理", "MCP 文件系统服务", CapabilityCategory.Mcp, true, "角色专属", null),
        CapabilityItem("cap_5", "Computer Use", "桌面操作能力", CapabilityCategory.ComputerUse, false, "继承全局", null),
        CapabilityItem("cap_6", "通知推送", "系统通知服务", CapabilityCategory.SystemTools, true, "继承全局", null),
        CapabilityItem("cap_7", "剪贴板", "剪贴板读写", CapabilityCategory.SystemTools, true, "继承全局", null),
        CapabilityItem("cap_8", "日程管理", "日历和提醒", CapabilityCategory.Plugins, true, "角色专属", "1.5.0")
    )

    fun samplePermissions(): List<PermissionItem> = listOf(
        PermissionItem("perm_1", "文件访问", "读写本地文件", PermissionType.File, true, "用户授予", "今天 14:30"),
        PermissionItem("perm_2", "相机", "拍照和录像", PermissionType.Camera, false, "未授予", null),
        PermissionItem("perm_3", "麦克风", "语音输入和通话", PermissionType.Microphone, true, "用户授予", "昨天 10:00"),
        PermissionItem("perm_4", "通知", "接收消息推送", PermissionType.Notification, true, "用户授予", "持续"),
        PermissionItem("perm_5", "位置", "获取地理位置", PermissionType.Location, false, "未授予", null),
        PermissionItem("perm_6", "Computer Use", "桌面操作权限", PermissionType.ComputerUse, false, "未授予", null),
        PermissionItem("perm_7", "扩展权限", "第三方扩展访问", PermissionType.Extension, true, "用户授予", "3天前")
    )

    fun sampleDataStats(): CharacterDataStats = CharacterDataStats(
        conversationCount = 342,
        messageCount = 5680,
        memoryCount = 1280,
        storageUsedMb = 45,
        storageQuotaMb = 500,
        firstCreated = "2025-03-15",
        lastActive = "2025-07-27"
    )

    fun sampleArchivedCharacters(): List<ArchivedCharacter> = listOf(
        ArchivedCharacter("arch_1", "旧版助手", null, "2025-06-01", "版本迭代替换", true),
        ArchivedCharacter("arch_2", "测试角色", null, "2025-05-15", "测试完成", false)
    )

    fun sampleAppearanceAssets(): List<AppearanceAsset> = listOf(
        AppearanceAsset("aa_1", AppearanceAssetType.Avatar, null, "默认头像", true),
        AppearanceAsset("aa_2", AppearanceAssetType.FullBody, null, "全身立绘", false),
        AppearanceAsset("aa_3", AppearanceAssetType.Expression, null, "微笑表情", false),
        AppearanceAsset("aa_4", AppearanceAssetType.Expression, null, "思考表情", false)
    )
}
