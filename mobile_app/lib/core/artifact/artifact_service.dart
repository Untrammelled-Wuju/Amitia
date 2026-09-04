import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:image_picker/image_picker.dart';

import 'artifact_model.dart';

enum UploadState {
  queued,
  uploading,
  uploaded,
  failed,
}

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

  HttpArtifactService({
    required Dio dio,
    required String baseUrl,
  })  : _dio = dio,
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
          onProgress(UploadTask(
            id: '',
            fileName: actualFileName,
            kind: kind,
            totalBytes: total,
            loadedBytes: sent,
            state: UploadState.uploading,
          ));
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
          onProgress(UploadTask(
            id: '',
            fileName: fileName,
            kind: kind,
            totalBytes: total,
            loadedBytes: sent,
            state: UploadState.uploading,
          ));
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
    final response = await _dio.delete('$_baseUrl/api/artifacts/v1/$artifactId');
    if (response.statusCode != 200) {
      throw ArtifactServiceException('delete_failed: ${response.statusCode}');
    }
  }

  @override
  String contentUrl(String artifactId) {
    return '$_baseUrl/api/artifacts/v1/$artifactId/content';
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
}

class ArtifactServiceException implements Exception {
  final String message;
  ArtifactServiceException(this.message);

  @override
  String toString() => 'ArtifactServiceException: $message';
}
