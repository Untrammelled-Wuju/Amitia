import 'dart:convert';
import 'dart:typed_data';

import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_error.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/widgets/character_avatar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class AvatarApi implements BackendServiceApi {
  final versions = <String>[];

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    expect(path, '/api/characters/role-1/avatar');
    final version = queryParameters!['v'] as String;
    versions.add(version);
    return Stream.value(utf8.encode(version));
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test(
    'only stored avatars on the active backend use authenticated transport',
    () {
      final backend = Uri.parse('http://localhost:18899');
      expect(isStoredCharacterAvatar('/avatars/new.png', backend), isTrue);
      expect(
        isStoredCharacterAvatar(
          'http://localhost:18899/avatars/new.png',
          backend,
        ),
        isTrue,
      );
      expect(
        isStoredCharacterAvatar(
          'https://external.test/avatars/new.png',
          backend,
        ),
        isFalse,
      );
      expect(
        isStoredCharacterAvatar('//external.test/avatars/new.png', backend),
        isFalse,
      );
      expect(
        isStoredCharacterAvatar('data:image/png;base64,AA==', backend),
        isFalse,
      );
    },
  );

  test(
    'new upload addresses fetch new bytes instead of retaining the old avatar',
    () async {
      final api = AvatarApi();
      final container = ProviderContainer(
        overrides: [
          backendServiceProvider.overrideWithValue(api),
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
      addTearDown(container.dispose);
      final firstSubscription = container.listen(
        characterAvatarBytesProvider((characterId: 'role-1', avatar: '/avatars/first.png')),
        (_, __) {},
      );
      final nextSubscription = container.listen(
        characterAvatarBytesProvider((characterId: 'role-1', avatar: '/avatars/next.png')),
        (_, __) {},
      );
      addTearDown(firstSubscription.close);
      addTearDown(nextSubscription.close);
      final first = await container.read(
        characterAvatarBytesProvider((
          characterId: 'role-1',
          avatar: '/avatars/first.png',
        )).future,
      );
      final next = await container.read(
        characterAvatarBytesProvider((
          characterId: 'role-1',
          avatar: '/avatars/next.png',
        )).future,
      );
      expect(utf8.decode(first), '/avatars/first.png');
      expect(utf8.decode(next), '/avatars/next.png');
      expect(api.versions, ['/avatars/first.png', '/avatars/next.png']);
    },
  );

  testWidgets('inline avatars appear and replacement data updates the image', (
    tester,
  ) async {
    final png = Uint8List.fromList(
      base64Decode(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aA1cAAAAASUVORK5CYII=',
      ),
    );
    final container = ProviderContainer(
      overrides: [
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
    addTearDown(container.dispose);
    Widget screen(String avatar) => UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(
          body: CharacterAvatar(
            characterId: 'role-1',
            avatar: avatar,
            initial: '角',
          ),
        ),
      ),
    );
    await tester.pumpWidget(screen(''));
    expect(find.text('角'), findsOneWidget);
    await tester.pumpWidget(
      screen('data:image/png;base64,${base64Encode(png)}'),
    );
    await tester.pumpAndSettle();
    expect(find.byType(Image), findsOneWidget);
    expect(
      (tester.widget<Image>(find.byType(Image)).image as MemoryImage).bytes,
      png,
    );
    await tester.pumpWidget(screen(''));
    expect(find.byType(Image), findsNothing);
    expect(find.text('角'), findsOneWidget);
  });
}
