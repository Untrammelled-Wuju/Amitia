import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_dto.dart';

void main() {
  group('DesktopPetPluginSummary.fromJson', () {
    test('parses full payload', () {
      final json = {
        'extensionId': 'ext-001',
        'pluginId': 'plg-123',
        'name': 'Test Plugin',
        'description': 'A test plugin',
        'version': '1.2.0',
        'enabled': true,
        'installState': 'installed',
        'publisher': 'test-pub',
        'permissionSummary': {
          'declared': ['camera', 'mic'],
          'granted': ['camera'],
        },
      };
      final s = DesktopPetPluginSummary.fromJson(json);
      expect(s.extensionId, 'ext-001');
      expect(s.pluginId, 'plg-123');
      expect(s.name, 'Test Plugin');
      expect(s.description, 'A test plugin');
      expect(s.version, '1.2.0');
      expect(s.enabled, true);
      expect(s.installState, 'installed');
      expect(s.publisher, 'test-pub');
      expect(s.permissionSummary, isNotNull);
      expect(s.permissionSummary!.declared, ['camera', 'mic']);
      expect(s.permissionSummary!.granted, ['camera']);
    });

    test('handles missing optional fields', () {
      final json = <String, dynamic>{
        'extensionId': 'ext-002',
        'pluginId': 'plg-999',
      };
      final s = DesktopPetPluginSummary.fromJson(json);
      expect(s.name, '');
      expect(s.description, '');
      expect(s.version, '');
      expect(s.enabled, false);
      expect(s.installState, '');
      expect(s.publisher, isNull);
      expect(s.permissionSummary, isNull);
    });

    test('handles null permissionSummary', () {
      final json = {
        'extensionId': 'ext-003',
        'pluginId': 'plg-333',
        'permissionSummary': null,
      };
      final s = DesktopPetPluginSummary.fromJson(json);
      expect(s.permissionSummary, isNull);
    });
  });

  group('DesktopPetPluginDetail.fromJson', () {
    test('parses full payload', () {
      final json = {
        'extensionId': 'ext-d1',
        'pluginId': 'plg-d1',
        'name': 'Detailed Plugin',
        'description': 'A detailed plugin',
        'version': '2.0.0',
        'enabled': false,
        'installState': 'installing',
        'publisher': 'pub-1',
        'requiredPermissions': ['network', 'storage'],
        'packageVersion': '2.0.0',
        'installedAt': '2026-01-01T00:00:00Z',
        'updatedAt': '2026-01-15T10:30:00Z',
        'source': 'marketplace',
        'permissionSummary': {
          'declared': ['network', 'storage'],
          'granted': ['network'],
        },
      };
      final d = DesktopPetPluginDetail.fromJson(json);
      expect(d.extensionId, 'ext-d1');
      expect(d.pluginId, 'plg-d1');
      expect(d.enabled, false);
      expect(d.installState, 'installing');
      expect(d.requiredPermissions, ['network', 'storage']);
      expect(d.packageVersion, '2.0.0');
      expect(d.installedAt, '2026-01-01T00:00:00Z');
      expect(d.updatedAt, '2026-01-15T10:30:00Z');
      expect(d.source, 'marketplace');
      expect(d.permissionSummary!.hasPermissions, true);
    });
  });

  group('DesktopPetPluginList.fromJson', () {
    test('parses list response', () {
      final json = {
        'plugins': [
          {'extensionId': 'e1', 'pluginId': 'p1', 'name': 'P1'},
          {'extensionId': 'e2', 'pluginId': 'p2', 'name': 'P2'},
        ],
        'total': 12,
        'page': 1,
        'pageSize': 10,
      };
      final l = DesktopPetPluginList.fromJson(json);
      expect(l.plugins.length, 2);
      expect(l.total, 12);
      expect(l.page, 1);
      expect(l.pageSize, 10);
      expect(l.isEmpty, false);
    });

    test('handles empty list', () {
      final json = {'plugins': <Map<String, dynamic>>[]};
      final l = DesktopPetPluginList.fromJson(json);
      expect(l.plugins, isEmpty);
      expect(l.total, 0);
      expect(l.isEmpty, true);
    });

    test('handles null plugins', () {
      final json = <String, dynamic>{};
      final l = DesktopPetPluginList.fromJson(json);
      expect(l.plugins, isEmpty);
    });
  });

  group('DesktopPetPluginInstallResult.fromJson', () {
    test('parses', () {
      final json = {
        'extensionId': 'ext-i1',
        'version': '1.0.0',
        'installState': 'installed',
      };
      final r = DesktopPetPluginInstallResult.fromJson(json);
      expect(r.extensionId, 'ext-i1');
      expect(r.version, '1.0.0');
      expect(r.installState, 'installed');
    });
  });

  group('DesktopPetPluginMutationResult.fromJson', () {
    test('parses success', () {
      final json = {'extensionId': 'ext-m1', 'success': true};
      final r = DesktopPetPluginMutationResult.fromJson(json);
      expect(r.extensionId, 'ext-m1');
      expect(r.success, true);
    });

    test('parses missing success', () {
      final json = {'extensionId': 'ext-m2'};
      final r = DesktopPetPluginMutationResult.fromJson(json);
      expect(r.success, false);
    });
  });

  group('DesktopPetPluginPermissionSummary', () {
    test('hasPermissions true when declared not empty', () {
      const p = DesktopPetPluginPermissionSummary(
        declared: ['camera'],
        granted: [],
      );
      expect(p.hasPermissions, true);
    });

    test('hasPermissions true when granted not empty', () {
      const p = DesktopPetPluginPermissionSummary(
        declared: [],
        granted: ['mic'],
      );
      expect(p.hasPermissions, true);
    });

    test('hasPermissions false when both empty', () {
      const p = DesktopPetPluginPermissionSummary(
        declared: [],
        granted: [],
      );
      expect(p.hasPermissions, false);
    });
  });
}
