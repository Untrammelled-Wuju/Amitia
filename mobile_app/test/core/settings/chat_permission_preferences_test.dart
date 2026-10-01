import 'package:amitia_app/core/settings/chat_permission_preferences.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('missing permission mode defaults to request approval', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final preferences = ChatPermissionPreferencesNotifier();

    await preferences.init();

    expect(preferences.state, chatPermissionRequestApproval);
  });

  test('full access persists across notifier instances', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final preferences = ChatPermissionPreferencesNotifier();
    await preferences.init();

    await preferences.setMode(chatPermissionFullAccess);

    final restored = ChatPermissionPreferencesNotifier();
    await restored.init();
    expect(restored.state, chatPermissionFullAccess);
  });

  test('invalid permission mode falls back to request approval', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{
      chatPermissionModeStorageKey: 'invalid',
    });
    final preferences = ChatPermissionPreferencesNotifier();

    await preferences.init();

    expect(preferences.state, chatPermissionRequestApproval);
  });
}
