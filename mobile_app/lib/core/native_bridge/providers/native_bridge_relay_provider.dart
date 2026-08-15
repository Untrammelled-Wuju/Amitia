import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../native_bridge_relay_client.dart';
import '../native_bridge_platform_dispatcher.dart';

final nativeBridgePlatformDispatcherProvider =
    Provider<NativeBridgePlatformDispatcher>((ref) {
  final dispatcher = createPlatformDispatcher();
  ref.onDispose(() async {});
  return dispatcher;
});

final nativeBridgeRelayClientProvider =
    StateNotifierProvider<NativeBridgeRelayClientNotifier, AsyncValue<void>>(
  (ref) {
    final dispatcher = ref.watch(nativeBridgePlatformDispatcherProvider);
    return NativeBridgeRelayClientNotifier(dispatcher: dispatcher);
  },
);

class NativeBridgeRelayClientNotifier extends StateNotifier<AsyncValue<void>> {
  final NativeBridgePlatformDispatcher dispatcher;
  NativeBridgeRelayClient? _client;
  StreamSubscription<bool>? _connSub;
  bool _isAndroid = false;
  bool _isIOS = false;

  NativeBridgeRelayClientNotifier({required this.dispatcher})
      : super(const AsyncValue.loading());

  void attachBackend(String baseUrl, {required bool isAndroid, required bool isIOS}) {
    _isAndroid = isAndroid;
    _isIOS = isIOS;
    if (!_isAndroid && !_isIOS) {
      _disposeClient();
      return;
    }
    final platform = _isAndroid ? 'android' : 'ios';
    if (_client != null && _client!.baseUrl == baseUrl && _client!.isConnected) {
      return;
    }
    _disposeClient();
    _client = NativeBridgeRelayClient(
      baseUrl: baseUrl,
      platform: platform,
      dispatcher: dispatcher,
    );
    _connSub = _client!.connectionState.listen((connected) {
      state = connected
          ? const AsyncValue.data(null)
          : const AsyncValue.loading();
    });
    _client!.connect();
  }

  NativeBridgeRelayClient? get client => _client;

  void _disposeClient() {
    _connSub?.cancel();
    _connSub = null;
    _client?.dispose();
    _client = null;
  }

  @override
  void dispose() {
    _disposeClient();
    super.dispose();
  }
}
