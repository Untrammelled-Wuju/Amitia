import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/providers/backend_transport_providers.dart';
import '../ui_runtime/ui_runtime_invalidation.dart';
import 'space_profile_service.dart';
import 'character_service.dart';
import 'character_detail_service.dart';
import 'chat_service.dart';
import 'memory_service.dart';
import 'profile_service.dart';
import 'episodic_service.dart';
import 'worldbook_service.dart';
import 'continuity_service.dart';
import 'reminder_service.dart';
import 'companion_service.dart';
import 'model_config_service.dart';
import 'feedback_service.dart';
import 'voice_service.dart';
import 'extension_service.dart';
import 'extension_task_service.dart';
import 'extension_view_invalidator.dart';
import 'system_service.dart';
import 'search_api_service.dart';
import 'channel_service.dart';
import 'workspace_service.dart';
import 'device_mesh_service.dart';
import 'privacy_service.dart';
import 'temporal_service.dart' as temporal_config;
import 'onboarding_service.dart';
import '../models/character.dart';
import '../models/conversation.dart';
import '../models/project.dart';
import '../models/memory.dart';
import '../models/profile.dart';
import '../models/episodic.dart';
import '../models/worldbook.dart';
import '../models/reminder.dart';
import '../models/model_config.dart';

BackendServiceApi _getDynamicServiceApi(Ref ref) {
  return ref.read(backendServiceProvider);
}

final spaceProfileServiceProvider = Provider<SpaceProfileService>(
  (ref) => SpaceProfileService(_getDynamicServiceApi(ref)),
);

final onboardingServiceProvider = Provider<OnboardingService>(
  (ref) => OnboardingService(_getDynamicServiceApi(ref)),
);

final characterServiceProvider = Provider<CharacterService>(
  (ref) => CharacterService(_getDynamicServiceApi(ref)),
);

final characterDetailServiceProvider = Provider<CharacterDetailService>(
  (ref) => CharacterDetailService(_getDynamicServiceApi(ref)),
);

final chatServiceProvider = Provider<ChatService>(
  (ref) => ChatService(_getDynamicServiceApi(ref)),
);

final memoryServiceProvider = Provider<MemoryService>(
  (ref) => MemoryService(_getDynamicServiceApi(ref)),
);

final profileServiceProvider = Provider<ProfileService>(
  (ref) => ProfileService(_getDynamicServiceApi(ref)),
);

final episodicServiceProvider = Provider<EpisodicService>(
  (ref) => EpisodicService(_getDynamicServiceApi(ref)),
);

final worldBookServiceProvider = Provider<WorldBookService>(
  (ref) => WorldBookService(_getDynamicServiceApi(ref)),
);

final continuityServiceProvider = Provider<ContinuityService>(
  (ref) => ContinuityService(_getDynamicServiceApi(ref)),
);

final reminderServiceProvider = Provider<ReminderService>(
  (ref) => ReminderService(_getDynamicServiceApi(ref)),
);

final companionServiceProvider = Provider<CompanionService>(
  (ref) => CompanionService(_getDynamicServiceApi(ref)),
);

final modelConfigServiceProvider = Provider<ModelConfigService>(
  (ref) => ModelConfigService(_getDynamicServiceApi(ref)),
);

final feedbackServiceProvider = Provider<FeedbackService>(
  (ref) => FeedbackService(_getDynamicServiceApi(ref)),
);

final ttsServiceProvider = Provider<TTSService>(
  (ref) => TTSService(_getDynamicServiceApi(ref)),
);

final asrServiceProvider = Provider<ASRService>(
  (ref) => ASRService(_getDynamicServiceApi(ref)),
);

final extensionServiceProvider = Provider<ExtensionService>(
  (ref) => ExtensionService(_getDynamicServiceApi(ref)),
);

final kernelExtensionEnablementProvider = FutureProvider<Map<String, bool>>((
  ref,
) async {
  final subscription = UIRuntimeInvalidationBus.changes.listen(
    (_) => ref.invalidateSelf(),
  );
  ref.onDispose(subscription.cancel);
  final items = await ref.read(extensionServiceProvider).kernelExtensions();
  return <String, bool>{
    for (final item in items)
      if ((item['extensionId'] ?? '').toString().trim().isNotEmpty)
        (item['extensionId'] ?? '').toString().trim():
            (item['enablement'] ?? '').toString() == 'enabled',
  };
});

final extensionTaskServiceProvider = Provider<ExtensionTaskService>(
  (ref) => ExtensionTaskService(_getDynamicServiceApi(ref)),
);

