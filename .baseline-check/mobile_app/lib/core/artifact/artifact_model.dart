import 'dart:convert';

enum ArtifactKind {
  image('image'),
  audio('audio'),
  video('video'),
  file('file');

  const ArtifactKind(this.value);
  final String value;

  static ArtifactKind fromMime(String mime) {
    if (mime.startsWith('image/')) return ArtifactKind.image;
    if (mime.startsWith('audio/')) return ArtifactKind.audio;
    if (mime.startsWith('video/')) return ArtifactKind.video;
    return ArtifactKind.file;
  }
}

enum ArtifactStatus {
  uploading('uploading'),
  ready('ready'),
  deleted('deleted');

  const ArtifactStatus(this.value);
  final String value;
}

class ArtifactMetadata {
  final String id;
  final String ownerUserId;
  final String workspaceId;
  final ArtifactKind kind;
  final String blobDigest;
  final int sizeBytes;
  final String mimeType;
  final String filename;
  final String fileExtension;
  final ArtifactStatus status;
  final String source;
  final int width;
  final int height;
  final int durationMs;
  final int revision;
  final DateTime createdAt;
  final DateTime updatedAt;

  const ArtifactMetadata({
    required this.id,
    required this.ownerUserId,
    required this.workspaceId,
    required this.kind,
    required this.blobDigest,
    required this.sizeBytes,
    required this.mimeType,
    required this.filename,
    required this.fileExtension,
    required this.status,
    required this.source,
    required this.width,
    required this.height,
    required this.durationMs,
    required this.revision,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ArtifactMetadata.fromJson(Map<String, dynamic> json) {
    return ArtifactMetadata(
      id: (json['artifactId'] ?? json['artifact_id'] ?? json['id'])?.toString() ?? '',
      ownerUserId: (json['ownerUserId'] ?? json['owner_user_id'])?.toString() ?? '',
      workspaceId: (json['workspaceId'] ?? json['workspace_id'])?.toString() ?? '',
      kind: ArtifactKind.values.firstWhere(
        (k) => k.value == json['kind'],
        orElse: () => ArtifactKind.file,
      ),
      blobDigest: (json['blobDigest'] ?? json['blob_digest'])?.toString() ?? '',
      sizeBytes: ((json['sizeBytes'] ?? json['size_bytes']) as num?)?.toInt() ?? 0,
      mimeType: (json['mimeType'] ?? json['mime_type'])?.toString() ?? '',
      filename: json['filename'] as String? ?? '',
      fileExtension: (json['fileExtension'] ?? json['file_extension'])?.toString() ?? '',
      status: ArtifactStatus.values.firstWhere(
        (s) => s.value == json['status'],
        orElse: () => ArtifactStatus.uploading,
      ),
      source: json['source'] as String? ?? '',
      width: (json['width'] as num?)?.toInt() ?? 0,
      height: (json['height'] as num?)?.toInt() ?? 0,
      durationMs: ((json['durationMs'] ?? json['duration_ms']) as num?)?.toInt() ?? 0,
      revision: (json['revision'] as num?)?.toInt() ?? 1,
      createdAt: DateTime.tryParse((json['createdAt'] ?? json['created_at'])?.toString() ?? '') ?? DateTime.now(),
      updatedAt: DateTime.tryParse((json['updatedAt'] ?? json['updated_at'])?.toString() ?? '') ?? DateTime.now(),
    );
  }

  String get resourceUri => 'amitia://artifacts/$id';

  Map<String, dynamic> toJson() => {
        'id': id,
        'owner_user_id': ownerUserId,
        'workspace_id': workspaceId,
        'kind': kind.value,
        'blob_digest': blobDigest,
        'size_bytes': sizeBytes,
        'mime_type': mimeType,
        'filename': filename,
        'file_extension': fileExtension,
        'status': status.value,
        'source': source,
        'width': width,
        'height': height,
        'duration_ms': durationMs,
        'revision': revision,
        'created_at': createdAt.toIso8601String(),
        'updated_at': updatedAt.toIso8601String(),
      };
}

String buildArtifactUri(String artifactId) => 'amitia://artifacts/$artifactId';

String? parseArtifactUri(String uri) {
  const prefix = 'amitia://artifacts/';
  if (!uri.startsWith(prefix)) return null;
  final id = uri.substring(prefix.length);
  if (id.isEmpty || id.contains('/') || id.contains('..')) return null;
  return id;
}
