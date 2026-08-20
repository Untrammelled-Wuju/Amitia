import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/extension_service.dart';
import '../services/providers.dart' show extensionServiceProvider;
import 'ui_provider.dart';

class UIRuntimeController extends StateNotifier<AsyncValue<UIProviderSnapshot?>> {
  UIRuntimeController(this._service) : super(const AsyncValue.data(null));
  final ExtensionService _service;
  bool _loading = false;

  Future<void> ensureLoaded({bool force = false}) async {
    if (_loading) return;
    if (!force && state.valueOrNull != null) return;
    _loading = true;
    if (state.valueOrNull == null) state = const AsyncValue.loading();
    try {
      final json = await _service.getUISnapshot(currentUIPlatform());
      if (json != null) {
        state = AsyncValue.data(UIProviderSnapshot.fromJson(json));
      }
    } catch (error, stack) {
      state = AsyncValue.error(error, stack);
    } finally {
      _loading = false;
    }
  }

  Future<void> updateProfile(UIProfile profile) async {
    await _service.updateUIProfile(profile.toJson());
    await ensureLoaded(force: true);
  }

  void invalidate() { state = const AsyncValue.data(null); }
}

final uiRuntimeProvider = StateNotifierProvider<UIRuntimeController, AsyncValue<UIProviderSnapshot?>>((ref) {
  return UIRuntimeController(ref.read(extensionServiceProvider));
});

final activeUIProvider = Provider.family<UIProviderDefinition?, String>((ref, capability) {
  return ref.watch(uiRuntimeProvider).valueOrNull?.resolve(capability);
});