final systemServiceProvider = Provider<SystemService>(
  (ref) => SystemService(_getDynamicServiceApi(ref)),
);

final searchApiServiceProvider = Provider<SearchApiService>(
  (ref) => SearchApiService(_getDynamicServiceApi(ref)),
);

final safetyServiceProvider = Provider<SafetyService>(
  (ref) => SafetyService(_getDynamicServiceApi(ref)),
);

final mcpServiceProvider = Provider<MCPService>(
  (ref) => MCPService(_getDynamicServiceApi(ref)),
);

final imageGenServiceProvider = Provider<ImageGenService>(
  (ref) => ImageGenService(_getDynamicServiceApi(ref)),
);

final visionServiceProvider = Provider<VisionService>(
  (ref) => VisionService(_getDynamicServiceApi(ref)),
);

final embeddingServiceProvider = Provider<EmbeddingService>(
  (ref) => EmbeddingService(_getDynamicServiceApi(ref)),
);

final emoteServiceProvider = Provider<EmoteService>(
  (ref) => EmoteService(_getDynamicServiceApi(ref)),
);

final proactiveServiceProvider = Provider<ProactiveService>(
  (ref) => ProactiveService(_getDynamicServiceApi(ref)),
);

final temporalServiceProvider = Provider<temporal_config.TemporalService>(
  (ref) => temporal_config.TemporalService(_getDynamicServiceApi(ref)),
);

final workspaceServiceProvider = Provider<WorkspaceService>(
  (ref) => WorkspaceService(_getDynamicServiceApi(ref)),
);

final deviceMeshServiceProvider = Provider<DeviceMeshService>(
  (ref) => DeviceMeshService(_getDynamicServiceApi(ref)),
);

final deviceMeshLocalServiceProvider = Provider<DeviceMeshLocalService?>((ref) {
  final api = ref.watch(rawDeviceLocalBackendServiceApiProvider);
  return api == null ? null : DeviceMeshLocalService(api);
});

final deviceMeshDevicesProvider =
    FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
      return ref.read(deviceMeshServiceProvider).devices();
    });

final localDeviceMeshIdentityProvider =
    FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
      final service = ref.watch(deviceMeshLocalServiceProvider);
      return service == null ? null : service.identity();
    });

final localDeviceMeshStatusProvider =
    FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
      final service = ref.watch(deviceMeshLocalServiceProvider);
      return service == null ? null : service.status();
    });

final privacyServiceProvider = Provider<PrivacyService>(
  (ref) => PrivacyService(_getDynamicServiceApi(ref)),
);

final moodServiceProvider = Provider<MoodService>(
  (ref) => MoodService(_getDynamicServiceApi(ref)),
);

final currentSpaceProfileProvider = FutureProvider.autoDispose<SpaceProfile?>((
  ref,
) async {
  try {
    return await ref.read(spaceProfileServiceProvider).fetch();
  } catch (_) {
    return null;
  }
});

final characterListProvider = FutureProvider.autoDispose<List<CharacterDto>>((
  ref,
) async {
  final svc = ref.read(characterServiceProvider);
  return svc.list();
});

final conversationListProvider =
    FutureProvider.autoDispose<List<ConversationDto>>((ref) async {
      final svc = ref.read(chatServiceProvider);
      return svc.listConversations();
    });

final conversationSidebarProvider =
    FutureProvider.autoDispose<ConversationSidebarDto>((ref) async {
      final svc = ref.read(chatServiceProvider);
      return svc.conversationSidebar();
    });

final activeConversationIdProvider = StateProvider<String>((ref) => '');
final conversationCollectionRevisionProvider = StateProvider<int>((ref) => 0);

/// Authoritative session snapshots pushed by the UI Host SSE path.
///
/// Activation commands carry the target snapshot before the server commits the
/// transition. Exposing that snapshot to the declarative runtime first lets the
/// host truthfully acknowledge that it has reconciled the requested revision.
final clientRuntimePushedSessionStateProvider = StateProvider.autoDispose
    .family<Map<String, dynamic>?, String>((ref, conversationId) => null);

