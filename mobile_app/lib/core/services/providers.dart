import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'auth_service.dart';
import 'character_service.dart';
import 'character_detail_service.dart';
import 'chat_service.dart';
import 'memory_service.dart';
import 'profile_service.dart';
import 'episodic_service.dart';
import 'worldbook_service.dart';
import 'reminder_service.dart';
import 'companion_service.dart';
import 'model_config_service.dart';
import 'feedback_service.dart';
import 'voice_service.dart';
import 'extension_service.dart';
import 'system_service.dart';
import 'channel_service.dart';
import '../api/api_client.dart';
import '../models/character.dart';
import '../models/conversation.dart';
import '../models/memory.dart';
import '../models/profile.dart';
import '../models/episodic.dart';
import '../models/worldbook.dart';
import '../models/reminder.dart';
import '../models/model_config.dart';

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final authServiceProvider = Provider<AuthService>((ref) => AuthService());

final characterServiceProvider = Provider<CharacterService>((ref) => CharacterService());

final characterDetailServiceProvider = Provider<CharacterDetailService>((ref) => CharacterDetailService());

final chatServiceProvider = Provider<ChatService>((ref) => ChatService());

final memoryServiceProvider = Provider<MemoryService>((ref) => MemoryService());

final profileServiceProvider = Provider<ProfileService>((ref) => ProfileService());

final episodicServiceProvider = Provider<EpisodicService>((ref) => EpisodicService());

final worldBookServiceProvider = Provider<WorldBookService>((ref) => WorldBookService());

final reminderServiceProvider = Provider<ReminderService>((ref) => ReminderService());

final companionServiceProvider = Provider<CompanionService>((ref) => CompanionService());

final modelConfigServiceProvider = Provider<ModelConfigService>((ref) => ModelConfigService());

final feedbackServiceProvider = Provider<FeedbackService>((ref) => FeedbackService());

final ttsServiceProvider = Provider<TTSService>((ref) => TTSService());

final asrServiceProvider = Provider<ASRService>((ref) => ASRService());

final extensionServiceProvider = Provider<ExtensionService>((ref) => ExtensionService());

final systemServiceProvider = Provider<SystemService>((ref) => SystemService());

final safetyServiceProvider = Provider<SafetyService>((ref) => SafetyService());

final mcpServiceProvider = Provider<MCPService>((ref) => MCPService());

final qqServiceProvider = Provider<QQService>((ref) => QQService());

final imageGenServiceProvider = Provider<ImageGenService>((ref) => ImageGenService());

final visionServiceProvider = Provider<VisionService>((ref) => VisionService());

final embeddingServiceProvider = Provider<EmbeddingService>((ref) => EmbeddingService());

final emoteServiceProvider = Provider<EmoteService>((ref) => EmoteService());

final proactiveServiceProvider = Provider<ProactiveService>((ref) => ProactiveService());

final temporalServiceProvider = Provider<TemporalService>((ref) => TemporalService());

final authStateProvider = FutureProvider<bool>((ref) async {
  final auth = ref.read(authServiceProvider);
  return auth.isLoggedIn;
});

final currentUserProvider = FutureProvider.autoDispose<UserInfo?>((ref) async {
  final auth = ref.read(authServiceProvider);
  return auth.currentUser;
});

final characterListProvider = FutureProvider.autoDispose<List<CharacterDto>>((ref) async {
  final svc = ref.read(characterServiceProvider);
  return svc.list();
});

final conversationListProvider = FutureProvider.autoDispose<List<ConversationDto>>((ref) async {
  final svc = ref.read(chatServiceProvider);
  return svc.listConversations();
});

final memoryListProvider = FutureProvider.autoDispose<List<MemoryDto>>((ref) async {
  final svc = ref.read(memoryServiceProvider);
  return svc.list();
});

final memoryCandidateListProvider = FutureProvider.autoDispose<List<MemoryCandidateDto>>((ref) async {
  final svc = ref.read(memoryServiceProvider);
  return svc.listCandidates();
});

final profileListProvider = FutureProvider.autoDispose<List<ProfileDto>>((ref) async {
  final svc = ref.read(profileServiceProvider);
  return svc.list();
});

final episodicListProvider = FutureProvider.autoDispose<List<EpisodicDto>>((ref) async {
  final svc = ref.read(episodicServiceProvider);
  return svc.list();
});

final worldBookListProvider = FutureProvider.autoDispose<List<WorldBookDto>>((ref) async {
  final svc = ref.read(worldBookServiceProvider);
  return svc.list();
});

final reminderListProvider = FutureProvider.autoDispose<List<ReminderDto>>((ref) async {
  final svc = ref.read(reminderServiceProvider);
  return svc.list();
});

final modelConfigListProvider = FutureProvider.autoDispose<List<ModelConfigDto>>((ref) async {
  final svc = ref.read(modelConfigServiceProvider);
  return svc.list();
});

final companionStateProvider = FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(companionServiceProvider);
  final state = await svc.state();
  if (state == null) return null;
  return {
    'state': state.state,
    'isSleeping': state.isSleeping,
    'currentActivity': state.currentActivity,
    'nextActivity': state.nextActivity,
    'wakeTime': state.wakeTime,
    'sleepTime': state.sleepTime,
  };
});

final startupStageProvider = FutureProvider<String>((ref) async {
  final auth = ref.read(authServiceProvider);
  final loggedIn = await auth.isLoggedIn;
  if (!loggedIn) return 'needsLogin';
  return 'ready';
});
