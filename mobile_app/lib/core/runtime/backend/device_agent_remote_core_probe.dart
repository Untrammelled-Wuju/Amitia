import 'package:dio/dio.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/providers/runtime_backend_connection_source.dart';
import '../../backend_transport/connectivity/backend_connectivity_probe.dart';
import 'mobile_backend_lifecycle.dart';

typedef LocalConnectionResolver =
    Future<BackendConnectionAvailability> Function();

final class DeviceAgentRemoteCoreProbe implements RemoteCoreProbe {
  final LocalConnectionResolver _resolveLocal;

  DeviceAgentRemoteCoreProbe({LocalConnectionResolver? resolveLocal})
    : _resolveLocal =
          resolveLocal ??
          (() => const RuntimeBackendConnectionSource().resolve());

  @override
  Future<BackendConnectivityResult> probe(
    Uri baseUri, {
    Duration timeout = const Duration(seconds: 5),
  }) async {
    final local = await _resolveLocal();
    if (local is! BackendConnectionAvailable) {
      return BackendConnectivityResult.unreachable;
    }

    final config = local.config;
    final proxyBase = Uri(
      scheme: config.endpoint.httpScheme,
      host: config.endpoint.host,
      port: config.endpoint.port,
    );
    final proxyPrefix = config.endpoint.pathPrefix.isEmpty
        ? '/internal/device-mesh/provider'
        : config.endpoint.pathPrefix;
    final readinessPath = '$proxyPrefix${config.endpoint.readinessPath}';
    final livenessPath = '$proxyPrefix${config.endpoint.livenessPath}';
    final dio = Dio(
      BaseOptions(
        baseUrl: proxyBase.toString(),
        connectTimeout: timeout,
        receiveTimeout: timeout,
        sendTimeout: timeout,
        validateStatus: (status) => status != null,
        headers: <String, String>{
          'Accept': 'application/json',
          'X-Amitia-Local-Token': config.credential.revealForTransport(),
        },
      ),
    );
    try {
      final ready = await dio.get<dynamic>(readinessPath);
      if (ready.statusCode == 200) return BackendConnectivityResult.ready;
      if (ready.statusCode != 404 && ready.statusCode != 405) {
        return BackendConnectivityResult.unreachable;
      }
      final live = await dio.get<dynamic>(livenessPath);
      return live.statusCode == 200
          ? BackendConnectivityResult.live
          : BackendConnectivityResult.unreachable;
    } on DioException catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      return BackendConnectivityResult.unreachable;
    } finally {
      dio.close(force: true);
    }
  }
}
