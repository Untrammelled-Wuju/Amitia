import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/channel_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _ImageGenerationApi extends Fake implements BackendServiceApi {
  String? path;
  Object? body;

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    this.path = path;
    body = data;
    return <String, dynamic>{
          'provider': 'stability',
          'images': <dynamic>[
            <String, dynamic>{'resourceUri': 'amitia://artifacts/generated'},
          ],
        }
        as T;
  }
}

void main() {
  test('图像生成使用统一 Image Intelligence 接口并保留参考图', () async {
    final api = _ImageGenerationApi();
    final result = await ImageGenService(api).generate(
      prompt: 'draw a tree',
      size: '1280x720',
      referenceResourceUris: const ['amitia://artifacts/source'],
    );
    expect(api.path, '/api/image-intelligence/generate');
    expect(api.body, containsPair('prompt', 'draw a tree'));
    expect(api.body, containsPair('width', 1280));
    expect(api.body, containsPair('height', 720));
    expect(
      api.body,
      containsPair('referenceResourceUris', const [
        'amitia://artifacts/source',
      ]),
    );
    expect(result['provider'], 'stability');
  });
}
