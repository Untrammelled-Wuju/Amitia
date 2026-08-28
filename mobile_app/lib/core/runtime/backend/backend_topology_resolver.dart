import 'mobile_deployment_mode.dart';
import 'backend_topology.dart';

export 'mobile_deployment_mode.dart' show DeploymentConfigValidationError;

abstract interface class BackendTopologyResolver {
  MobileBackendTopology resolve(MobileDeploymentConfig config);
}

class DefaultBackendTopologyResolver implements BackendTopologyResolver {
  final Uri Function()? localEndpointProvider;

  const DefaultBackendTopologyResolver({this.localEndpointProvider});

  @override
  MobileBackendTopology resolve(MobileDeploymentConfig config) {
    switch (config.mode) {
      case MobileDeploymentMode.local:
        return _resolveLocal(config);
      case MobileDeploymentMode.cloud:
        return _resolveCloud(config);
    }
  }

  MobileBackendTopology _resolveLocal(MobileDeploymentConfig config) {
    final localEp =
        localEndpointProvider?.call() ?? Uri.parse('http://127.0.0.1:18899');
    final normalized = normalizeLocalUri(localEp);
    final wsUri = toWebSocketUri(normalized);
    final endpoint = BackendEndpoint(
      role: BackendEndpointRole.businessCore,
      httpBaseUri: normalized,
      websocketBaseUri: wsUri,
      isRemote: false,
    );
    return MobileBackendTopology(
      mode: MobileDeploymentMode.local,
      businessCore: endpoint,
      localRuntime: endpoint,
      embeddedRuntimeProfile: EmbeddedRuntimeProfile.local,
      requiresEmbeddedRuntime: true,
    );
  }

  MobileBackendTopology _resolveCloud(MobileDeploymentConfig config) {
    final raw = config.remoteCoreUri;
    if (raw == null || raw.trim().isEmpty) {
      throw DeploymentConfigValidationError(
        'cloud mode requires remote core URI',
      );
    }
    final normalized = normalizeRemoteCoreUri(raw);
    final wsUri = toWebSocketUri(normalized);
    final endpoint = BackendEndpoint(
      role: BackendEndpointRole.businessCore,
      httpBaseUri: normalized,
      websocketBaseUri: wsUri,
      isRemote: true,
    );
    final localEp =
        localEndpointProvider?.call() ?? Uri.parse('http://127.0.0.1:18899');
    final localNormalized = normalizeLocalUri(localEp);
    final localEndpoint = BackendEndpoint(
      role: BackendEndpointRole.localRuntime,
      httpBaseUri: localNormalized,
      websocketBaseUri: toWebSocketUri(localNormalized),
      isRemote: false,
    );
    return MobileBackendTopology(
      mode: MobileDeploymentMode.cloud,
      businessCore: endpoint,
      localRuntime: localEndpoint,
      embeddedRuntimeProfile: EmbeddedRuntimeProfile.deviceAgent,
      requiresEmbeddedRuntime: true,
    );
  }
}

Uri normalizeRemoteCoreUri(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) {
    throw DeploymentConfigValidationError('remote core URI is empty');
  }
  var parsed = Uri.tryParse(trimmed);
  if (parsed == null || !parsed.hasScheme) {
    if (trimmed.startsWith('//')) {
      parsed = Uri.tryParse('https:$trimmed');
    } else {
      parsed = Uri.tryParse('https://$trimmed');
    }
  }
  if (parsed == null || !parsed.hasScheme) {
    throw DeploymentConfigValidationError('invalid remote core URI: $raw');
  }
  final scheme = parsed.scheme.toLowerCase();
  if (scheme != 'http' && scheme != 'https') {
    throw DeploymentConfigValidationError(
      'remote core URI must use http or https scheme: $raw',
    );
  }
  if (parsed.userInfo.isNotEmpty) {
    throw DeploymentConfigValidationError(
      'remote core URI must not contain embedded credentials: $raw',
    );
  }
  if (parsed.hasFragment) {
    throw DeploymentConfigValidationError(
      'remote core URI must not contain fragment: $raw',
    );
  }
  if (parsed.hasQuery) {
    throw DeploymentConfigValidationError(
      'remote core URI must not contain query: $raw',
    );
  }
  final host = parsed.host;
  if (host.isEmpty) {
    throw DeploymentConfigValidationError(
      'remote core URI must contain a host: $raw',
    );
  }
  var port = parsed.port;
  if (port == 0) {
    port = scheme == 'https' ? 443 : 80;
  }
  final resolved = Uri(scheme: scheme, host: host, port: port);
  return resolved;
}

Uri toWebSocketUri(Uri httpUri) {
  final scheme = httpUri.scheme == 'https' ? 'wss' : 'ws';
  return httpUri.replace(scheme: scheme);
}

Uri normalizeLocalUri(Uri uri) {
  if (uri.scheme != 'http' && uri.scheme != 'https') {
    return Uri.parse('http://127.0.0.1:${uri.port == 0 ? 18899 : uri.port}');
  }
  return uri;
}
