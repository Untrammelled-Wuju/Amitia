import 'dart:io';
import 'package:amitia_app/core/settings/background_preferences.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  late Directory root;
  setUp(() async {
    SharedPreferences.setMockInitialValues({});
    root = await Directory.systemTemp.createTemp('amitia-background-test-');
  });
  tearDown(() async {
    await root.delete(recursive: true);
  });
  BackgroundPreferencesNotifier notifier() =>
      BackgroundPreferencesNotifier(directory: () async => root);

  test(
    'copies selected media into private storage, restores preferences, and removes replaced files',
    () async {
      final source = File('${root.path}/photo.png');
      await source.writeAsBytes([1, 2, 3]);
      final prefs = notifier();
      await prefs.init();
      expect(prefs.state.active, false);
      await prefs.importFile(source.path, 'photo.png');
      final owned = prefs.state.path;
      expect(owned, isNot(source.path));
      await source.delete();
      expect(await File(owned).readAsBytes(), [1, 2, 3]);
      await Future.wait([
        prefs.update(opacity: 0.6),
        prefs.update(blurEnabled: true, blurRadius: 16),
      ]);
      await prefs.update(enabled: false);
      final restored = notifier();
      await restored.init();
      expect(restored.state.opacity, 0.6);
      expect(restored.state.blurRadius, 16);
      expect(restored.state.path, owned);
      expect(restored.state.enabled, false);
      await restored.update(enabled: true);
      final second = File('${root.path}/second.mp4');
      await second.writeAsBytes([1]);
      await restored.importFile(second.path, 'second.mp4');
      expect(restored.state.kind, BackgroundMediaKind.video);
      expect(await File(owned).exists(), false);
      final selected = restored.state.path;
      await restored.clear();
      expect(await File(selected).exists(), false);
      expect(await second.exists(), true);
      expect(restored.state.active, false);
      prefs.dispose();
      restored.dispose();
    },
  );
  test(
    'rejects invalid formats, empty and oversized files, and unsafe persisted paths',
    () {
      expect(() => validateBackgroundFile('a.exe', 1), throwsFormatException);
      expect(() => validateBackgroundFile('a.jpg', 0), throwsFormatException);
      expect(
        () => validateBackgroundFile('a.mp4', 151 * 1024 * 1024),
        throwsFormatException,
      );
      expect(
        BackgroundPreferences.decode(
          '{"enabled":true,"file":"../outside.png","opacity":5,"blurRadius":90}',
          root,
        ).active,
        false,
      );
      expect(BackgroundPreferences.decode('invalid', root).opacity, 0.35);
    },
  );
  test(
    'missing media restores the default background without breaking startup',
    () async {
      SharedPreferences.setMockInitialValues({
        'appearance.background.v1':
            '{"enabled":true,"file":"background-123.png","name":"lost.png"}',
      });
      final prefs = notifier();
      await prefs.init();
      expect(prefs.state.active, false);
      expect(prefs.state.path, '');
      prefs.dispose();
    },
  );
}
