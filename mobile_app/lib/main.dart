import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'app/app.dart';
import 'core/debug/debug_log_service.dart';
import 'core/debug/debug_log_overlay.dart';
import 'core/debug/debug_runtime_bridge.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  final container = ProviderContainer();
  container.read(debugLogServiceProvider).init();
  runApp(UncontrolledProviderScope(
    container: container,
    child: const _DebugRoot(),
  ));
}

class _DebugRoot extends ConsumerWidget {
  const _DebugRoot();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(debugRuntimeLogBridgeProvider);
    return Directionality(
      textDirection: TextDirection.ltr,
      child: Stack(
        children: [
          const AmitiaAppRoot(),
          const DebugLogOverlay(),
        ],
      ),
    );
  }
}
