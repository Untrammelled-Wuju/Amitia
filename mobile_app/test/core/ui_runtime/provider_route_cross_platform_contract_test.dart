import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

const _root = '..';

Map<String, dynamic> _readJson(String path) {
  return jsonDecode(File('$_root/$path').readAsStringSync())
      as Map<String, dynamic>;
}

Map<String, dynamic> _contribution(Map<String, dynamic> manifest, String id) {
  final modules = (manifest['modules'] as List).cast<Map<String, dynamic>>();
  return modules
      .expand(
        (module) =>
            (module['contributions'] as List).cast<Map<String, dynamic>>(),
      )
      .firstWhere((item) => (item['spec'] as Map?)?['providerId'] == id);
}

void main() {
  final contract = _readJson(
    'contracts/ui-runtime/v1/provider-route-consistency.json',
  );
  final protectedRoutes = (contract['protectedRoutes'] as List).cast<String>();
  final providerRoutes = (contract['providerRoutes'] as List)
      .cast<Map<String, dynamic>>();

  test('protects the same core recovery routes', () {
    expect(contract['schemaVersion'], 1);
    for (final path in protectedRoutes) {
      expect(isProtectedProviderRoutePath(path), isTrue, reason: path);
    }
  });

  test('keeps the same plugin route batch provider-owned', () {
    for (final item in providerRoutes) {
      expect(
        isProtectedProviderRoutePath(item['path'] as String),
        isFalse,
        reason: item['path'] as String,
      );
    }
  });

  test('publishes the same manifest route and platform entries', () {
    for (final item in providerRoutes) {
      final manifest = _readJson(item['manifest'] as String);
      expect((manifest['plugin'] as Map)['id'], item['extensionId']);
      final contribution = _contribution(
        manifest,
        item['registryProviderId'] as String,
      );
      final spec = contribution['spec'] as Map<String, dynamic>;
      final entries = (spec['entries'] as Map).keys.cast<String>().toList();
      final expectedEntries = (item['entries'] as List).cast<String>().toList();
      expect(
        entries.toSet(),
        expectedEntries.toSet(),
        reason: item['path'] as String,
      );
      final routes = ((spec['metadata'] as Map)['routes'] as List)
          .cast<Map<String, dynamic>>();
      final route = routes.firstWhere(
        (candidate) =>
            candidate['id'] == item['routeId'] &&
            candidate['path'] == item['path'],
        orElse: () => <String, dynamic>{},
      );
      expect(route['path'], item['path']);
      expect(route['providerId'], item['pageProviderId']);
      expect(route['capability'], 'page.provider');
    }
  });
}
