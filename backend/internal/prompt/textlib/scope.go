package textlib

type SourceSet string

const (
	SourceSetA    SourceSet = "source_a"
	SourceSetRefB SourceSet = "ref_b"
)

type Category string

const (
	CatMainChat       Category = "main_chat"
	CatPersonality    Category = "personality"
	CatEmotionFusion  Category = "emotion_fusion"
	CatAdultIntimacy  Category = "adult_intimacy"
	CatMemory         Category = "memory"
	CatProactive      Category = "proactive"
	CatOutputCleaning Category = "output_cleaning"
	CatChannelRules   Category = "channel_rules"
	CatWaveMessages   Category = "wave_messages"
	CatAntiRepeat     Category = "anti_repeat"
)

type PromptConstant struct {
	Name      string
	SourceSet SourceSet
	Category  Category
	Raw       string
}

var AllConstants = []PromptConstant{
	// Source A
	{Name: "RawPromptMainchat", SourceSet: SourceSetA, Category: CatMainChat, Raw: RawPromptMainchat},
	{Name: "RawPromptPersonality", SourceSet: SourceSetA, Category: CatPersonality, Raw: RawPromptPersonality},
	{Name: "RawPromptPersonalityen", SourceSet: SourceSetA, Category: CatPersonality, Raw: RawPromptPersonalityen},
	{Name: "RawPromptEmotionfusion", SourceSet: SourceSetA, Category: CatEmotionFusion, Raw: RawPromptEmotionfusion},
	{Name: "RawPromptEmotionfusionen", SourceSet: SourceSetA, Category: CatEmotionFusion, Raw: RawPromptEmotionfusionen},
	{Name: "RawPromptAdultmode", SourceSet: SourceSetA, Category: CatAdultIntimacy, Raw: RawPromptAdultmode},
	{Name: "RawPromptMemoryfactextract", SourceSet: SourceSetA, Category: CatMemory, Raw: RawPromptMemoryfactextract},
	{Name: "RawPromptMemoryconsolidation", SourceSet: SourceSetA, Category: CatMemory, Raw: RawPromptMemoryconsolidation},
	{Name: "RawPromptMemorycontradiction", SourceSet: SourceSetA, Category: CatMemory, Raw: RawPromptMemorycontradiction},
	{Name: "RawPromptMemoryepisode", SourceSet: SourceSetA, Category: CatMemory, Raw: RawPromptMemoryepisode},
	{Name: "RawPromptMemorysixdimension", SourceSet: SourceSetA, Category: CatMemory, Raw: RawPromptMemorysixdimension},
	{Name: "RawMemoryFactExtractor", SourceSet: SourceSetA, Category: CatMemory, Raw: RawMemoryFactExtractor},
	{Name: "RawMemoryConsolidator", SourceSet: SourceSetA, Category: CatMemory, Raw: RawMemoryConsolidator},
	{Name: "RawMemoryContradictionDetector", SourceSet: SourceSetA, Category: CatMemory, Raw: RawMemoryContradictionDetector},
	{Name: "RawMemoryEpisodeExtractor", SourceSet: SourceSetA, Category: CatMemory, Raw: RawMemoryEpisodeExtractor},
	{Name: "RawMemoryUserDossier", SourceSet: SourceSetA, Category: CatMemory, Raw: RawMemoryUserDossier},
	{Name: "RawChatBuildWaveMessages", SourceSet: SourceSetA, Category: CatWaveMessages, Raw: RawChatBuildWaveMessages},
	{Name: "RawCompanionProactiveCompose", SourceSet: SourceSetA, Category: CatProactive, Raw: RawCompanionProactiveCompose},
	{Name: "RawCompanionProactivePersonalityContext", SourceSet: SourceSetA, Category: CatProactive, Raw: RawCompanionProactivePersonalityContext},
	{Name: "RawPersonalityPresets", SourceSet: SourceSetA, Category: CatPersonality, Raw: RawPersonalityPresets},

	// RefB
	{Name: "RawNetworkAiPromptBuilder", SourceSet: SourceSetRefB, Category: CatMainChat, Raw: RawNetworkAiPromptBuilder},
	{Name: "RawCommonRolePromptProvider", SourceSet: SourceSetRefB, Category: CatMainChat, Raw: RawCommonRolePromptProvider},
	{Name: "RawFeatureChatUiViewmodelChatPromptBuilder", SourceSet: SourceSetRefB, Category: CatMainChat, Raw: RawFeatureChatUiViewmodelChatPromptBuilder},
	{Name: "RawNetworkResponsePostProcessor", SourceSet: SourceSetRefB, Category: CatOutputCleaning, Raw: RawNetworkResponsePostProcessor},
	{Name: "RawFeatureChatUiViewmodelAiResponseFinalizer", SourceSet: SourceSetRefB, Category: CatOutputCleaning, Raw: RawFeatureChatUiViewmodelAiResponseFinalizer},
	{Name: "RawCommonContentFilter", SourceSet: SourceSetRefB, Category: CatOutputCleaning, Raw: RawCommonContentFilter},
	{Name: "RawChannelWechatShortRules", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawChannelWechatShortRules},
	{Name: "RawChannelQQShortRules", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawChannelQQShortRules},
	{Name: "RawChannelWebDesktopRules", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawChannelWebDesktopRules},
	{Name: "RawAntiRepeat", SourceSet: SourceSetA, Category: CatAntiRepeat, Raw: RawAntiRepeat},
	{Name: "RawAntiRepeatPriorAware", SourceSet: SourceSetA, Category: CatAntiRepeat, Raw: RawAntiRepeatPriorAware},
	{Name: "RawFeatureMemoryEngineMemoryManager", SourceSet: SourceSetRefB, Category: CatMemory, Raw: RawFeatureMemoryEngineMemoryManager},
	{Name: "RawDatabaseRepositoryMemoryRepository", SourceSet: SourceSetRefB, Category: CatMemory, Raw: RawDatabaseRepositoryMemoryRepository},
	{Name: "RawFeatureNotificationCompanionMessageWorker", SourceSet: SourceSetRefB, Category: CatProactive, Raw: RawFeatureNotificationCompanionMessageWorker},
	{Name: "RawFeatureNotificationAiReplyWorker", SourceSet: SourceSetRefB, Category: CatProactive, Raw: RawFeatureNotificationAiReplyWorker},
	{Name: "RawFeatureWechatDataWeChatChatBridge", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawFeatureWechatDataWeChatChatBridge},
	{Name: "RawFeatureWechatServiceWeChatAiReplyWorker", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawFeatureWechatServiceWeChatAiReplyWorker},
	{Name: "RawFeatureWechatServiceWeChatProactiveMessageReceiver", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawFeatureWechatServiceWeChatProactiveMessageReceiver},
	{Name: "RawFeatureQqbotDataQQBotChatBridge", SourceSet: SourceSetRefB, Category: CatChannelRules, Raw: RawFeatureQqbotDataQQBotChatBridge},
	{Name: "RawDatabaseRolePresets", SourceSet: SourceSetRefB, Category: CatPersonality, Raw: RawDatabaseRolePresets},
	{Name: "RawAIBOYFRIENDROLEDESIGN", SourceSet: SourceSetRefB, Category: CatPersonality, Raw: RawAIBOYFRIENDROLEDESIGN},
	{Name: "RawSuperpowersPlans20260624memorysystemoptimization", SourceSet: SourceSetRefB, Category: CatMemory, Raw: RawSuperpowersPlans20260624memorysystemoptimization},
	{Name: "RawSuperpowersSpecs20260624memorysystemoptimizationdesign", SourceSet: SourceSetRefB, Category: CatMemory, Raw: RawSuperpowersSpecs20260624memorysystemoptimizationdesign},
}
