import 'package:amitia_app/core/services/space_profile_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('avatar cache persists and clears the user avatar', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final cache = SpaceAvatarCache();

    await cache.write('data:image/png;base64,avatar');
    expect(await cache.read(), 'data:image/png;base64,avatar');

    await cache.clear();
    expect(await cache.read(), isEmpty);
  });
}
