import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:image_picker/image_picker.dart';

import 'artifact_model.dart';

enum UploadState { queued, uploading, uploaded, failed }

class UploadTask {
  final String id;
  final String fileName;
  final ArtifactKind kind;
  final int totalBytes;
  UploadState state;
  int loadedBytes;
  String? error;
  ArtifactMetadata? artifact;

  UploadTask({
    required this.id,
    required this.fileName,
    required this.kind,
    required this.totalBytes,
    this.state = UploadState.queued,
    this.loadedBytes = 0,
    this.error,
    this.artifact,
  });

  double get progress => totalBytes > 0 ? loadedBytes / totalBytes : 0.0;
}

typedef UploadProgressCallback = void Function(UploadTask task);

abstract class ArtifactService {
  Future<ArtifactMetadata> uploadFile({
    required String filePath,
    required ArtifactKind kind,
    String? fileName,
    String? mimeType,
    String source = 'user_upload',
    UploadProgressCallback? onProgress,
  });

  Future<ArtifactMetadata> uploadBytes({
    required Uint8List bytes,
    required ArtifactKind kind,
    required String fileName,
    required String mimeType,
    String source = 'ui_provider',
    UploadProgressCallback? onProgress,
  });

  Future<ArtifactMetadata> getMetadata(String artifactId);

  Future<void> deleteArtifact(String artifactId);

  String contentUrl(String artifactId);

  Future<Uri> resolveMediaUri(String rawUrl, {bool embed = false});

  Future<Uri> resolveDownloadUri(String rawUrl);

  Future<String> readText(String rawUrl);

  Future<bool> saveToUserLocation({
    required String rawUrl,
    required String fileName,
    required String mimeType,
  });

  Future<ArtifactMetadata> pickAndUploadImage({
    ImageSource source = ImageSource.gallery,
    UploadProgressCallback? onProgress,
  });

  Future<ArtifactMetadata> pickAndUploadVideo({
    ImageSource source = ImageSource.gallery,
    UploadProgressCallback? onProgress,
  });

  Future<ArtifactMetadata> pickAndUploadFile({
    UploadProgressCallback? onProgress,
  });

  Future<ArtifactMetadata> pickAndUploadAudio({
    UploadProgressCallback? onProgress,
  });
}

class HttpArtifactService implements ArtifactService {
  final Dio _dio;
  final String _baseUrl;

  HttpArtifactService({required Dio dio, required String baseUrl})
    : _dio = dio,
      _baseUrl = baseUrl.replaceAll(RegExp(r'/$'), '');

  @override
  Future<ArtifactMetadata> uploadFile({
    required String filePath,
    required ArtifactKind kind,
    String? fileName,
    String? mimeType,
    String source = 'user_upload',
    UploadProgressCallback? onProgress,
  }) async {
    final file = File(filePath);
    final actualFileName = fileName ?? file.path.split('/').last;
    final actualMimeType = mimeType ?? _detectMimeType(actualFileName);

    final formData = FormData.fromMap({
      'kind': kind.value,
      'source': source,
      'mime_type': actualMimeType,
      'file': await MultipartFile.fromFile(
        filePath,
        filename: actualFileName,
        contentType: DioMediaType.parse(actualMimeType),
      ),
    });

    final response = await _dio.post(
      '$_baseUrl/api/artifacts/v1',
      data: formData,
      onSendProgress: (sent, total) {
        if (onProgress != null) {
          onProgress(
            UploadTask(
              id: '',
              fileName: actualFileName,
              kind: kind,
              totalBytes: total,
              loadedBytes: sent,
              state: UploadState.uploading,
            ),
          );
        }
      },
      options: Options(
        headers: {'Content-Type': 'multipart/form-data'},
        validateStatus: (status) => status != null && status < 500,
      ),
    );

    if (response.statusCode != 200) {
      throw ArtifactServiceException('upload_failed: ${response.statusCode}');
    }

    final data = response.data;
    if (data is Map<String, dynamic> && data['artifact'] != null) {
      return ArtifactMetadata.fromJson(data['artifact']);
    }
    throw ArtifactServiceException('invalid_artifact_response');
  }

