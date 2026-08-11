import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/providers.dart';
import '../../infrastructure/desktop_pet_plugin_api.dart';
import '../../infrastructure/desktop_pet_plugin_dto.dart';

final desktopPetPluginApiProvider = Provider<DesktopPetPluginApi>((ref) {
  final api = ref.watch(_backendServiceProvider);
  if (api == null) {
    throw StateError('BackendServiceApi not available');
  }
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
  final String? error;
  final Set<String> operationByPluginId;
  final int generation;

  const DesktopPetPluginState({
    this.plugins = const [],
    this.loading = false,
    this.refreshing = false,
    this.error,
    this.operationByPluginId = const {},
    this.generation = 0,
  });

  DesktopPetPluginState copyWith({
    List<DesktopPetPluginSummaryRef>? plugins,
    bool? loading,
    bool? refreshing,
    String? error,
    bool clearError = false,
    Set<String>? operationByPluginId,
    int? generation,
  }) {
    return DesktopPetPluginState(
      plugins: plugins ?? this.plugins,
      loading: loading ?? this.loading,
      refreshing: refreshing ?? this.refreshing,
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

  DesktopPetPluginController({required this.api}) : super(const DesktopPetPluginState());

  Future<void> load() async {
    final gen = state.generation + 1;
    state = state.copyWith(loading: true, generation: gen, clearError: true);
    try {
      final result = await api.list();
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(
        plugins: result.plugins
            .map((p) => DesktopPetPluginSummaryRef(
                  pluginId: p.pluginId,
                  extensionId: p.extensionId,
                  name: p.name,
                  description: p.description,
                  version: p.version,
                  enabled: p.enabled,
                  installState: p.installState,
                ))
            .toList(),
        loading: false,
        clearError: true,
      );
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(loading: false, error: e.toString());
    }
  }

  Future<void> refresh() async {
    final gen = state.generation + 1;
    state = state.copyWith(refreshing: true, generation: gen, clearError: true);
    try {
      final result = await api.list();
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(
        plugins: result.plugins
            .map((p) => DesktopPetPluginSummaryRef(
                  pluginId: p.pluginId,
                  extensionId: p.extensionId,
                  name: p.name,
                  description: p.description,
                  version: p.version,
                  enabled: p.enabled,
                  installState: p.installState,
                ))
            .toList(),
        refreshing: false,
        clearError: true,
      );
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(refreshing: false, error: e.toString());
    }
  }

  Future<DesktopPetPluginDetail?> detail(String pluginId) async {
    try {
      return await api.detail(pluginId);
    } catch (_) {
      return null;
    }
  }

  Future<bool> enable(String pluginId, String extensionId) async {
    return _withOperation(pluginId, () async {
      final r = await api.enable(extensionId);
      return r.success;
    });
  }

  Future<bool> disable(String pluginId, String extensionId) async {
    return _withOperation(pluginId, () async {
      final r = await api.disable(extensionId);
      return r.success;
    });
  }

  Future<bool> uninstall(String pluginId, String extensionId) async {
    return _withOperation(pluginId, () async {
      final r = await api.uninstall(extensionId);
      return r.success;
    });
  }

  Future<bool> _withOperation(String pluginId, Future<bool> Function() op) async {
    final ops = Set<String>.from(state.operationByPluginId)..add(pluginId);
    state = state.copyWith(operationByPluginId: ops);
    try {
      final ok = await op();
      return ok;
    } catch (_) {
      return false;
    } finally {
      if (!mounted) return;
      final newOps = Set<String>.from(state.operationByPluginId)..remove(pluginId);
      state = state.copyWith(operationByPluginId: newOps);
    }
  }

  bool hasOperation(String pluginId) => state.operationByPluginId.contains(pluginId);

  @override
  void dispose() {
    super.dispose();
  }
}
