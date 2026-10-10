import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/trusted_service_path.dart';

void main() {
  test('compound service identity survives every static operation', () {
    const id = 'com.amitia/channel-qq/service?+%中文';
    for (final operation in [
      null,
      'start',
      'stop',
      'health',
      'status',
      'invoke',
      'quarantine/release',
    ]) {
      final uri = Uri.parse(trustedServicePath(id, operation: operation));
      expect(
        uri.path,
        '/api/extensions/services${operation == null ? '' : '/$operation'}',
      );
      expect(uri.queryParametersAll['service_id'], [id]);
    }
  });
  test('reserved service names and invalid targets are handled explicitly', () {
    expect(
      Uri.parse(trustedServicePath('health')).path,
      '/api/extensions/services',
    );
    expect(() => trustedServicePath(' '), throwsArgumentError);
    expect(
      () => trustedServicePath('id', operation: '../start'),
      throwsArgumentError,
    );
  });
}
