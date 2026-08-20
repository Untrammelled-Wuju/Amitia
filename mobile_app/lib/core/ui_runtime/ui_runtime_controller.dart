import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/extension_service.dart';
import '../services/providers.dart' show extensionServiceProvider;
import '../backend_connection/providers/backend_connection_providers.dart' show accountSessionProvider;
import '../backend_connection/backend_connection_availability.dart';
import '../backend_connection/providers/runtime_backend_connection_source.dart';
import '../backend_transport/backend_service_api.dart';
import '../backend_transport/http/backend_http_client.dart';
import '../runtime/backend/mobile_backend_providers.dart' show mobileDeploymentConfigProvider;
import '../runtime/backend/mobile_deployment_mode.dart';
import 'ui_client_info.dart';
import 'ui_device_identity.dart';
import 'ui_provider.dart';
import 'ui_snapshot_cache.dart';

final uiRuntimeUsingLastKnownGoodProvider = StateProvider<bool>((ref) => false);

class UIRuntimeController extends StateNotifier<AsyncValue<UIProviderSnapshot?>> {
  UIRuntimeController(
    this._service, {
    required void Function(bool) onLastKnownGood,
    required Future<String> Function() cacheNamespace,
    required Future<String?> Function() meshDeviceId,
  })  : _cacheNamespace = cacheNamespace,
        _meshDeviceId = meshDeviceId,
        _onLastKnownGood = onLastKnownGood,
        super(const AsyncValue.data(null)) {
    // Mobile does not depend on the web SSE transport. Periodic reconciliation
    // keeps cloud profile/provider changes convergent while app is alive.
    _syncTimer = Timer.periodic(const Duration(seconds: 30), (_) {
      if (state.valueOrNull != null && !_loading) {
        unawaited(ensureLoaded(force: true));
      }
    });
  }

  final ExtensionService _service;
  final void Function(bool) _onLastKnownGood;
  final Future<String> Function() _cacheNamespace;
  final Future<String?> Function() _meshDeviceId;
  final UIDeviceIdentity _identity = UIDeviceIdentity();
  final UISnapshotCache _cache = UISnapshotCache();
  late final Timer _syncTimer;
  bool _loading = false;

  Future<String> get deviceId async {
    final meshId = (await _meshDeviceId())?.trim() ?? '';
    if (meshId.isNotEmpty) return meshId;
    return _identity.getOrCreate();
  }

  Future<void> ensureLoaded({bool force = false}) async {
    if (_loading) return;
    if (!force && state.valueOrNull != null) return;
    _loading = true;
    if (state.valueOrNull == null) state = const AsyncValue.loading();
    final platform = currentUIPlatform();
    final currentDeviceId = await deviceId;
    final cacheNamespace = await _cacheNamespace();
    try {
      final json = await _service.getUISnapshot(platform, deviceId: currentDeviceId);
      state = AsyncValue.data(UIProviderSnapshot.fromJson(json));
      _onLastKnownGood(false);
      await _cache.save(cacheNamespace, platform, currentDeviceId, json);
    } catch (error, stack) {
      final cached = await _cache.load(cacheNamespace, platform, currentDeviceId);
      if (cached != null) {
        final offline = Map<String, dynamic>.from(cached);
        final rawContext = (offline['providerContext'] as Map?)?.cast<String, dynamic>() ?? <String, dynamic>{};
        final client = currentUIClientInfo();
        offline['providerContext'] = <String, dynamic>{
          ...rawContext,
          'platform': platform,
          'deviceId': currentDeviceId,
          'architecture': client.architecture,
          'appVersion': client.appVersion,
          'deviceOnline': false,
          'runtimeVersion': '',
          'deviceCapabilities': const <String>[],
        };
        state = AsyncValue.data(UIProviderSnapshot.fromJson(offline));
        _onLastKnownGood(true);
      } else {
        state = AsyncValue.error(error, stack);
        _onLastKnownGood(false);
      }
    } finally {
      _loading = false;
    }
  }

