import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/error_utils.dart';
import '../../infrastructure/desktop_pet_plugin_api.dart';
import '../../infrastructure/desktop_pet_plugin_dto.dart';

final desktopPetPluginApiProvider = Provider<DesktopPetPluginApi>((ref) {
  final api = ref.watch(backendServiceProvider);
  return DesktopPetPluginApi(api);
});

final desktopPetPluginControllerProvider =
    StateNotifierProvider<DesktopPetPluginController, DesktopPetPluginState>(
      (ref) => DesktopPetPluginController(
        api: ref.watch(desktopPetPluginApiProvider),
      ),
    );

class DesktopPetPluginState {
  final List<DesktopPetPluginSummaryRef> plugins;
  final bool loading;
  final bool refreshing;
  final bool installing;
  final String? error;
  final Set<String> operationByPluginId;
  final int generation;

  const DesktopPetPluginState({
    this.plugins = const [],
    this.loading = false,
    this.refreshing = false,
    this.installing = false,
    this.error,
    this.operationByPluginId = const {},
    this.generation = 0,
  });

  DesktopPetPluginState copyWith({
    List<DesktopPetPluginSummaryRef>? plugins,
    bool? loading,
    bool? refreshing,
    bool? installing,
    String? error,
    bool clearError = false,
    Set<String>? operationByPluginId,
    int? generation,
  }) {
    return DesktopPetPluginState(
      plugins: plugins ?? this.plugins,
      loading: loading ?? this.loading,
      refreshing: refreshing ?? this.refreshing,
      installing: installing ?? this.installing,
      error: clearError ? null : (error ?? this.error),
      operationByPluginId: operationByPluginId ?? this.operationByPluginId,
      generation: generation ?? this.generation,
    );
  }
}

class DesktopPetPluginSummaryRef {
  final String pluginId;
  final String extensionId;
  final String name;
  final String description;
  final String version;
  final bool enabled;
  final String installState;

  const DesktopPetPluginSummaryRef({
    required this.pluginId,
    required this.extensionId,
    required this.name,
    required this.description,
    required this.version,
    required this.enabled,
    required this.installState,
  });
}

class DesktopPetPluginController extends StateNotifier<DesktopPetPluginState> {
  final DesktopPetPluginApi api;

  DesktopPetPluginController({required this.api})
    : super(const DesktopPetPluginState());

  DesktopPetPluginState get current => state;

  Future<void> load() async {
    final gen = state.generation + 1;
    state = state.copyWith(loading: true, generation: gen, clearError: true);
    try {
      final ok = await _fetchCanonical(gen, initial: true);
      if (!mounted) return;
      if (ok && gen == state.generation) {
        state = state.copyWith(loading: false);
      }
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(loading: false, error: safeErrorMessage(e));
    }
  }

  Future<void> refresh() async {
    final gen = state.generation + 1;
    state = state.copyWith(refreshing: true, generation: gen, clearError: true);
    try {
      final ok = await _fetchCanonical(gen, initial: false);
      if (!mounted) return;
      if (ok && gen == state.generation) {
        state = state.copyWith(refreshing: false);
      }
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(refreshing: false, error: safeErrorMessage(e));
    }
  }

  Future<bool> _fetchCanonical(int gen, {required bool initial}) async {
    final result = await api.list();
    if (!mounted) return false;
    if (gen != state.generation) return false;
    state = state.copyWith(
      plugins: result.plugins
          .map(
            (p) => DesktopPetPluginSummaryRef(
              pluginId: p.pluginId,
              extensionId: p.extensionId,
              name: p.name,
              description: p.description,
              version: p.version,
              enabled: p.enabled,
              installState: p.installState,
            ),
          )
          .toList(),
      clearError: true,
    );
    return true;
  }

  Future<DesktopPetPluginDetail?> detail(String pluginId) async {
    try {
      return await api.detail(pluginId);
    } catch (error) {
      if (mounted) {
        state = state.copyWith(error: safeErrorMessage(error));
      }
      return null;
    }
  }

  Future<Map<String, dynamic>?> previewPackage(
    String packagePath, {
    String expectedExtensionId = '',
  }) async {
    if (state.installing) return null;
    final path = packagePath.trim();
    if (path.isEmpty) return null;
    state = state.copyWith(installing: true, clearError: true);
    try {
      return await api.previewPackage(
        path,
        expectedExtensionId: expectedExtensionId,
      );
    } catch (error) {
      if (mounted) {
        state = state.copyWith(error: safeErrorMessage(error));
      }
      return null;
    } finally {
      if (mounted) {
        state = state.copyWith(installing: false);
      }
    }
  }