final clientRuntimeSessionStateProvider = StreamProvider.autoDispose
    .family<Map<String, dynamic>, String>((ref, conversationId) async* {
      final id = conversationId.trim();
      if (id.isEmpty) {
        yield const <String, dynamic>{
          'conversationId': '',
          'revision': 0,
          'packages': <dynamic>[],
        };
        return;
      }
      final svc = ref.read(extensionServiceProvider);
      final pushed = ref.watch(clientRuntimePushedSessionStateProvider(id));
      var lastRevision = -1;
      if (pushed != null) {
        lastRevision = (pushed['revision'] as num?)?.toInt() ?? 0;
        yield pushed;
      }
      while (true) {
        try {
          var state = await svc.getClientRuntimeSessionState(id);
          var revision = (state['revision'] as num?)?.toInt() ?? 0;
          final packages = (state['packages'] as List?) ?? const <dynamic>[];
          final hasPendingActivation = packages.whereType<Map>().any((raw) {
            final package = raw.cast<String, dynamic>();
            final transition = (package['transitionState'] ?? '')
                .toString()
                .toLowerCase();
            return package['running'] == true &&
                (package['targetVersion'] ?? '').toString().trim().isNotEmpty &&
                transition == 'awaiting_client';
          });
          if (hasPendingActivation) {
            try {
              state = await svc.acknowledgeClientRuntimeSessionState(
                id,
                revision,
              );
              revision = (state['revision'] as num?)?.toInt() ?? revision;
            } catch (_) {}
          }
          if (revision != lastRevision) {
            lastRevision = revision;
            yield state;
          }
        } catch (_) {}
        await Future<void>.delayed(const Duration(seconds: 2));
      }
    });

final conversationUIEventWindowProvider = StreamProvider.autoDispose
    .family<List<Map<String, dynamic>>, String>((ref, conversationId) async* {
      final id = conversationId.trim();
      if (id.isEmpty) {
        yield const <Map<String, dynamic>>[];
        return;
      }
      final svc = ref.read(extensionServiceProvider);
      final records = <Map<String, dynamic>>[];
      final seen = <String>{};
      var cursor = 0;
      var emittedInitial = false;
      const pageSize = 2000;

      while (true) {
        var changed = false;
        while (true) {
          try {
            final page = await svc.getConversationUIEventsAfterSequence(
              id,
              afterSequence: cursor,
              limit: pageSize,
            );
            if (page.isEmpty) break;
            for (final row in page) {
              final sequence = (row['sequence'] as num?)?.toInt() ?? 0;
              final eventId = (row['eventId'] ?? '').toString();
              final key = sequence > 0 ? 'seq:$sequence' : 'id:$eventId';
              if (seen.add(key)) {
                records.add(row);
                changed = true;
              }
              if (sequence > cursor) cursor = sequence;
            }
            if (page.length < pageSize) break;
          } catch (_) {
            break;
          }
        }
        if (changed || !emittedInitial) {
          emittedInitial = true;
          yield List<Map<String, dynamic>>.unmodifiable(records);
        }
        await Future<void>.delayed(const Duration(seconds: 1));
      }
    });

final memoryListProvider = FutureProvider.autoDispose<List<MemoryDto>>((
  ref,
) async {
  final svc = ref.read(memoryServiceProvider);
  return svc.list();
});

final memoryListByCharacterProvider = FutureProvider.autoDispose
    .family<List<MemoryDto>, String>((ref, characterId) async {
      final svc = ref.read(memoryServiceProvider);
      return svc.list(characterId: characterId);
    });

final memoryCandidateListProvider =
    FutureProvider.autoDispose<List<MemoryCandidateDto>>((ref) async {
      final svc = ref.read(memoryServiceProvider);
      return svc.listCandidates();
    });

final profileListProvider = FutureProvider.autoDispose<List<ProfileDto>>((
  ref,
) async {
  final svc = ref.read(profileServiceProvider);
  return svc.list();
});

final episodicListProvider = FutureProvider.autoDispose<List<EpisodicDto>>((
  ref,
) async {
  final svc = ref.read(episodicServiceProvider);
  return svc.list();
});

final worldBookListProvider = FutureProvider.autoDispose<List<WorldBookDto>>((
  ref,
) async {
  final svc = ref.read(worldBookServiceProvider);
  return svc.list();
});

final reminderListProvider = FutureProvider.autoDispose<List<ReminderDto>>((
  ref,
) async {
  final svc = ref.read(reminderServiceProvider);
  return svc.list();
});

final modelConfigListProvider =
    FutureProvider.autoDispose<List<ModelConfigDto>>((ref) async {
      final svc = ref.read(modelConfigServiceProvider);
      return svc.list();
    });

final companionStateProvider =
    FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
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

final companionStateByCharacterProvider = FutureProvider.autoDispose
    .family<Map<String, dynamic>?, String>((ref, characterId) async {
      final svc = ref.read(companionServiceProvider);
      final state = await svc.state(characterId: characterId);
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

final startupStageProvider = FutureProvider<String>((ref) async => 'ready');

final extensionViewInvalidatorProvider = Provider<ExtensionViewInvalidator>(
  (ref) => ExtensionViewInvalidatorImpl(),
);