  Future<UIProfileEnvelope> loadProfileScope(UIProfileScopeKind scope) async {
    final json = await _service.getUIProfile(
      platform: currentUIPlatform(),
      deviceId: await deviceId,
      scope: uiProfileScopeKindValue(scope),
    );
    return UIProfileEnvelope.fromJson(json);
  }

  Future<UIProfile> updateProfile(UIProfile profile, {UIProfileScopeKind scope = UIProfileScopeKind.user}) async {
    try {
      final json = await _service.updateUIProfile(
        profile.toJson(),
        platform: currentUIPlatform(),
        deviceId: await deviceId,
        scope: uiProfileScopeKindValue(scope),
      );
      final saved = UIProfile.fromJson(json);
      await ensureLoaded(force: true);
      return saved;
    } catch (_) {
      // Force a fresh cloud revision before the caller offers retry.
      await ensureLoaded(force: true);
      rethrow;
    }
  }

  Future<UIProfile> updateSelection(
    String capability,
    String? providerId, {
    UIProfileScopeKind scope = UIProfileScopeKind.user,
  }) async {
    final envelope = await loadProfileScope(scope);
    final current = envelope.scopeProfile;
    final selections = Map<String, String>.from(current.selections);
    if (providerId == null || providerId.isEmpty) {
      selections.remove(capability);
    } else {
      selections[capability] = providerId;
    }
    return updateProfile(
      UIProfile(
        profileId: current.profileId,
        name: current.name,
        selections: selections,
        scope: current.scope,
        revision: current.revision,
        updatedAt: DateTime.now().millisecondsSinceEpoch,
      ),
      scope: scope,
    );
  }

  Future<void> resetProfileScope(UIProfileScopeKind scope) async {
    if (scope == UIProfileScopeKind.global) {
      throw StateError('Global UI profile cannot be deleted');
    }
    final envelope = await loadProfileScope(scope);
    await _service.deleteUIProfileOverride(
      platform: currentUIPlatform(),
      deviceId: await deviceId,
      scope: uiProfileScopeKindValue(scope),
      revision: envelope.scopeProfile.revision,
    );
    await ensureLoaded(force: true);
  }

  void invalidate() => state = const AsyncValue.data(null);

  @override
  void dispose() {
    _syncTimer.cancel();
    super.dispose();
  }
}

final uiRuntimeProvider = StateNotifierProvider<UIRuntimeController, AsyncValue<UIProviderSnapshot?>>((ref) {
  return UIRuntimeController(
    ref.read(extensionServiceProvider),
    onLastKnownGood: (value) => ref.read(uiRuntimeUsingLastKnownGoodProvider.notifier).state = value,
    cacheNamespace: () async {
      final deployment = ref.read(mobileDeploymentConfigProvider);
      final userId = await ref.read(accountSessionProvider).getUserId();
      final endpoint = deployment.remoteCoreUri?.trim();
      return '${deployment.mode.storageValue}:${endpoint == null || endpoint.isEmpty ? 'embedded' : endpoint}|user=${userId ?? 'anonymous'}';
    },
    meshDeviceId: () async {
      final deployment = ref.read(mobileDeploymentConfigProvider);
      if (deployment.mode != MobileDeploymentMode.hybrid) return null;
      BackendHttpClient? http;
      try {
        final availability = await const RuntimeBackendConnectionSource().resolve();
        if (availability is! BackendConnectionAvailable) return null;
        http = BackendHttpClient(availability.config);
        final api = BackendServiceApi(http, availability.config.generation);
        final status = await api.get<Map<String, dynamic>>('/internal/device-mesh/status');
        final id = (status?['deviceId'] ?? '').toString().trim();
        return id.isEmpty ? null : id;
      } catch (_) {
        return null;
      } finally {
        if (http != null) await http.close();
      }
    },
  );
});

final activeUIProvider = Provider.family<UIProviderDefinition?, String>((ref, capability) {
  return ref.watch(uiRuntimeProvider).valueOrNull?.resolve(capability);
});
