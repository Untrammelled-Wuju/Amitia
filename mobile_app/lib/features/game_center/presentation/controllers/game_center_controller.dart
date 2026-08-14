import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/game_center_api.dart';
import '../../domain/game_center_dto.dart';

class GameCenterState {
  final List<GamePluginSummary> plugins;
  final bool pluginsLoading;
  final bool pluginsRefreshing;
  final String? pluginsError;
  final int generation;

  final String? selectedPluginId;
  final String? selectedExtensionId;
  final GamePluginDetail? pluginDetail;
  final bool pluginDetailLoading;
  final String? pluginDetailError;

  final String? selectedRuntimeId;
  final GameRuntimeDetail? runtimeDetail;
  final bool runtimeLoading;
  final String? runtimeError;

  final Set<String> packageOperationByExtensionId;
  final Set<String> runtimeOperationByRuntimeId;

  const GameCenterState({
    this.plugins = const [],
    this.pluginsLoading = false,
    this.pluginsRefreshing = false,
    this.pluginsError,
    this.generation = 0,
    this.selectedPluginId,
    this.selectedExtensionId,
    this.pluginDetail,
    this.pluginDetailLoading = false,
    this.pluginDetailError,
    this.selectedRuntimeId,
    this.runtimeDetail,
    this.runtimeLoading = false,
    this.runtimeError,
    this.packageOperationByExtensionId = const {},
    this.runtimeOperationByRuntimeId = const {},
  });

  GameCenterState copyWith({
    List<GamePluginSummary>? plugins,
    bool? pluginsLoading,
    bool? pluginsRefreshing,
    String? pluginsError,
    bool clearPluginsError = false,
    int? generation,
    String? selectedPluginId,
    bool clearSelectedPlugin = false,
    String? selectedExtensionId,
    bool clearSelectedExtension = false,
    GamePluginDetail? pluginDetail,
    bool? pluginDetailLoading,
    String? pluginDetailError,
    bool clearPluginDetailError = false,
    String? selectedRuntimeId,
    bool clearSelectedRuntime = false,
    GameRuntimeDetail? runtimeDetail,
    bool? runtimeLoading,
    String? runtimeError,
    bool clearRuntimeError = false,
    Set<String>? packageOperationByExtensionId,
    Set<String>? runtimeOperationByRuntimeId,
  }) {
    return GameCenterState(
      plugins: plugins ?? this.plugins,
      pluginsLoading: pluginsLoading ?? this.pluginsLoading,
      pluginsRefreshing: pluginsRefreshing ?? this.pluginsRefreshing,
      pluginsError: clearPluginsError ? null : (pluginsError ?? this.pluginsError),
      generation: generation ?? this.generation,
      selectedPluginId: clearSelectedPlugin ? null : (selectedPluginId ?? this.selectedPluginId),
      selectedExtensionId: clearSelectedExtension ? null : (selectedExtensionId ?? this.selectedExtensionId),
      pluginDetail: pluginDetail ?? this.pluginDetail,
      pluginDetailLoading: pluginDetailLoading ?? this.pluginDetailLoading,
      pluginDetailError: clearPluginDetailError ? null : (pluginDetailError ?? this.pluginDetailError),
      selectedRuntimeId: clearSelectedRuntime ? null : (selectedRuntimeId ?? this.selectedRuntimeId),
      runtimeDetail: runtimeDetail ?? this.runtimeDetail,
      runtimeLoading: runtimeLoading ?? this.runtimeLoading,
      runtimeError: clearRuntimeError ? null : (runtimeError ?? this.runtimeError),
      packageOperationByExtensionId: packageOperationByExtensionId ?? this.packageOperationByExtensionId,
      runtimeOperationByRuntimeId: runtimeOperationByRuntimeId ?? this.runtimeOperationByRuntimeId,
    );
  }
}

class GameCenterController extends StateNotifier<GameCenterState> {
  final GameCenterApi api;

  GameCenterController({required this.api}) : super(const GameCenterState());

  Future<void> loadPlugins() async {
    final gen = state.generation + 1;
    state = state.copyWith(pluginsLoading: true, generation: gen, clearPluginsError: true);
    try {
      final result = await api.listPlugins();
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(plugins: result.items, pluginsLoading: false);
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(pluginsLoading: false, pluginsError: e.toString());
    }
  }

  Future<void> refreshPlugins() async {
    final gen = state.generation + 1;
    state = state.copyWith(pluginsRefreshing: true, generation: gen, clearPluginsError: true);
    try {
      final result = await api.listPlugins();
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(plugins: result.items, pluginsRefreshing: false);
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(pluginsRefreshing: false, pluginsError: e.toString());
    }
  }

  Future<void> selectPlugin(
    String pluginId, {
    required String extensionId,
  }) async {
    final gen = state.generation + 1;
    state = state.copyWith(
      selectedPluginId: pluginId,
      selectedExtensionId: extensionId,
      pluginDetailLoading: true,
      generation: gen,
      clearPluginDetailError: true,
    );
    try {
      final detail = await api.getPlugin(pluginId, extensionId: extensionId);
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(pluginDetail: detail, pluginDetailLoading: false);
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(pluginDetailLoading: false, pluginDetailError: e.toString());
    }
  }

