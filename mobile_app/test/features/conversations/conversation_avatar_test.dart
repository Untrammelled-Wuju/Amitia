import 'dart:convert';

import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_error.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/character_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/widgets/character_avatar.dart';
import 'package:amitia_app/features/conversations/presentation/widgets/conversation_list_tile.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

const _png =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aA1cAAAAASUVORK5CYII=';
const _avatar = 'data:image/png;base64,$_png';

class _UnusedApi implements BackendServiceApi {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _Characters extends CharacterService {
  _Characters(this.characters) : super(_UnusedApi());

  final Map<String, CharacterDto> characters;
  final requested = <String>[];

  @override
  Future<CharacterDto?> getById(String id) async {
    requested.add(id);
    return characters[id];
  }
}

void main() {
  late List<CharacterDto> visible;
  late _Characters service;
  late ProviderContainer container;

  setUp(() {
    visible = [];
    service = _Characters({});
    container = ProviderContainer(
      overrides: [
        characterListProvider.overrideWith((ref) async => visible),
        characterServiceProvider.overrideWithValue(service),
        backendConnectionProvider.overrideWith(
          (ref) async => const BackendConnectionUnavailable(
            BackendConnectionError(
              BackendConnectionErrorCode.RUNTIME_NOT_READY,
              'test',
            ),
          ),
        ),
      ],
    );
  });

  tearDown(() => container.dispose());

  Future<void> show(
    WidgetTester tester,
    String roleId, {
    CharacterDto? character,
  }) async {
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: ConversationListTile(
              conversation: ConversationDto(
                id: 'conversation',
                characterId: roleId,
                title: '你好',
              ),
              character: character,
              timeText: '',
              showDivider: false,
              onTap: () {},
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('使用关联角色图片而非对话标题首字', (tester) async {
    visible = [CharacterDto(id: 'role-a', name: '小夏', avatar: _avatar)];
    await show(tester, 'role-a');
    final image = tester.widget<Image>(find.byType(Image));
    expect((image.image as MemoryImage).bytes, base64Decode(_png));
    expect(find.text('你'), findsNothing);
    expect(service.requested, isEmpty);
  });

  testWidgets('关联角色未在启用列表时按其编号读取头像', (tester) async {
    service.characters['disabled'] = CharacterDto(
      id: 'disabled',
      name: '历史角色',
      avatar: _avatar,
    );
    await show(tester, 'disabled');
    expect(find.byType(Image), findsOneWidget);
    expect(service.requested, ['disabled']);
  });

  testWidgets('没有头像或角色已删除时使用图标', (tester) async {
    visible = [CharacterDto(id: 'role-a', name: '小夏')];
    await show(tester, 'role-a');
    expect(find.byIcon(Icons.person_rounded), findsOneWidget);
    expect(find.text('你'), findsNothing);
    expect(find.text('小'), findsNothing);
    await show(tester, 'deleted');
    expect(find.byIcon(Icons.person_rounded), findsOneWidget);
    expect(service.requested, ['deleted']);
  });

  testWidgets('角色切换后不会沿用上一个角色头像', (tester) async {
    final previous = CharacterDto(id: 'role-a', name: '小夏', avatar: _avatar);
    visible = [previous, CharacterDto(id: 'role-b', name: '小林')];
    await show(tester, 'role-a', character: previous);
    expect(find.byType(Image), findsOneWidget);
    await show(tester, 'role-b', character: previous);
    expect(find.byType(Image), findsNothing);
    final avatar = tester.widget<CharacterAvatar>(find.byType(CharacterAvatar));
    expect(avatar.characterId, 'role-b');
    expect(avatar.initial, '小林');
  });

  testWidgets('角色头像更新后列表重新显示新头像', (tester) async {
    visible = [CharacterDto(id: 'role-a', name: '小夏')];
    await show(tester, 'role-a');
    expect(find.byType(Image), findsNothing);
    visible = [CharacterDto(id: 'role-a', name: '小夏', avatar: _avatar)];
    container.invalidate(characterListProvider);
    await tester.pumpAndSettle();
    expect(find.byType(Image), findsOneWidget);
    expect(find.text('你'), findsNothing);
  });
}
