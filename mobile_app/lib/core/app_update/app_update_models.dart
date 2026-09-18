class InstalledAppInfo {
  final String packageName;
  final String versionName;
  final int versionCode;
  final bool canInstallPackages;

  const InstalledAppInfo({
    required this.packageName,
    required this.versionName,
    required this.versionCode,
    required this.canInstallPackages,
  });

  factory InstalledAppInfo.fromMap(Map<Object?, Object?> value) {
    return InstalledAppInfo(
      packageName: (value['packageName'] ?? '').toString(),
      versionName: (value['versionName'] ?? '').toString(),
      versionCode: _asInt(value['versionCode']),
      canInstallPackages: value['canInstallPackages'] == true,
    );
  }
}

class AppUpdateApk {
  final String url;
  final int size;
  final String sha256;
  final String abi;

  const AppUpdateApk({
    required this.url,
    required this.size,
    required this.sha256,
    required this.abi,
  });

  factory AppUpdateApk.fromMap(Map<String, dynamic> value) {
    return AppUpdateApk(
      url: (value['url'] ?? '').toString(),
      size: _asInt(value['size']),
      sha256: (value['sha256'] ?? '').toString().toLowerCase(),
      abi: (value['abi'] ?? '').toString(),
    );
  }
}

class AppUpdateManifest {
  final int schemaVersion;
  final String product;
  final String packageName;
  final String channel;
  final int versionCode;
  final String versionName;
  final int minSupportedVersionCode;
  final bool mandatory;
  final int rolloutPercentage;
  final DateTime? publishedAt;
  final AppUpdateApk apk;
  final String releaseNotes;

  const AppUpdateManifest({
    required this.schemaVersion,
    required this.product,
    required this.packageName,
    required this.channel,
    required this.versionCode,
    required this.versionName,
    required this.minSupportedVersionCode,
    required this.mandatory,
    required this.rolloutPercentage,
    required this.publishedAt,
    required this.apk,
    required this.releaseNotes,
  });

  factory AppUpdateManifest.fromMap(Map<String, dynamic> value) {
    return AppUpdateManifest(
      schemaVersion: _asInt(value['schemaVersion']),
      product: (value['product'] ?? '').toString(),
      packageName: (value['packageName'] ?? '').toString(),
      channel: (value['channel'] ?? '').toString(),
      versionCode: _asInt(value['versionCode']),
      versionName: (value['versionName'] ?? '').toString(),
      minSupportedVersionCode: _asInt(value['minSupportedVersionCode']),
      mandatory: value['mandatory'] == true,
      rolloutPercentage: _asInt(value['rolloutPercentage']),
      publishedAt: DateTime.tryParse((value['publishedAt'] ?? '').toString()),
      apk: AppUpdateApk.fromMap(
        Map<String, dynamic>.from(
          value['apk'] as Map? ?? const <String, dynamic>{},
        ),
      ),
      releaseNotes: (value['releaseNotes'] ?? '').toString(),
    );
  }

  bool requiresVersion(int installedVersionCode) {
    return mandatory || installedVersionCode < minSupportedVersionCode;
  }
}

class AppUpdateCheckResult {
  final InstalledAppInfo installed;
  final AppUpdateManifest? available;
  final String? reason;

  const AppUpdateCheckResult({
    required this.installed,
    required this.available,
    required this.reason,
  });

  bool get hasUpdate => available != null;
}

class AppUpdateDownloadStatus {
  final int statusCode;
  final String status;
  final int downloadedBytes;
  final int totalBytes;
  final int reasonCode;
  final String? localUri;

  const AppUpdateDownloadStatus({
    required this.statusCode,
    required this.status,
    required this.downloadedBytes,
    required this.totalBytes,
    required this.reasonCode,
    required this.localUri,
  });

  factory AppUpdateDownloadStatus.fromMap(Map<Object?, Object?> value) {
    return AppUpdateDownloadStatus(
      statusCode: _asInt(value['statusCode']),
      status: (value['status'] ?? '').toString(),
      downloadedBytes: _asInt(value['downloadedBytes']),
      totalBytes: _asInt(value['totalBytes']),
      reasonCode: _asInt(value['reasonCode']),
      localUri: value['localUri']?.toString(),
    );
  }

  bool get successful => statusCode == 8;
  bool get failed => statusCode == 16 || statusCode < 0;

  double get progress {
    if (totalBytes <= 0) return 0;
    return (downloadedBytes / totalBytes).clamp(0, 1).toDouble();
  }
}

class AppUpdateInstallResult {
  final int statusCode;
  final String status;
  final String message;
  final int sessionId;
  final DateTime? timestamp;

  const AppUpdateInstallResult({
    required this.statusCode,
    required this.status,
    required this.message,
    required this.sessionId,
    required this.timestamp,
  });

  factory AppUpdateInstallResult.fromMap(Map<Object?, Object?> value) {
    final millis = _asInt(value['timestamp']);
    return AppUpdateInstallResult(
      statusCode: _asInt(value['statusCode']),
      status: (value['status'] ?? '').toString(),
      message: (value['message'] ?? '').toString(),
      sessionId: _asInt(value['sessionId']),
      timestamp: millis > 0
          ? DateTime.fromMillisecondsSinceEpoch(millis)
          : null,
    );
  }

  bool get successful => statusCode == 0;
}

int _asInt(Object? value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? 0;
}
