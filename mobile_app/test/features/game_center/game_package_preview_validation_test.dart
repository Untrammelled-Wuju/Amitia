import 'package:amitia_app/features/game_center/data/game_center_package_lifecycle.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('incompatible preview preserves original backend issues', () {
    final preview = <String, dynamic>{
      'managementTarget': 'game_center',
      'contributionKinds': ['gamex'],
      'installable': false,
      'compatible': true,
      'issues': [
        {
          'code': 'gamex_network_platform_unsupported',
          'message': 'restricted network sandbox is unavailable',
        },
      ],
    };

    expect(
      () => validateGamePackagePreview(preview, expectedExtensionId: ''),
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          allOf(
            contains('gamex_network_platform_unsupported'),
            contains('restricted network sandbox is unavailable'),
          ),
        ),
      ),
    );
  });
}
