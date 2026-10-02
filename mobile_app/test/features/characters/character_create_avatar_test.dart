import 'dart:convert';

import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/services/character_detail_service.dart';
import 'package:amitia_app/core/services/character_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/widgets/profile_avatar.dart';
import 'package:amitia_app/features/characters/presentation/pages/character_create_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker_platform_interface/image_picker_platform_interface.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _Picker extends ImagePickerPlatform {
  XFile? image;

  @override
  Future<XFile?> getImageFromSource({
    required ImageSource source,
    ImagePickerOptions options = const ImagePickerOptions(),
  }) async => image;
}

class _Characters extends Fake implements CharacterService {
  int creates = 0;
  Map<String, dynamic>? payload;

  @override
  Future<CharacterDto?> create(Map<String, dynamic> data) async {
    creates++;
    payload = data;
    return CharacterDto(id: 'created-character', name: data['name'] as String);
  }
}

class _Details extends Fake implements CharacterDetailService {
  bool fail = false;
  final uploads = <(String, String)>[];

  @override
  Future<Map<String, dynamic>?> uploadAvatar(String id, String path) async {
    uploads.add((id, path));
    if (fail) throw StateError('upload failed');
    return {'avatarUrl': '/avatars/created.png'};
  }
}

void main() {
  late _Picker picker;
  late ImagePickerPlatform originalPicker;

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    originalPicker = ImagePickerPlatform.instance;
    picker = _Picker();
    ImagePickerPlatform.instance = picker;
  });
  tearDown(() => ImagePickerPlatform.instance = originalPicker);

  Future<(_Characters, _Details)> render(WidgetTester tester) async {
    final characters = _Characters();
    final details = _Details();
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, __) => const Scaffold(body: Text('角色列表')),
        ),
        GoRoute(
          path: '/create',
          builder: (_, __) => const CharacterCreatePage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          characterServiceProvider.overrideWithValue(characters),
          characterDetailServiceProvider.overrideWithValue(details),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    router.push('/create');
    await tester.pumpAndSettle();
    expect(find.text('上传角色头像'), findsOneWidget);
    expect(find.text('选择角色主题色'), findsNothing);
    return (characters, details);
  }

  void selectImage() {
    picker.image = XFile.fromData(
      base64Decode(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aR1cAAAAASUVORK5CYII=',
      ),
      path: '/selected-avatar.png',
      mimeType: 'image/png',
    );
  }

  Future<void> preview(WidgetTester tester) async {
    for (var step = 0; step < 6; step++) {
      if (step == 1) {
        await tester.enterText(find.byType(TextField).first, '小夏');
      }
      await tester.tap(find.text('下一步'));
      await tester.pumpAndSettle();
    }
  }

  testWidgets(
    'without an image, preview uses the name and creates without upload',
    (tester) async {
      final (characters, details) = await render(tester);
      await preview(tester);
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).avatar,
        '',
      );
      expect(find.text('小'), findsOneWidget);
      await tester.tap(find.text('完成创建'));
      await tester.pumpAndSettle();
      expect(characters.creates, 1);
      expect(characters.payload?['name'], '小夏');
      expect(details.uploads, isEmpty);
      expect(find.text('角色列表'), findsOneWidget);
    },
  );

  testWidgets(
    'selected image appears in preview and uploads against the created ID',
    (tester) async {
      final (characters, details) = await render(tester);
      selectImage();
      await tester.tap(find.text('选择图片'));
      await tester.pumpAndSettle();
      await preview(tester);
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).avatar,
        startsWith('data:image/'),
      );
      await tester.tap(find.text('完成创建'));
      await tester.pumpAndSettle();
      expect(characters.creates, 1);
      expect(details.uploads, [('created-character', '/selected-avatar.png')]);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'upload failure keeps the created character and does not offer duplicate creation',
    (tester) async {
      final (characters, details) = await render(tester);
      details.fail = true;
      selectImage();
      await tester.tap(find.text('选择图片'));
      await tester.pumpAndSettle();
      await preview(tester);
      await tester.tap(find.text('完成创建'));
      await tester.pumpAndSettle();
      expect(characters.creates, 1);
      expect(details.uploads, hasLength(1));
      expect(find.textContaining('头像上传失败'), findsOneWidget);
      expect(find.text('完成创建'), findsNothing);
    },
  );

  testWidgets(
    'cancelling replacement preserves the image, removing it restores text',
    (tester) async {
      final (_, details) = await render(tester);
      selectImage();
      await tester.tap(find.text('选择图片'));
      await tester.pumpAndSettle();
      final selected = tester
          .widget<ProfileAvatar>(find.byType(ProfileAvatar))
          .avatar;
      picker.image = null;
      await tester.tap(find.text('更换头像'));
      await tester.pumpAndSettle();
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).avatar,
        selected,
      );
      await tester.tap(find.text('使用文字头像'));
      await tester.pumpAndSettle();
      await preview(tester);
      expect(
        tester.widget<ProfileAvatar>(find.byType(ProfileAvatar)).avatar,
        '',
      );
      await tester.tap(find.text('完成创建'));
      await tester.pumpAndSettle();
      expect(details.uploads, isEmpty);
    },
  );
}
