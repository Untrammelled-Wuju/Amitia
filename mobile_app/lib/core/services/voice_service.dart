import '../backend_transport/backend_service_api.dart';
import '../models/voice.dart';

class TTSService {
  final BackendServiceApi _api;

  TTSService(this._api);

  Future<List<VoiceConfigDto>> listConfigs() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/configs');
    if (resp == null) return [];
    return resp.map((e) => VoiceConfigDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<VoiceConfigDto?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/configs', data: data);
    if (resp == null) return null;
    return VoiceConfigDto.fromJson(resp);
  }

  Future<VoiceConfigDto?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/tts/configs/$id', data: data);
    if (resp == null) return null;
    return VoiceConfigDto.fromJson(resp);
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/tts/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/tts/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/configs/$id/test');
    return resp;
  }

  Future<Map<String, dynamic>?> testConnection(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/test-connection', data: data);
    return resp;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> voices() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/voices');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> emotions() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/emotions');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> synthesize(String text, {String? voiceId}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/tts/synthesize',
      data: {
        'text': text,
        if (voiceId != null) 'voiceId': int.tryParse(voiceId) ?? 0,
      },
    );
    return resp;
  }

  Future<Map<String, dynamic>?> synthesizeForCharacter(String characterId, String text) async {
    return _api.post<Map<String, dynamic>>(
      '/api/tts/synthesize',
      data: {'characterId': characterId, 'text': text},
    );
  }
}

class ASRService {
  final BackendServiceApi _api;

  ASRService(this._api);

  Future<List<Map<String, dynamic>>> configs() async {
    final resp = await _api.get<List<dynamic>>('/api/asr/configs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/configs', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/asr/configs/$id', data: data);
    return resp;
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/asr/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/asr/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/configs/$id/test');
    return resp;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/asr/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> uploadAudio(String filePath) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/upload', data: {'filePath': filePath});
    return resp;
  }

  Future<Map<String, dynamic>?> submitUrl(String url) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/submit', data: {'url': url});
    return resp;
  }

  Future<Map<String, dynamic>?> queryResult(String taskId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/asr/query', queryParameters: {'taskId': taskId});
    return resp;
  }
}
