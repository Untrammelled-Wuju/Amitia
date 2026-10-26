import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/providers/backend_transport_providers.dart';
import '../ui_runtime/ui_runtime_invalidation.dart';
import 'space_profile_service.dart';
import 'character_service.dart';
import 'character_detail_service.dart';
import 'chat_service.dart';
import 'knowledge_service.dart';
import 'device_owned_memory_service.dart';
import 'device_owned_chat_service.dart';
import 'profile_service.dart';
import 'worldbook_service.dart';
import 'continuity_service.dart';
import 'reminder_service.dart';
import 'model_config_service.dart';
import 'feedback_service.dart';
import 'voice_service.dart';
import 'extension_service.dart';
import 'extension_task_service.dart';
import 'extension_view_invalidator.dart';
import 'system_service.dart';
import 'user_feedback_service.dart';
import 'channel_service.dart';
import 'workspace_service.dart';
import 'device_mesh_service.dart';
import 'device_owned_speech_service.dart';
import 'privacy_service.dart';
import 'temporal_service.dart' as temporal_config;
import 'onboarding_service.dart';
import '../models/character.dart';
import '../models/conversation.dart';
import '../models/project.dart';
import '../models/reminder.dart';
import '../models/model_config.dart';
import '../runtime/backend/mobile_backend_providers.dart';
import '../runtime/backend/mobile_deployment_mode.dart';
import '../device_mesh/mobile_device_mesh_identity.dart';
import '../device_mesh/mobile_device_mesh_provisioning.dart';

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
  (ref) => ChatService(
    _getDynamicServiceApi(ref),
    providerKey: () {
      final config = ref.read(mobileDeploymentConfigProvider);
      return config.mode == MobileDeploymentMode.cloud
          ? config.remoteCoreUri ?? ''
          : 'device-local';
    },
    providerTransition: () async {
      final localApi = ref.read(rawDeviceLocalBackendServiceApiProvider);
      return localApi?.get<Map<String, dynamic>>(
        '/internal/device-mesh/status',
      );
    },
  ),
);

final knowledgeServiceProvider = Provider<KnowledgeService>(
  (ref) => KnowledgeService(_getDynamicServiceApi(ref)),
);

final memoryDataSourceProvider = Provider<DeviceOwnedChatService>((ref) {
  ref.watch(mobileDeploymentConfigProvider);
  final api = ref.watch(rawBackendServiceApiProvider);
  final source = DeviceOwnedChatService(
    _getDynamicServiceApi(ref),
    providerStateKey: 'amitia.memory.provider-state',
    providerKey: () {
      final current = ref.read(rawBackendServiceApiProvider);
      if (current == null || !identical(current, api)) return '';
      final config = ref.read(mobileDeploymentConfigProvider);
      return '${config.mode.name}:${config.remoteCoreUri ?? 'local'}:${current.generation}';
    },
  );
  ref.onDispose(() => source.stopLocal('记忆服务已变化，请重新加载'));
  return source;
});

final deviceOwnedMemoryServiceProvider = Provider<DeviceOwnedMemoryService>((
  ref,
) {
  final api = ref.watch(rawBackendServiceApiProvider);
  if (api == null) throw StateError('记忆 Core 服务尚未就绪');
  return DeviceOwnedMemoryService(
    api,
    isCurrent: () => identical(ref.read(rawBackendServiceApiProvider), api),
  );
});

final ownedMemoryCountProvider = FutureProvider.autoDispose<int>((ref) async {
  final rows = await ref.read(deviceOwnedMemoryServiceProvider).listMemories();
  return rows.length;
});

final ownedMemoryModeProvider = FutureProvider<bool>((ref) async {
  final owned = ref.watch(memoryDataSourceProvider);
  final ready = await owned.refresh();
  if (!ready) {
    throw StateError('当前 Core 的设备数据服务尚未就绪，请恢复服务后重试');
  }
  return true;
});

final profileServiceProvider = Provider<ProfileService>(
  (ref) => ProfileService(
    ref.watch(memoryDataSourceProvider),
    memory: () => ref.read(deviceOwnedMemoryServiceProvider),
  ),
);

final worldBookServiceProvider = Provider<WorldBookService>(
  (ref) => WorldBookService(
    ref.read(chatServiceProvider).owned,
    _getDynamicServiceApi(ref),
  ),
);

final continuityServiceProvider = Provider<ContinuityService>(
  (ref) => ContinuityService(
    _getDynamicServiceApi(ref),
    isBound: () =>
        ref.read(mobileDeploymentConfigProvider).mode ==
        MobileDeploymentMode.cloud,
  ),
);

final reminderServiceProvider = Provider<ReminderService>(
  (ref) => ReminderService(_getDynamicServiceApi(ref)),
);

final modelConfigServiceProvider = Provider<ModelConfigService>(
  (ref) => ModelConfigService(_getDynamicServiceApi(ref)),
);

final feedbackServiceProvider = Provider<FeedbackService>(
  (ref) => FeedbackService(
    _getDynamicServiceApi(ref),
    owned: ref.read(chatServiceProvider).owned,
  ),
);

