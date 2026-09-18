import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/backend/backend_topology_resolver.dart';
import 'package:amitia_app/core/runtime/backend/backend_topology.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';

void main() {
  group('normalizeRemoteCoreUri', () {
    test('uses http for private IPv4 addresses without an explicit scheme', () {
      expect(
        normalizeRemoteCoreUri('192.168.1.10:18899').toString(),
        'http://192.168.1.10:18899',
      );
    });

    test('uses https for public hosts without an explicit scheme', () {
      expect(
        normalizeRemoteCoreUri('cloud.example.com').toString(),
        'https://cloud.example.com',
      );
    });

    test('does not treat public hostnames beginning with fc or fd as IPv6', () {
      expect(
        normalizeRemoteCoreUri('fcloud.example.com').toString(),
        'https://fcloud.example.com',
      );
      expect(
        normalizeRemoteCoreUri('fdomain.example.com').toString(),
        'https://fdomain.example.com',
      );
    });

    test('preserves an explicit scheme and root-only endpoint', () {
      expect(
        normalizeRemoteCoreUri('http://10.0.0.8:18899/').toString(),
        'http://10.0.0.8:18899',
      );
    });

    test('canonicalizes default ports like the desktop URL contract', () {
      expect(
        normalizeRemoteCoreUri('https://cloud.example.com:443').toString(),
        'https://cloud.example.com',
      );
      expect(
        normalizeRemoteCoreUri('http://192.168.1.10:80').toString(),
        'http://192.168.1.10',
      );
    });

    test('rejects subpaths instead of silently dropping them', () {
      expect(
        () => normalizeRemoteCoreUri('https://cloud.example.com/core'),
        throwsA(isA<DeploymentConfigValidationError>()),
      );
    });

    test('rejects query, fragment and embedded credentials', () {
      for (final value in <String>[
        'https://cloud.example.com?tenant=a',
        'https://cloud.example.com#fragment',
        'https://user:pass@cloud.example.com',
      ]) {
        expect(
          () => normalizeRemoteCoreUri(value),
          throwsA(isA<DeploymentConfigValidationError>()),
          reason: value,
        );
      }
    });
  });

  group('DefaultBackendTopologyResolver', () {
    const resolver = DefaultBackendTopologyResolver();

    test('cloud mode keeps business core remote and local runtime on device', () {
      final topology = resolver.resolve(
        const MobileDeploymentConfig(
          mode: MobileDeploymentMode.cloud,
          remoteCoreUri: 'cloud.example.com',
        ),
      );

      expect(topology.businessCore.httpBaseUri.toString(), 'https://cloud.example.com');
      expect(topology.businessCore.isRemote, isTrue);
      expect(topology.localRuntime!.httpBaseUri.toString(), 'http://127.0.0.1:18899');
      expect(topology.localRuntime!.isRemote, isFalse);
      expect(topology.embeddedRuntimeProfile, EmbeddedRuntimeProfile.deviceAgent);
      expect(topology.requiresEmbeddedRuntime, isTrue);
    });

    test('local mode uses the embedded runtime for both execution planes', () {
      final topology = resolver.resolve(MobileDeploymentConfig.local);

      expect(topology.businessCore.httpBaseUri.toString(), 'http://127.0.0.1:18899');
      expect(topology.localRuntime!.httpBaseUri, topology.businessCore.httpBaseUri);
      expect(topology.embeddedRuntimeProfile, EmbeddedRuntimeProfile.local);
    });
  });
}