  Future<bool> commitPackagePreview(
    Map<String, dynamic> preview, {
    String expectedExtensionId = '',
  }) async {
    if (state.installing) return false;
    final gen = state.generation + 1;
    state = state.copyWith(installing: true, generation: gen, clearError: true);
    var dispatched = false;
    String? operationError;
    try {
      dispatched = true;
      final operationId = await api.commitPackagePreview(
        preview,
        expectedExtensionId: expectedExtensionId,
      );
      return operationId.isNotEmpty;
    } catch (error) {
      operationError = safeErrorMessage(error);
      if (mounted) {
        state = state.copyWith(error: operationError);
      }
      return false;
    } finally {
      if (dispatched && mounted) {
        await _refetchAfterMutation();
      }
      if (mounted) {
        final finalError = operationError ?? state.error;
        state = state.copyWith(
          installing: false,
          error: finalError,
          clearError: finalError == null,
        );
      }
    }
  }

  Future<bool> install(String packagePath) async {
    if (state.installing) {
      return false;
    }
    final trimmed = packagePath.trim();
    if (trimmed.isEmpty) {
      return false;
    }

    final gen = state.generation + 1;
    state = state.copyWith(installing: true, generation: gen, clearError: true);

    var dispatched = false;
    String? operationError;
    try {
      dispatched = true;
      final result = await api.install(trimmed);
      return result.extensionId.isNotEmpty;
    } catch (e) {
      operationError = safeErrorMessage(e);
      if (mounted) {
        state = state.copyWith(error: operationError);
      }
      return false;
    } finally {
      if (dispatched && mounted) {
        await _refetchAfterMutation();
      }
      if (mounted) {
        final finalError = operationError ?? state.error;
        state = state.copyWith(
          installing: false,
          error: finalError,
          clearError: finalError == null,
        );
      }
    }
  }

  Future<bool> update(
    String pluginId,
    String extensionId,
    String packagePath,
  ) async {
    return _withPluginMutation(pluginId, () async {
      final result = await api.update(extensionId, packagePath.trim());
      return result.extensionId == extensionId;
    });
  }

  Future<bool> enable(String pluginId, String extensionId) async {
    return _withPluginMutation(pluginId, () async {
      final r = await api.enable(extensionId);
      return r.success && r.extensionId == extensionId;
    });
  }

  Future<bool> disable(String pluginId, String extensionId) async {
    return _withPluginMutation(pluginId, () async {
      final r = await api.disable(extensionId);
      return r.success && r.extensionId == extensionId;
    });
  }

  Future<bool> uninstall(String pluginId, String extensionId) async {
    return _withPluginMutation(pluginId, () async {
      final r = await api.uninstall(extensionId);
      return r.success && r.extensionId == extensionId;
    });
  }

  Future<bool> _withPluginMutation(
    String pluginId,
    Future<bool> Function() op,
  ) async {
    if (state.operationByPluginId.contains(pluginId)) {
      return false;
    }

    final ops = Set<String>.from(state.operationByPluginId)..add(pluginId);
    state = state.copyWith(operationByPluginId: ops, clearError: true);

    var dispatched = false;
    String? operationError;
    try {
      dispatched = true;
      final ok = await op();
      return ok;
    } catch (e) {
      operationError = safeErrorMessage(e);
      if (mounted) {
        state = state.copyWith(error: operationError);
      }
      return false;
    } finally {
      if (dispatched && mounted) {
        await _refetchAfterMutation();
      }
      if (mounted) {
        final next = Set<String>.from(state.operationByPluginId)
          ..remove(pluginId);
        final finalError = operationError ?? state.error;
        state = state.copyWith(
          operationByPluginId: next,
          error: finalError,
          clearError: finalError == null,
        );
      }
    }
  }

  Future<void> _refetchAfterMutation() async {
    final gen = state.generation + 1;
    state = state.copyWith(generation: gen, clearError: true);
    try {
      await _fetchCanonical(gen, initial: false);
    } catch (e) {
      if (mounted) {
        state = state.copyWith(error: safeErrorMessage(e));
      }
    }
  }

  bool hasOperation(String pluginId) =>
      state.operationByPluginId.contains(pluginId);
}
