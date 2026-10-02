import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/characters/presentation/widgets/character_generation_chat.dart';
import 'package:amitia_app/features/characters/presentation/widgets/character_personality_editor.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  final calls = <Map<String, dynamic>>[];
  final replies = <Map<String, dynamic>>[
    {
      'reply': '已生成',
      'draft': {
        'name': '星河',
        'personalityConfig': {'warmth': 72},
      },
    },
    {
      'reply': '已完善',
      'draft': {'identity': '图书管理员'},
    },
    {
      'reply': '已调整',
      'draft': {'personality': '温和'},
    },
  ];
  bool fail = false;
  Completer<void>? pending;
  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(Map<String, dynamic>.from(data as Map));
    if (pending != null) await pending!.future;
    if (fail) throw StateError('offline');
    return fromJson!(replies.removeAt(0));
  }
}

void main() {
  testWidgets('sent message is visible before reply and next stays available', (tester) async {
    final api = _Api()..pending = Completer<void>();
    final applied = <Map<String, dynamic>>[];
    await tester.pumpWidget(ProviderScope(
      overrides: [backendServiceProvider.overrideWithValue(api)],
      child: MaterialApp(home: Scaffold(body: CharacterGenerationChat(
        currentDraft: () => {'name': ''},
        onApply: applied.add,
      ))),
    ));
    await tester.enterText(find.byType(TextField), '设计图书管理员');
    await tester.tap(find.byTooltip('发送'));
    await tester.pump();
    expect(find.text('设计图书管理员'), findsWidgets);
    expect(find.text('正在整理角色草稿…'), findsOneWidget);
    await tester.tap(find.text('下一步：编辑角色卡'));
    await tester.pump();
    expect(applied, [{'name': ''}]);
    api.pending!.complete();
    await tester.pumpAndSettle();
    expect(applied.length, 1);
    expect(tester.takeException(), isNull);
  });
  test('mobile exposes all 32 personality defaults', () {
    expect(characterPersonalityDefaults.length, 32);
    expect(
      characterPersonalityDefaults.values.every(
        (value) => value >= 0 && value <= 100,
      ),
      true,
    );
  });
  testWidgets('multi-turn draft applies without replacing later manual edits', (
    tester,
  ) async {
    final api = _Api();
    var current = <String, dynamic>{
      'name': '旧名',
      'personalityConfig': {'warmth': 50},
    };
    Map<String, dynamic>? applied;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [backendServiceProvider.overrideWithValue(api)],
        child: MaterialApp(
          home: Scaffold(
            body: CharacterGenerationChat(
              currentDraft: () => current,
              onApply: (value) => applied = value,
            ),
          ),
        ),
      ),
    );
    Future<void> send(String text) async {
      await tester.enterText(find.byType(TextField), text);
      await tester.tap(find.byTooltip('发送'));
      await tester.pumpAndSettle();
    }

    await send('喜欢天文');
    await send('图书管理员');
    expect((api.calls[1]['messages'] as List).length, 3);
    expect((api.calls[1]['draft'] as Map)['name'], '星河');
    await tester.tap(find.text('下一步：编辑角色卡'));
    await tester.pumpAndSettle();
    expect(applied?['name'], '星河');
    expect(applied?['identity'], '图书管理员');
    current = {
      'name': '手工改名',
      'personalityConfig': {'warmth': 60},
    };
    await send('温和一些');
    expect((api.calls[2]['draft'] as Map)['name'], '手工改名');
    expect(
      ((api.calls[2]['draft'] as Map)['personalityConfig'] as Map)['warmth'],
      60,
    );
    expect(tester.takeException(), isNull);
  });
  testWidgets('failure preserves the user input and does not apply a draft', (
    tester,
  ) async {
    final api = _Api()..fail = true;
    var applied = false;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [backendServiceProvider.overrideWithValue(api)],
        child: MaterialApp(
          home: Scaffold(
            body: CharacterGenerationChat(
              currentDraft: () => {'name': '原角色'},
              onApply: (_) => applied = true,
            ),
          ),
        ),
      ),
    );
    await tester.enterText(find.byType(TextField), '完善设定');
    await tester.tap(find.byTooltip('发送'));
    await tester.pumpAndSettle();
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      '完善设定',
    );
    expect(applied, false);
    expect(find.textContaining('生成失败'), findsOneWidget);
  });
}
