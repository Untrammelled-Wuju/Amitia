import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_connection/backend_connection_availability.dart';
import '../backend_connection/backend_connection_config.dart';
import '../backend_connection/backend_uri_builder.dart';
import '../backend_connection/providers/backend_connection_providers.dart';
import '../backend_transport/auth/backend_auth_header.dart';
import 'artifact_model.dart';
import 'artifact_service.dart';

final artifactServiceProvider = FutureProvider<ArtifactService>((ref) async {
  final availability = await ref.watch(backendConnectionProvider.future);
  if (availability is! BackendConnectionAvailable) {
    throw StateError('后端当前不可用');
  }
  final config = availability.config;
  final dio = createAuthenticatedDio(config);
  ref.onDispose(() => dio.close(force: true));
  return HttpArtifactService(
    dio: dio,
    baseUrl: BackendUriBuilder().httpBase(config).toString(),
  );
});

Dio createAuthenticatedDio(BackendConnectionConfig config) {
  final token = config.credential.revealForTransport();
  final headers = <String, dynamic>{'Accept': 'application/json'};
  switch (config.authStrategy) {
    case BackendAuthStrategy.localToken:
      headers[BackendAuthHeader.localToken] = token;
    case BackendAuthStrategy.bearer:
      headers[BackendAuthHeader.authorization] = 'Bearer $token';
  }
  return Dio(
    BaseOptions(
      baseUrl: BackendUriBuilder().httpBase(config).toString(),
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 120),
      headers: headers,
    ),
  );
}

final uploadedAttachmentsProvider = StateNotifierProvider<UploadedAttachmentsNotifier, List<ArtifactMetadata>>((ref) {
  return UploadedAttachmentsNotifier();
});

class UploadedAttachmentsNotifier extends StateNotifier<List<ArtifactMetadata>> {
  UploadedAttachmentsNotifier() : super([]);

  void add(ArtifactMetadata artifact) {
    state = [...state, artifact];
  }

  void remove(String artifactId) {
    state = state.where((a) => a.id != artifactId).toList();
  }

  void clear() {
    state = [];
  }
}