  @override
  Future<ArtifactMetadata> uploadBytes({
    required Uint8List bytes,
    required ArtifactKind kind,
    required String fileName,
    required String mimeType,
    String source = 'ui_provider',
    UploadProgressCallback? onProgress,
  }) async {
    if (bytes.isEmpty) {
      throw ArtifactServiceException('empty_upload');
    }
    final formData = FormData.fromMap({
      'kind': kind.value,
      'source': source,
      'mime_type': mimeType,
      'file': MultipartFile.fromBytes(
        bytes,
        filename: fileName,
        contentType: DioMediaType.parse(mimeType),
      ),
    });

    final response = await _dio.post(
      '$_baseUrl/api/artifacts/v1',
      data: formData,
      onSendProgress: (sent, total) {
        if (onProgress != null) {
          onProgress(
            UploadTask(
              id: '',
              fileName: fileName,
              kind: kind,
              totalBytes: total,
              loadedBytes: sent,
              state: UploadState.uploading,
            ),
          );
        }
      },
      options: Options(
        headers: {'Content-Type': 'multipart/form-data'},
        validateStatus: (status) => status != null && status < 500,
      ),
    );

    if (response.statusCode != 200) {
      throw ArtifactServiceException('upload_failed: ${response.statusCode}');
    }
    final data = response.data;
    if (data is Map<String, dynamic> && data['artifact'] != null) {
      return ArtifactMetadata.fromJson(data['artifact']);
    }
    throw ArtifactServiceException('invalid_artifact_response');
  }

  @override
  Future<ArtifactMetadata> getMetadata(String artifactId) async {
    final response = await _dio.get('$_baseUrl/api/artifacts/v1/$artifactId');
    if (response.statusCode != 200) {
      throw ArtifactServiceException('metadata_failed: ${response.statusCode}');
    }
    final data = response.data;
    if (data is Map<String, dynamic> && data['artifact'] != null) {
      return ArtifactMetadata.fromJson(data['artifact']);
    }
    throw ArtifactServiceException('invalid_artifact_response');
  }

  @override
  Future<void> deleteArtifact(String artifactId) async {
    final response = await _dio.delete(
      '$_baseUrl/api/artifacts/v1/$artifactId',
    );
    if (response.statusCode != 200) {
      throw ArtifactServiceException('delete_failed: ${response.statusCode}');
    }
  }

  @override
  String contentUrl(String artifactId) {
    return '$_baseUrl/api/artifacts/v1/$artifactId/content';
  }

  @override
  Future<Uri> resolveMediaUri(String rawUrl, {bool embed = false}) async {
    final value = rawUrl.trim();
    if (value.isEmpty) throw ArtifactServiceException('empty_media_url');
    final parsed = Uri.tryParse(value);
    if (parsed == null) throw ArtifactServiceException('invalid_media_url');
    final contentMatch = RegExp(r'/api/artifacts/v1/([^/?#]+)/content(?:[?#]|$)').firstMatch(value);
    final artifactId = parseArtifactUri(value) ??
        (contentMatch == null ? null : Uri.decodeComponent(contentMatch.group(1)!));
    if (artifactId != null) {
      if (embed || Uri.parse(_baseUrl).path.contains('/internal/device-mesh/provider')) {
        final response = await _dio.get<List<int>>(
          '$_baseUrl/api/artifacts/v1/${Uri.encodeComponent(artifactId)}/content',
          options: Options(responseType: ResponseType.bytes),
        );
        return Uri.dataFromBytes(response.data ?? const <int>[],
            mimeType: (response.headers.value('content-type') ?? 'application/octet-stream').split(';').first);
      }
      final response = await _dio.get(
        '$_baseUrl/api/artifacts/v1/${Uri.encodeComponent(artifactId)}/media-ticket',
      );
      final data = response.data;
      final payload = data is Map && data['data'] is Map
          ? data['data'] as Map
          : data;
      final ticketPath = payload is Map
          ? (payload['url'] ?? '').toString().trim()
          : '';
      if (ticketPath.isEmpty) {
        throw ArtifactServiceException('invalid_media_ticket_response');
      }
      return Uri.parse(ticketPath).hasScheme
          ? Uri.parse(ticketPath)
          : Uri.parse('$_baseUrl/${ticketPath.replaceFirst(RegExp(r'^/+'), '')}');
    }
    if (parsed.hasScheme && const ['http', 'https', 'data', 'file'].contains(parsed.scheme.toLowerCase())) return parsed;
    if (value.startsWith('/')) {
      return Uri.parse('$_baseUrl$value');
    }
    throw ArtifactServiceException('unsupported_media_url');
  }

