import 'package:dio/dio.dart';
import 'package:riverpod/riverpod.dart';

import '../backend_connection/backend_connection_config.dart';
import '../backend_transport/http/backend_http_transport.dart';
import 'artifact_service.dart';

final artifactDioProvider = Provider<Dio>((ref) {
  final connection = ref.watch(backendConnectionConfigProvider);
  final baseUri = connection.httpBaseUrl;
  return Dio(BaseOptions(
    baseUrl: baseUri.toString(),
    connectTimeout: const Duration(seconds: 10),
    receiveTimeout: const Duration(seconds: 120),
    headers: {
      'Accept': 'application/json',
    },
  ));
});

final artifactServiceProvider = Provider<ArtifactService>((ref) {
  final connection = ref.watch(backendConnectionConfigProvider);
  final dio = ref.watch(artifactDioProvider);
  final service = HttpArtifactService(
    dio: dio,
    baseUrl: connection.httpBaseUrl,
  );
  ref.onDispose(() => dio.close(force: true));
  return service);
});

final uploadedAttachmentsProvider = StateNotifierProvider<
    UploadedAttachmentsNotifier, List<ArtifactMetadata>>((ref) {
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