final ttsServiceProvider = Provider<TTSService>(
  (ref) => TTSService(
    _getDynamicServiceApi(ref),
    isBound: () =>
        ref.read(mobileDeploymentConfigProvider).mode ==
        MobileDeploymentMode.cloud,
  ),
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

final extensionTaskServiceProvider = Provider<ExtensionTaskService>((ref) {
  final api = ref.watch(rawBackendServiceApiProvider);
  return ExtensionTaskService(
    _getDynamicServiceApi(ref),
    isBound: () =>
        ref.read(mobileDeploymentConfigProvider).mode ==
        MobileDeploymentMode.cloud,
    taskIsCurrent: () =>
        api != null && identical(ref.read(rawBackendServiceApiProvider), api),
  );
});

final sourceTaskApprovalServiceProvider = Provider<ExtensionTaskService>((ref) {
  final api = ref.watch(rawDeviceLocalBackendServiceApiProvider);
  if (api == null || api.generation <= 0) {
    throw StateError('本机 Device Agent Runtime 尚未就绪');
  }
  return ExtensionTaskService(
    api,
    sourceIsCurrent: () =>
        identical(ref.read(rawDeviceLocalBackendServiceApiProvider), api),
  );
});

final systemServiceProvider = Provider<SystemService>(
  (ref) => SystemService(
    _getDynamicServiceApi(ref),
    owned: ref.read(chatServiceProvider).owned,
  ),
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

final temporalServiceProvider = Provider<temporal_config.TemporalService>(
  (ref) => temporal_config.TemporalService(_getDynamicServiceApi(ref)),
);

final workspaceServiceProvider = Provider<WorkspaceService>(
  (ref) => WorkspaceService(_getDynamicServiceApi(ref)),
);

final deviceMeshServiceProvider = Provider<DeviceMeshService>(
  (ref) => DeviceMeshService(_getDynamicServiceApi(ref)),
);

final mobileDeviceMeshIdentityProvider = Provider<MobileDeviceMeshIdentity>(
  (ref) => const MobileDeviceMeshIdentity(),
);

final mobileDeviceMeshProvisioningProvider =
    Provider<MobileDeviceMeshProvisioning>(
      (ref) => const MobileDeviceMeshProvisioning(),
    );

final deviceOwnedSpeechServiceProvider = Provider<DeviceOwnedSpeechService>(
  (ref) => DeviceOwnedSpeechService(
    api: _getDynamicServiceApi(ref),
    currentApi: () => ref.read(rawBackendServiceApiProvider),
    providerKey: () {
      final config = ref.read(mobileDeploymentConfigProvider);
      return config.mode == MobileDeploymentMode.cloud
          ? config.remoteCoreUri ?? ''
          : '';
    },
  ),
);

final deviceMeshLocalServiceProvider = Provider<DeviceMeshLocalService?>((ref) {
  final api = ref.watch(rawDeviceLocalBackendServiceApiProvider);
  return api == null ? null : DeviceMeshLocalService(api);
});

final deviceMeshDevicesProvider =
    FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
      return ref.read(deviceMeshServiceProvider).devices();
    });

final deviceCoordinationProvider =
    FutureProvider.autoDispose<Map<String, dynamic>>((ref) async {
      return ref.watch(deviceMeshServiceProvider).coordination();
    });

final localDeviceMeshIdentityProvider =
    FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
      final service = ref.watch(deviceMeshLocalServiceProvider);
      return service?.identity();
    });

final localDeviceMeshStatusProvider =
    FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
      final service = ref.watch(deviceMeshLocalServiceProvider);
      return service?.status();
    });

final privacyServiceProvider = Provider<PrivacyService>(
  (ref) => PrivacyService(_getDynamicServiceApi(ref)),
);

final userFeedbackServiceProvider = Provider<UserFeedbackService>(
  (ref) => UserFeedbackService(_getDynamicServiceApi(ref)),
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

final conversationCharacterProvider = FutureProvider.autoDispose
    .family<CharacterDto?, String>((ref, characterId) async {
      final id = characterId.trim();
      if (id.isEmpty) return null;
      final service = ref.watch(characterServiceProvider);
      try {
        final characters = await ref.watch(characterListProvider.future);
        for (final character in characters) {
          if (character.id == id) return character;
        }
      } catch (_) {}
      final character = await service.getById(id);
      return character?.id == id ? character : null;
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

final reminderListProvider = FutureProvider.autoDispose<List<ReminderDto>>((
  ref,
) async {
  final svc = ref.read(reminderServiceProvider);
  return svc.list();
});

final modelConfigListProvider =
    FutureProvider.autoDispose<List<ModelConfigDto>>((ref) async {
      final svc = ref.read(modelConfigServiceProvider);
      return svc.available();
    });

final startupStageProvider = FutureProvider<String>((ref) async => 'ready');

final extensionViewInvalidatorProvider = Provider<ExtensionViewInvalidator>(
  (ref) => ExtensionViewInvalidatorImpl(),
);