  @override
  Future<String> readText(String rawUrl) async {
    final bytes = await _downloadBytes(rawUrl);
    if (bytes.length > 5 * 1024 * 1024) throw ArtifactServiceException('文件过大，无法预览，请下载查看');
    return utf8.decode(bytes);
  }

  @override
  Future<Uri> resolveDownloadUri(String rawUrl) async {
    final resolved = await resolveMediaUri(rawUrl);
    if (resolved.scheme == 'data' || resolved.scheme == 'file') return resolved;
    return resolved.replace(
      queryParameters: <String, String>{
        ...resolved.queryParameters,
        'download': '1',
      },
    );
  }

  @override
  Future<bool> saveToUserLocation({
    required String rawUrl,
    required String fileName,
    required String mimeType,
  }) async {
    final bytes = await _downloadBytes(rawUrl);
    if (bytes.isEmpty) throw ArtifactServiceException('empty_download');
    final safeName = _safeFileName(fileName, mimeType);
    final output = await FilePicker.platform.saveFile(
      dialogTitle: '保存附件',
      fileName: safeName,
      type: _fileTypeForMime(mimeType, safeName),
      allowedExtensions: _allowedExtensions(safeName),
      bytes: bytes,
    );
    return output != null && output.trim().isNotEmpty;
  }

  @override
  Future<ArtifactMetadata> pickAndUploadImage({
    ImageSource source = ImageSource.gallery,
    UploadProgressCallback? onProgress,
  }) async {
    final picker = ImagePicker();
    final picked = await picker.pickImage(source: source);
    if (picked == null) {
      throw ArtifactServiceException('user_cancelled');
    }
    return uploadFile(
      filePath: picked.path,
      kind: ArtifactKind.image,
      fileName: picked.name,
      onProgress: onProgress,
    );
  }

  @override
  Future<ArtifactMetadata> pickAndUploadVideo({
    ImageSource source = ImageSource.gallery,
    UploadProgressCallback? onProgress,
  }) async {
    final picker = ImagePicker();
    final picked = await picker.pickVideo(source: source);
    if (picked == null) {
      throw ArtifactServiceException('user_cancelled');
    }
    return uploadFile(
      filePath: picked.path,
      kind: ArtifactKind.video,
      fileName: picked.name,
      onProgress: onProgress,
    );
  }

  @override
  Future<ArtifactMetadata> pickAndUploadFile({
    UploadProgressCallback? onProgress,
  }) async {
    final result = await FilePicker.platform.pickFiles(withReadStream: true);
    if (result == null || result.files.isEmpty) {
      throw ArtifactServiceException('user_cancelled');
    }
    final file = result.files.first;
    if (file.path == null || file.path!.isEmpty) {
      throw ArtifactServiceException('file_path_unavailable');
    }
    return uploadFile(
      filePath: file.path!,
      kind: ArtifactKind.fromMime(_detectMimeType(file.name)),
      fileName: file.name,
      onProgress: onProgress,
    );
  }