  Future<void> selectRuntime(String runtimeId, {String? pluginId}) async {
    final gen = state.generation + 1;
    state = state.copyWith(
      selectedRuntimeId: runtimeId,
      runtimeLoading: true,
      generation: gen,
      clearRuntimeError: true,
    );
    try {
      final detail = await api.getRuntime(runtimeId, pluginId: pluginId);
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(runtimeDetail: detail, runtimeLoading: false);
    } catch (e) {
      if (!mounted) return;
      if (gen != state.generation) return;
      state = state.copyWith(runtimeLoading: false, runtimeError: e.toString());
    }
  }

  void clearSelection() {
    state = state.copyWith(
      clearSelectedPlugin: true,
      clearSelectedExtension: true,
      clearSelectedRuntime: true,
    );
  }

  Future<bool> install(String archivePath) async {
    try {
      await api.installPlugin(archivePath);
      await refreshPlugins();
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<bool> update(String extensionId, String archivePath) async {
    return _withPackageOp(extensionId, () async {
      await api.updatePlugin(extensionId, archivePath);
      await refreshPlugins();
      final pluginId = state.selectedPluginId;
      final extId = state.selectedExtensionId;
      if (pluginId != null && extId != null) {
        await selectPlugin(pluginId, extensionId: extId);
      }
      return true;
    });
  }

  Future<bool> enable(String extensionId) async {
    return _withPackageOp(extensionId, () async {
      final ok = await api.enablePlugin(extensionId);
      if (ok) {
        await refreshPlugins();
        final pluginId = state.selectedPluginId;
        final extId = state.selectedExtensionId;
        if (pluginId != null && extId != null) {
          await selectPlugin(pluginId, extensionId: extId);
        }
      }
      return ok;
    });
  }

  Future<bool> disable(String extensionId) async {
    return _withPackageOp(extensionId, () async {
      final ok = await api.disablePlugin(extensionId);
      if (ok) {
        await refreshPlugins();
        final pluginId = state.selectedPluginId;
        final extId = state.selectedExtensionId;
        if (pluginId != null && extId != null) {
          await selectPlugin(pluginId, extensionId: extId);
        }
      }
      return ok;
    });
  }

  Future<bool> uninstall(String extensionId) async {
    return _withPackageOp(extensionId, () async {
      final ok = await api.uninstallPlugin(extensionId);
      if (ok) {
        await refreshPlugins();
        clearSelection();
      }
      return ok;
    });
  }

  Future<bool> _withPackageOp(String extensionId, Future<bool> Function() op) async {
    final ops = Set<String>.from(state.packageOperationByExtensionId)..add(extensionId);
    state = state.copyWith(packageOperationByExtensionId: ops);
    try {
      return await op();
    } catch (_) {
      return false;
    } finally {
      if (mounted) {
        final newOps = Set<String>.from(state.packageOperationByExtensionId)..remove(extensionId);
        state = state.copyWith(packageOperationByExtensionId: newOps);
      }
    }
  }

  bool hasPackageOp(String extensionId) => state.packageOperationByExtensionId.contains(extensionId);

  Future<bool> startRuntime(String runtimeId) async {
    return _withRuntimeOp(runtimeId, () async {
      await api.startRuntime(runtimeId);
      await selectRuntime(runtimeId);
      return true;
    });
  }

  Future<bool> stopRuntime(String runtimeId) async {
    return _withRuntimeOp(runtimeId, () async {
      await api.stopRuntime(runtimeId);
      await selectRuntime(runtimeId);
      return true;
    });
  }

  Future<bool> restartRuntime(String runtimeId) async {
    return _withRuntimeOp(runtimeId, () async {
      await api.restartRuntime(runtimeId);
      await selectRuntime(runtimeId);
      return true;
    });
  }

  Future<bool> takeover(String runtimeId) async {
    return _withRuntimeOp(runtimeId, () async {
      final result = await api.takeover(runtimeId);
      if (result.success) {
        await selectRuntime(runtimeId);
      }
      return result.success;
    });
  }

  Future<bool> release(String runtimeId, {String targetMode = 'observe'}) async {
    return _withRuntimeOp(runtimeId, () async {
      final epoch = state.runtimeDetail?.controlAuthority?.epoch ?? 0;
      final result = await api.release(
        runtimeId,
        targetMode: targetMode,
        expectedEpoch: epoch,
      );
      if (result.success) {
        await selectRuntime(runtimeId);
      }
      return result.success;
    });
  }

  Future<bool> emergencyStop(String runtimeId) async {
    return _withRuntimeOp(runtimeId, () async {
      final result = await api.emergencyStop(runtimeId);
      if (result.success) {
        await selectRuntime(runtimeId);
      }
      return result.success;
    });
  }

  Future<bool> _withRuntimeOp(String runtimeId, Future<bool> Function() op) async {
    final ops = Set<String>.from(state.runtimeOperationByRuntimeId)..add(runtimeId);
    state = state.copyWith(runtimeOperationByRuntimeId: ops);
    try {
      return await op();
    } catch (_) {
      return false;
    } finally {
      if (mounted) {
        final newOps = Set<String>.from(state.runtimeOperationByRuntimeId)..remove(runtimeId);
        state = state.copyWith(runtimeOperationByRuntimeId: newOps);
      }
    }
  }

  bool hasRuntimeOp(String runtimeId) => state.runtimeOperationByRuntimeId.contains(runtimeId);
}
