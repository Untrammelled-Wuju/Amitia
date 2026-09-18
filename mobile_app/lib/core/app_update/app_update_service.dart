import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';

import '../ui_runtime/ui_device_identity.dart';
import 'app_update_models.dart';

class AppUpdateService {
  static const MethodChannel _channel = MethodChannel(
    'com.amitia.app_update/control',
  );
  static const String updateBaseUrl = String.fromEnvironment(
    'AMITIA_ANDROID_UPDATE_BASE_URL',
    defaultValue: 'https://amitia.untrammelled.top/amitia/android',
  );
  static const String updateChannel = String.fromEnvironment(
    'AMITIA_ANDROID_UPDATE_CHANNEL',
    defaultValue: 'stable',
  );

  AppUpdateService({Dio? dio, String? baseUrl})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 15),
              receiveTimeout: const Duration(seconds: 30),
              headers: const <String, Object>{'Accept': 'application/json'},
            ),
          ),
      _baseUrl = baseUrl ?? updateBaseUrl;

  final Dio _dio;
  final String _baseUrl;

  Future<InstalledAppInfo> getInstalledInfo() async {
    final value = await _channel.invokeMethod<Map<Object?, Object?>>(
      'getInstalledInfo',
    );
    return InstalledAppInfo.fromMap(value ?? const <Object?, Object?>{});
  }

  Future<AppUpdateCheckResult> check({String? channel}) async {
    final installed = await getInstalledInfo();
    final normalizedChannel = _normalizeChannel(channel ?? updateChannel);
    final manifestUrl = '$_baseUrl/$normalizedChannel.json';
    late final Response<List<int>> manifestResponse;
    try {
      manifestResponse = await _dio.get<List<int>>(
        manifestUrl,
        options: Options(
          responseType: ResponseType.bytes,
          headers: const <String, Object>{'Cache-Control': 'no-cache'},
        ),
      );
    } on DioException catch (error) {
      if (_isNotFound(error)) {
        return AppUpdateCheckResult(
          installed: installed,
          available: null,
          reason: 'manifest_unavailable',
        );
      }
      rethrow;
    }
    final manifestBytes = manifestResponse.data ?? const <int>[];
    if (manifestBytes.isEmpty) {
      throw StateError('更新清单为空');
    }
    final manifestText = utf8.decode(manifestBytes);
    late final Response<String> signatureResponse;
    try {
      signatureResponse = await _dio.get<String>(
        '$manifestUrl.sig',
        options: Options(
          responseType: ResponseType.plain,
          headers: const <String, Object>{'Cache-Control': 'no-cache'},
        ),
      );
    } on DioException catch (error) {
      if (_isNotFound(error)) {
        return AppUpdateCheckResult(
          installed: installed,
          available: null,
          reason: 'manifest_unavailable',
        );
      }
      rethrow;
    }
    final signature = (signatureResponse.data ?? '').trim();
    final signatureValid = await _channel.invokeMethod<bool>(
      'verifyManifest',
      <String, Object>{'manifest': manifestText, 'signatureBase64': signature},
    );
    if (signatureValid != true) {
      throw StateError('更新清单签名无效');
    }

    final decoded = jsonDecode(manifestText);
    if (decoded is! Map) {
      throw StateError('更新清单格式无效');
    }
    final manifest = AppUpdateManifest.fromMap(
      Map<String, dynamic>.from(decoded),
    );
    _validateManifest(manifest, installed, normalizedChannel);

    if (manifest.versionCode <= installed.versionCode) {
      return AppUpdateCheckResult(
        installed: installed,
        available: null,
        reason: 'already_latest',
      );
    }
    if (!await _isWithinRollout(manifest)) {
      return AppUpdateCheckResult(
        installed: installed,
        available: null,
        reason: 'rollout_excluded',
      );
    }
    return AppUpdateCheckResult(
      installed: installed,
      available: manifest,
      reason: null,
    );
  }

  Future<int> download(AppUpdateManifest manifest) async {
    final fileName =
        'amitia-${manifest.versionName}-${manifest.versionCode}-${manifest.apk.abi}.apk';
    final downloadId = await _channel
        .invokeMethod<int>('download', <String, Object>{
          'url': manifest.apk.url,
          'fileName': fileName,
          'expectedSize': manifest.apk.size,
          'expectedSha256': manifest.apk.sha256,
        });
    if (downloadId == null) {
      throw StateError('下载任务创建失败');
    }
    return downloadId;
  }

  Future<AppUpdateDownloadStatus> getDownloadStatus(int downloadId) async {
    final value = await _channel.invokeMethod<Map<Object?, Object?>>(
      'getDownloadStatus',
      <String, Object>{'downloadId': downloadId},
    );
    return AppUpdateDownloadStatus.fromMap(value ?? const <Object?, Object?>{});
  }

  Future<void> install(int downloadId, AppUpdateManifest manifest) async {
    await _channel.invokeMethod<void>('install', <String, Object>{
      'downloadId': downloadId,
      'expectedSha256': manifest.apk.sha256,
      'expectedSize': manifest.apk.size,
    });
  }

  Future<void> openInstallPermissionSettings() async {
    await _channel.invokeMethod<void>('openInstallPermissionSettings');
  }

  Future<AppUpdateInstallResult?> consumeInstallResult() async {
    final value = await _channel.invokeMethod<Map<Object?, Object?>>(
      'getPendingInstallResult',
    );
    if (value == null || value.isEmpty) return null;
    return AppUpdateInstallResult.fromMap(value);
  }

  Future<bool> _isWithinRollout(AppUpdateManifest manifest) async {
    if (manifest.rolloutPercentage >= 100) return true;
    if (manifest.rolloutPercentage <= 0) return false;
    final deviceId = await UIDeviceIdentity().getOrCreate();
    final digest = sha256.convert(
      utf8.encode('$deviceId:${manifest.versionCode}'),
    );
    final bucket =
        int.parse(digest.toString().substring(0, 8), radix: 16) % 100;
    return bucket < manifest.rolloutPercentage;
  }

  void _validateManifest(
    AppUpdateManifest manifest,
    InstalledAppInfo installed,
    String channel,
  ) {
    if (manifest.schemaVersion != 1) {
      throw StateError('不支持的更新清单版本');
    }
    if (manifest.product != 'android') {
      throw StateError('更新清单产品类型不匹配');
    }
    if (manifest.packageName != installed.packageName) {
      throw StateError('更新清单包名不匹配');
    }
    if (manifest.channel != channel) {
      throw StateError('更新清单通道不匹配');
    }
    if (manifest.versionCode <= 0 || manifest.versionName.isEmpty) {
      throw StateError('更新清单版本信息无效');
    }
    if (manifest.apk.url.isEmpty ||
        manifest.apk.size <= 0 ||
        manifest.apk.sha256.length != 64) {
      throw StateError('更新清单 APK 信息无效');
    }
    if (manifest.apk.abi != 'arm64-v8a') {
      throw StateError('当前安装包不支持更新清单 ABI');
    }
  }

  String _normalizeChannel(String value) {
    final normalized = value.trim().toLowerCase();
    if (normalized == 'stable' ||
        normalized == 'beta' ||
        normalized == 'alpha') {
      return normalized;
    }
    return 'stable';
  }

  bool _isNotFound(DioException error) {
    return error.response?.statusCode == 404;
  }

  void dispose() {
    _dio.close(force: true);
  }
}
