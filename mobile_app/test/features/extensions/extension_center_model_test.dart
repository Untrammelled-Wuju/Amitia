import 'package:amitia_app/core/services/extension_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('extension center preserves canonical card and group fields', () {
    final view = ExtensionCenterView.fromJson({
      'schemaVersion': 1,
      'targetMode': 'current',
      'generatedAt': '2026-11-01T09:00:00Z',
      'partial': true,
      'errors': [
        {
          'code': 'UPDATE_CHECK_FAILED',
          'message': 'registry unavailable: original',
          'source': 'package_distribution',
        },
      ],
      'installed': [
        _card('com.example/installed', 'installed_enabled'),
      ],
      'discover': <dynamic>[],
      'localImports': [
        _card('com.example/local', 'installed_disabled'),
      ],
      'dev': [
        _card('com.example/dev', 'installed_enabled'),
      ],
      'updates': [
        _card('com.example/update', 'installed_enabled'),
      ],
      'needsAction': [
        _card('com.example/permission', 'needs_permission'),
      ],
    });

    expect(view.installed.single.trust, 'official');
    expect(view.installed.single.permissionRisk, 'high');
    expect(view.localImports.single.extensionId, 'com.example/local');
    expect(view.dev.single.devMode, isTrue);
    expect(view.updates.single.updateAvailable, isTrue);
    expect(view.updates.single.latestVersion, '2.0.0');
    expect(view.needsAction.single.missingPermissions, ['camera']);
    expect(view.partial, isTrue);
    expect(view.errors.single['message'], 'registry unavailable: original');
  });
}

Map<String, dynamic> _card(String id, String status) {
  return {
    'extensionId': id,
    'installationId': 'installation-$id',
    'generation': 3,
    'displayName': id,
    'description': 'description',
    'version': '1.0.0',
    'latestVersion': '2.0.0',
    'status': status,
    'enabled': status == 'installed_enabled',
    'updateAvailable': id.endsWith('/update'),
    'trust': 'official',
    'permissionRisk': 'high',
    'source': 'installed',
    'systemManaged': false,
    'devMode': id.endsWith('/dev'),
    'permissions': ['camera'],
    'missingPermissions': id.endsWith('/permission') ? ['camera'] : <String>[],
    'actions': [
      {'actionId': 'detail', 'available': true},
    ],
    'diagnostics': <dynamic>[],
    'contributionTags': <dynamic>[],
    'platforms': ['android'],
  };
}