  @override
  Future<ArtifactMetadata> pickAndUploadAudio({
    UploadProgressCallback? onProgress,
  }) async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['mp3', 'wav', 'm4a', 'aac', 'ogg', 'webm'],
      withReadStream: true,
    );
    if (result == null || result.files.isEmpty) {
      throw ArtifactServiceException('user_cancelled');
    }
    final file = result.files.first;
    if (file.path == null || file.path!.isEmpty) {
      throw ArtifactServiceException('file_path_unavailable');
    }
    return uploadFile(
      filePath: file.path!,
      kind: ArtifactKind.audio,
      fileName: file.name,
      onProgress: onProgress,
    );
  }

  String _detectMimeType(String fileName) {
    final ext = fileName.split('.').last.toLowerCase();
    switch (ext) {
      case 'jpg':
      case 'jpeg':
        return 'image/jpeg';
      case 'png':
        return 'image/png';
      case 'gif':
        return 'image/gif';
      case 'webp':
        return 'image/webp';
      case 'mp4':
        return 'video/mp4';
      case 'mov':
        return 'video/quicktime';
      case 'webm':
        return 'video/webm';
      case 'mp3':
        return 'audio/mpeg';
      case 'wav':
        return 'audio/wav';
      case 'ogg':
        return 'audio/ogg';
      case 'm4a':
        return 'audio/mp4';
      default:
        return 'application/octet-stream';
    }
  }

  Future<Uint8List> _downloadBytes(String rawUrl) async {
    final value = rawUrl.trim();
    if (value.startsWith('data:')) {
      final comma = value.indexOf(',');
      if (comma <= 5) throw ArtifactServiceException('invalid_data_uri');
      final header = value.substring(5, comma);
      final payload = value.substring(comma + 1);
      if (header.toLowerCase().contains(';base64')) {
        return base64Decode(payload);
      }
      return Uint8List.fromList(utf8.encode(Uri.decodeComponent(payload)));
    }
    final resolved = await resolveDownloadUri(value);
    if (resolved.scheme == 'data') return resolved.data!.contentAsBytes();
    if (resolved.scheme == 'file') {
      return File(resolved.toFilePath()).readAsBytes();
    }
    final useBackendClient =
        !resolved.hasScheme || resolved.toString().startsWith('$_baseUrl/');
    final client = useBackendClient
        ? _dio
        : Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 15),
              receiveTimeout: const Duration(minutes: 10),
              followRedirects: true,
            ),
          );
    try {
      final response = await client.get<List<int>>(
        resolved.toString(),
        options: Options(responseType: ResponseType.bytes),
      );
      return Uint8List.fromList(response.data ?? const <int>[]);
    } finally {
      if (!identical(client, _dio)) client.close(force: true);
    }
  }

  String _safeFileName(String fileName, String mimeType) {
    var value = fileName.replaceAll(RegExp(r'[\\/:*?"<>|\r\n]+'), '-').trim();
    if (value.isEmpty) value = 'attachment';
    if (!value.contains('.')) {
      value = '$value${_extensionForMime(mimeType)}';
    }
    if (value.length > 180) {
      final dot = value.lastIndexOf('.');
      final extension = dot > 0 ? value.substring(dot) : '';
      value = '${value.substring(0, 180 - extension.length)}$extension';
    }
    return value;
  }

  String _extensionForMime(String mimeType) {
    switch (mimeType.toLowerCase()) {
      case 'image/jpeg':
        return '.jpg';
      case 'image/png':
        return '.png';
      case 'image/gif':
        return '.gif';
      case 'image/webp':
        return '.webp';
      case 'video/mp4':
        return '.mp4';
      case 'video/quicktime':
        return '.mov';
      case 'audio/mpeg':
        return '.mp3';
      case 'audio/wav':
        return '.wav';
      case 'audio/ogg':
        return '.ogg';
      case 'audio/mp4':
        return '.m4a';
      case 'application/pdf':
        return '.pdf';
      case 'application/zip':
        return '.zip';
      case 'text/plain':
        return '.txt';
      default:
        return '.bin';
    }
  }

  FileType _fileTypeForMime(String mimeType, String fileName) {
    if (mimeType.startsWith('image/')) return FileType.image;
    if (mimeType.startsWith('video/')) return FileType.video;
    if (mimeType.startsWith('audio/')) return FileType.audio;
    final extension = fileName.contains('.')
        ? fileName.split('.').last.toLowerCase()
        : '';
    if (extension.isEmpty || extension == 'bin') return FileType.any;
    return FileType.custom;
  }

  List<String>? _allowedExtensions(String fileName) {
    final dot = fileName.lastIndexOf('.');
    if (dot <= 0 || dot == fileName.length - 1) return null;
    return <String>[fileName.substring(dot + 1).toLowerCase()];
  }
}

class ArtifactServiceException implements Exception {
  final String message;
  ArtifactServiceException(this.message);

  @override
  String toString() => 'ArtifactServiceException: $message';
}
