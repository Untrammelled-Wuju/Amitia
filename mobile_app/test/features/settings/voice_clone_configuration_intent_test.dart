import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/voice_service.dart';
import 'package:amitia_app/features/settings/presentation/widgets/voice_clone_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _Api extends Fake implements BackendServiceApi {
  _Api(this.generation);
  @override
  final int generation;
}

class _Voices extends Fake implements TTSService {
  final replies = [
    Completer<List<Map<String, dynamic>>>(),
    Completer<List<Map<String, dynamic>>>(),
  ];
  int reads = 0;
  @override
  Future<List<Map<String, dynamic>>> listClonedVoices() =>
      replies[reads++].future;
}

void main() {
  testWidgets('切配置归属后旧音色读取结果不能覆盖新 Core 音色', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final voices = _Voices();
    final container = ProviderContainer(overrides: [
      rawBackendServiceApiProvider.overrideWithValue(_Api(1)),
      ttsServiceProvider.overrideWithValue(voices),
    ]);
    addTearDown(container.dispose);
    Future<void> render(int generation) async {
      await tester.pumpWidget(UncontrolledProviderScope(
        container: container,
        child: MaterialApp(home: Scaffold(body: VoiceCloneManager(
          configurationIntent: CoreConfigurationIntent(
            generation: generation,
            coreId: null,
            canConfigure: true,
            isCurrent: () => true,
          ),
        ))),
      ));
      await tester.pump();
    }

    await render(1);
    expect(voices.reads, 1);
    container.updateOverrides([
      rawBackendServiceApiProvider.overrideWithValue(_Api(2)),
      ttsServiceProvider.overrideWithValue(voices),
    ]);
    await render(2);
    expect(voices.reads, 2);
    voices.replies[1].complete([{'speakerId': 'same-id', 'name': 'Core C 音色'}]);
    await tester.pumpAndSettle();
    expect(find.text('Core C 音色'), findsOneWidget);
    voices.replies[0].complete([{'speakerId': 'same-id', 'name': 'Core B 旧音色'}]);
    await tester.pumpAndSettle();
    expect(find.text('Core C 音色'), findsOneWidget);
    expect(find.text('Core B 旧音色'), findsNothing);
    expect(find.textContaining('加载失败'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
