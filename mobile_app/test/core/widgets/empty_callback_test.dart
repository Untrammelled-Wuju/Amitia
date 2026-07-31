import 'dart:io';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Empty callback verification', () {
    test('no empty onPressed callbacks in lib source files', () async {
      final libDir = Directory('lib');
      final emptyCallbackPattern = RegExp(r'onPressed:\s*\(\s*\)\s*\{\s*\}');
      final violations = <String>[];

      await for (final entity in libDir.list(recursive: true)) {
        if (entity is File && entity.path.endsWith('.dart')) {
          final content = await entity.readAsString();
          if (emptyCallbackPattern.hasMatch(content)) {
            violations.add(entity.path);
          }
        }
      }

      expect(violations, isEmpty,
          reason: 'Files with empty onPressed callbacks: ${violations.join(', ')}');
    });

    test('no empty onTap callbacks in lib source files', () async {
      final libDir = Directory('lib');
      final emptyCallbackPattern = RegExp(r'onTap:\s*\(\s*\)\s*\{\s*\}');
      final violations = <String>[];

      await for (final entity in libDir.list(recursive: true)) {
        if (entity is File && entity.path.endsWith('.dart')) {
          final content = await entity.readAsString();
          if (emptyCallbackPattern.hasMatch(content)) {
            violations.add(entity.path);
          }
        }
      }

      expect(violations, isEmpty,
          reason: 'Files with empty onTap callbacks: ${violations.join(', ')}');
    });

    test('amitia_dialogs.dart exports all expected functions', () {
      final dialogFile = File('lib/core/widgets/amitia_dialogs.dart');
      expect(dialogFile.existsSync(), isTrue);

      final content = dialogFile.readAsStringSync();
      expect(content, contains('showAmitiaConfirmDialog'));
      expect(content, contains('showAmitiaInfoDialog'));
      expect(content, contains('showAmitiaActionSheet'));
      expect(content, contains('amitiaSnackBar'));
      expect(content, contains('amitiaComingSoon'));
      expect(content, contains('AmitiaActionSheetItem'));
    });

    test('amitia_misc.dart exports amitia_dialogs.dart', () {
      final miscFile = File('lib/core/widgets/amitia_misc.dart');
      expect(miscFile.existsSync(), isTrue);

      final content = miscFile.readAsStringSync();
      expect(content, contains("export 'amitia_dialogs.dart'"));
    });

    test('app_routes.dart has no legacy about/toolbox constants', () {
      final routesFile = File('lib/app/app_routes.dart');
      expect(routesFile.existsSync(), isTrue);

      final content = routesFile.readAsStringSync();
      expect(content, isNot(contains("static const about =")));
      expect(content, isNot(contains("static const toolbox =")));
    });

    test('router.dart has settings/toolbox route', () {
      final routerFile = File('lib/app/router.dart');
      expect(routerFile.existsSync(), isTrue);

      final content = routerFile.readAsStringSync();
      expect(content, contains("/settings/toolbox"));
    });

    test('router.dart does not have standalone about/toolbox routes', () {
      final routerFile = File('lib/app/router.dart');
      expect(routerFile.existsSync(), isTrue);

      final content = routerFile.readAsStringSync();
      expect(content, isNot(contains("path: '/about'")));
      expect(content, isNot(contains("path: '/toolbox'")));
    });
  });
}
