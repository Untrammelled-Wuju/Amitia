import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'app/app.dart';
import 'core/debug/debug_log_service.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final container = ProviderContainer();
  container.read(debugLogServiceProvider).init();
  runApp(
    UncontrolledProviderScope(
      container: container,
      child: const AmitiaAppRoot(),
    ),
  );
}
