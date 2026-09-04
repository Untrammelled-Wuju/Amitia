import 'mobile_deployment_mode.dart';
import 'backend_topology.dart';


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

  final candidate = trimmed.startsWith('//') ? trimmed.substring(2) : trimmed;
  Uri? parsed;
  if (_hasHttpScheme(candidate)) {
    parsed = Uri.tryParse(candidate);
  } else {
    final probe = Uri.tryParse('http://$candidate');
    final host = probe?.host.trim().toLowerCase() ?? '';
    if (host.isEmpty) {
      throw DeploymentConfigValidationError('invalid remote core URI: $raw');
    }
    final scheme = _isPrivateOrLoopbackHost(host) ? 'http' : 'https';
    parsed = Uri.tryParse('$scheme://$candidate');
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
  final normalizedPath = parsed.path.replaceAll(RegExp(r'/+$'), '');
  if (normalizedPath.isNotEmpty) {
    throw DeploymentConfigValidationError(
      'remote core URI must point to the core root, subpaths are not supported: $raw',
    );
  }
  final host = parsed.host.toLowerCase();
  if (host.isEmpty) {
    throw DeploymentConfigValidationError(
      'remote core URI must contain a host: $raw',
    );
  }

  final explicitPort = parsed.hasPort ? parsed.port : null;
  final canonicalPort = (scheme == 'https' && explicitPort == 443) ||
          (scheme == 'http' && explicitPort == 80)
      ? null
      : explicitPort;
  return Uri(
    scheme: scheme,
    host: host,
    port: canonicalPort,
  );
}

bool _hasHttpScheme(String raw) => RegExp(r'^https?://', caseSensitive: false).hasMatch(raw);

bool _isPrivateOrLoopbackHost(String hostname) {
  final host = hostname.toLowerCase();
  if (host == 'localhost' || host.endsWith('.localhost')) return true;
  if (host == '::1' || host == '0:0:0:0:0:0:0:1') return true;
  if (RegExp(r'^(fc|fd)[0-9a-f]{2}:', caseSensitive: false).hasMatch(host) ||
      host.startsWith('fe80:')) {
    return true;
  }
  final parts = host.split('.');
  if (parts.length != 4) return false;
  final octets = parts.map(int.tryParse).toList(growable: false);
  if (octets.any((part) => part == null || part < 0 || part > 255)) return false;
  final a = octets[0]!;
  final b = octets[1]!;
  if (a == 127 || a == 10) return true;
  if (a == 192 && b == 168) return true;
  if (a == 172 && b >= 16 && b <= 31) return true;
  if (a == 169 && b == 254) return true;
  return false;
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
