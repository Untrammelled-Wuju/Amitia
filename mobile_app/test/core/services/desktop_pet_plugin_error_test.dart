import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/error_utils.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_api.dart';
import 'package:amitia_app/features/desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart';
import 'package:flutter_test/flutter_test.dart';

import 'owned_task_submission_test.dart' show TaskTransport;

void main() {
  test('install failure survives post-mutation refresh', () async {
    final transport = TaskTransport([
      Exception('petx validation failed: QA_ORIGINAL'),
      {'plugins': <Object>[], 'total': 0, 'page': 1, 'pageSize': 20},
    ]);
    final controller = DesktopPetPluginController(
      api: DesktopPetPluginApi(BackendServiceApi(transport, 1)),
    );
    addTearDown(controller.dispose);

    expect(await controller.install('/tmp/qa.petx'), isFalse);

    expect(controller.state.error, contains('QA_ORIGINAL'));
    expect(controller.state.installing, isFalse);
  });

  test('generic errors keep their original text', () {
    expect(
      safeErrorMessage(Exception('QA_ORIGINAL_CAUSE')),
      contains('QA_ORIGINAL_CAUSE'),
    );
  });
}
