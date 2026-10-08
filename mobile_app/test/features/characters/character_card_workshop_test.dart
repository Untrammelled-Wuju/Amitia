import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/services/character_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/characters/presentation/pages/character_card_workshop_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _Characters extends Fake implements CharacterService {
  static const authorityValue =
      'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
  @override
  Future<String> authority() async => authorityValue;
  Map<String, dynamic>? created;
  Map<String, dynamic>? updated;
  String currentAuthority = authorityValue;
  @override
  Future<CharacterDto?> create(
    Map<String, dynamic> data, {
    String? roleAuthority,
  }) async {
    expect(roleAuthority, authorityValue);
    if (roleAuthority != currentAuthority) {
      throw StateError('角色数据归属已变化，请重新加载角色');
    }
    created = data;
    return CharacterDto(id: 'new-role', name: data['name'] as String);
  }

  @override
  Future<CharacterDto?> setActive(String id, {String? roleAuthority}) async =>
      CharacterDto(id: id, name: '星河');

  @override
  Future<CharacterDto?> update(
    String id,
    Map<String, dynamic> data, {
    String? roleAuthority,
  }) async {
    expect(roleAuthority, authorityValue);
    updated = data;
    return CharacterDto(id: id, name: data['name'] as String);
  }
}

class _Api extends Fake implements BackendServiceApi {
  bool failLoad = false;
  Map<String, dynamic>? savedCard;
  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => fromJson!({
    'reply': '已生成角色草稿',
    'draft': {
      'name': '星河',
      'identity': '图书管理员',
      'characterBase': '你是星河',
      'scenario': '夜晚的图书馆',
      'personalityConfig': {'warmth': 72},
    },
  });

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (failLoad) throw StateError('offline');
    return fromJson!({
      'extensions': {'keep': true},
    });
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    savedCard = Map<String, dynamic>.from(data as Map);
    return fromJson!(savedCard);
  }
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  Future<void> render(
    WidgetTester tester,
    _Api api,
    _Characters service,
    List<CharacterDto> roles,
  ) async {
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, __) => const CharacterCardWorkshopPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          backendServiceProvider.overrideWithValue(api),
          characterServiceProvider.overrideWithValue(service),
          characterListProvider.overrideWith((ref) async => roles),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets(
    'generated mobile card stays editable and saves all personality fields',
    (tester) async {
      final api = _Api();
      final service = _Characters();
      await render(tester, api, service, []);
      expect(find.text('第 1 步：对话生成'), findsNothing);
      expect(find.text('保存角色卡'), findsNothing);
      await tester.tap(find.text('创建角色卡').last);
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, '生成图书管理员');
      await tester.tap(find.byTooltip('发送'));
      await tester.pumpAndSettle();
      expect(service.created, isNull);
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      final name = find.widgetWithText(TextField, '名称');
      await tester.ensureVisible(name);
      await tester.enterText(name, '手动调整的名称');
      FocusManager.instance.primaryFocus?.unfocus();
      await tester.pumpAndSettle();
      final save = find.text('保存角色卡');
      await tester.ensureVisible(save);
      await tester.pumpAndSettle();
      await tester.tap(save);
      await tester.pumpAndSettle();
      expect(service.created?['name'], '手动调整的名称');
      expect(service.created?['identity'], '图书管理员');
      expect(service.created?['characterBase'], '你是星河');
      expect((service.created?['personalityConfig'] as Map).length, 32);
      expect((service.created?['personalityConfig'] as Map)['warmth'], 72);
      expect(api.savedCard?['systemPrompt'], '你是星河');
      expect(api.savedCard?['scenario'], '夜晚的图书馆');
      expect(find.text('角色卡工坊'), findsOneWidget);
      expect(find.text('角色需求'), findsNothing);
    },
  );

  testWidgets(
    'draft captured before Core switch cannot be saved to the new Core',
    (tester) async {
      final service = _Characters();
      final api = _Api();
      await render(tester, api, service, []);
      await tester.tap(find.text('创建角色卡').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, '名称'), '设备草稿');
      FocusManager.instance.primaryFocus?.unfocus();
      await tester.pumpAndSettle();
      service.currentAuthority = 'b' * 64;
      final save = find.text('保存角色卡');
      await tester.ensureVisible(save);
      await tester.pumpAndSettle();
      await tester.tap(save);
      await tester.pumpAndSettle();
      expect(service.created, isNull);
      expect(api.savedCard, isNull);
      expect(find.textContaining('角色数据归属已变化'), findsOneWidget);
      expect(find.text('角色卡已保存'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'next opens a blank editor without generating and keeps manual edits',
    (tester) async {
      final service = _Characters();
      await render(tester, _Api(), service, []);
      await tester.tap(find.text('创建角色卡').last);
      await tester.pumpAndSettle();
      expect(find.text('角色设计助手'), findsOneWidget);
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      for (final label in ['名称', '身份', '角色描述', 'System Prompt', '场景设定']) {
        final field = tester.widget<TextField>(
          find.widgetWithText(TextField, label),
        );
        expect(field.controller!.text, isEmpty);
      }
      final name = find.widgetWithText(TextField, '名称');
      await tester.enterText(name, '手动角色');
      FocusManager.instance.primaryFocus?.unfocus();
      await tester.tap(find.byTooltip('上一步'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      expect(tester.widget<TextField>(name).controller!.text, '手动角色');
      expect(service.created, isNull);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'creation back navigation preserves draft and returns to workshop',
    (tester) async {
      await render(tester, _Api(), _Characters(), []);
      await tester.tap(find.text('创建角色卡').last);
      await tester.pumpAndSettle();
      expect(find.text('第 1 步：对话生成'), findsOneWidget);
      expect(find.text('编辑角色'), findsNothing);
      await tester.enterText(find.byType(TextField).first, '生成图书管理员');
      await tester.tap(find.byTooltip('发送'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      expect(find.text('第 2 步：编辑角色卡'), findsOneWidget);
      await tester.tap(find.byTooltip('上一步'));
      await tester.pumpAndSettle();
      expect(find.text('已生成角色草稿'), findsOneWidget);
      expect(find.text('第 1 步：对话生成'), findsOneWidget);
      await tester.tap(find.text('下一步：编辑角色卡'));
      await tester.pumpAndSettle();
      expect(find.text('第 2 步：编辑角色卡'), findsOneWidget);
      await tester.tap(find.byTooltip('上一步'));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('返回工坊'));
      await tester.pumpAndSettle();
      expect(find.text('角色卡工坊'), findsOneWidget);
      expect(find.text('角色需求'), findsNothing);
    },
  );

  testWidgets('failed card load blocks editing until retry succeeds', (
    tester,
  ) async {
    final api = _Api()..failLoad = true;
    await render(tester, api, _Characters(), [
      CharacterDto(
        id: 'existing',
        name: '原角色',
        roleAuthority: _Characters.authorityValue,
      ),
    ]);
    expect(find.text('角色需求'), findsNothing);
    await tester.tap(find.text('原角色'));
    await tester.pumpAndSettle();
    expect(find.text('角色卡加载失败，请重试后再编辑'), findsOneWidget);
    expect(find.text('保存角色卡'), findsNothing);
    api.failLoad = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('保存角色卡'), findsOneWidget);
  });

  testWidgets('editing existing card preserves imported extra data', (
    tester,
  ) async {
    final api = _Api();
    final service = _Characters();
    await render(tester, api, service, [
      CharacterDto(
        id: 'existing',
        name: '原角色',
        characterBase: '原提示词',
        roleAuthority: _Characters.authorityValue,
      ),
    ]);
    await tester.tap(find.text('原角色'));
    await tester.pumpAndSettle();
    final save = find.text('保存角色卡');
    await tester.ensureVisible(save);
    await tester.pumpAndSettle();
    await tester.tap(save);
    await tester.pumpAndSettle();
    expect(service.created, isNull);
    expect(service.updated?['characterBase'], '原提示词');
    expect(api.savedCard?['extensions'], {'keep': true});
  });
}
