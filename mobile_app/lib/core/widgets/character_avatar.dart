import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';

import '../../app/theme/app_colors.dart';
import '../backend_connection/providers/backend_connection_providers.dart';
import '../backend_connection/backend_connection_availability.dart';
import '../backend_transport/providers/backend_transport_providers.dart';

typedef CharacterAvatarKey = ({String characterId, String avatar});

final characterAvatarBytesProvider = FutureProvider.autoDispose
    .family<Uint8List, CharacterAvatarKey>((ref, key) async {
      await ref.watch(backendConnectionProvider.future);
      final api = ref.watch(backendServiceProvider);
      final cancelToken = CancelToken();
      ref.onDispose(cancelToken.cancel);
      final stream = await api.getStream(
        '/api/characters/${Uri.encodeComponent(key.characterId)}/avatar',
        queryParameters: {'v': key.avatar},
        cancelToken: cancelToken,
      );
      final bytes = BytesBuilder(copy: false);
      await for (final chunk in stream) {
        bytes.add(chunk);
      }
      return bytes.takeBytes();
    });

bool isStoredCharacterAvatar(String avatar, Uri? backend) {
  final uri = Uri.tryParse(avatar.trim());
  if (uri == null || !uri.path.startsWith('/avatars/')) return false;
  return !uri.hasAuthority ||
      (backend != null &&
          (uri.scheme == 'http' || uri.scheme == 'https') &&
          uri.origin == backend.origin);
}

class CharacterAvatar extends ConsumerWidget {
  final String characterId;
  final String avatar;
  final String initial;
  final double size;
  final Color? color;
  final BorderRadius? borderRadius;

  const CharacterAvatar({
    super.key,
    required this.characterId,
    required this.avatar,
    required this.initial,
    this.size = 48,
    this.color,
    this.borderRadius,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final fallback = Center(
      child: Text(
        initial.trim().isEmpty ? '?' : initial.trim().characters.first,
        style: TextStyle(
          color: Colors.white,
          fontSize: size * 0.42,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
    Widget image = fallback;
    final value = avatar.trim();
    if (value.isNotEmpty) {
      final connection = ref.watch(backendConnectionProvider).valueOrNull;
      final backend = connection is BackendConnectionAvailable
          ? Uri(
              scheme: connection.config.endpoint.httpScheme,
              host: connection.config.endpoint.host,
              port: connection.config.endpoint.port,
            )
          : null;
      if (isStoredCharacterAvatar(value, backend) && characterId.isNotEmpty) {
        final bytes = ref
            .watch(
              characterAvatarBytesProvider((
                characterId: characterId,
                avatar: value,
              )),
            )
            .valueOrNull;
        if (bytes != null) {
          image = Image.memory(
            bytes,
            fit: BoxFit.cover,
            errorBuilder: (_, __, ___) => fallback,
          );
        }
      } else if (value.startsWith('data:image/')) {
        try {
          image = Image.memory(
            base64Decode(value.substring(value.indexOf(',') + 1)),
            fit: BoxFit.cover,
            errorBuilder: (_, __, ___) => fallback,
          );
        } catch (_) {}
      } else if (value.startsWith('https://') || value.startsWith('http://')) {
        image = Image.network(
          value,
          fit: BoxFit.cover,
          errorBuilder: (_, __, ___) => fallback,
        );
      }
    }
    return Container(
      width: size,
      height: size,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: color ?? context.accentPrimary,
        borderRadius: borderRadius ?? BorderRadius.circular(size / 2),
      ),
      child: image,
    );
  }
}
