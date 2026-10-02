import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum BackgroundMediaKind { image, video }

@immutable
class BackgroundPreferences {
  final bool enabled;
  final BackgroundMediaKind kind;
  final String path;
  final String name;
  final double opacity;
  final bool blurEnabled;
  final double blurRadius;
  const BackgroundPreferences({
    this.enabled = false,
    this.kind = BackgroundMediaKind.image,
    this.path = '',
    this.name = '',
    this.opacity = 0.35,
    this.blurEnabled = false,
    this.blurRadius = 10,
  });
  bool get active => enabled && path.isNotEmpty;
  BackgroundPreferences copyWith({
    bool? enabled,
    BackgroundMediaKind? kind,
    String? path,
    String? name,
    double? opacity,
    bool? blurEnabled,
    double? blurRadius,
  }) => BackgroundPreferences(
    enabled: enabled ?? this.enabled,
    kind: kind ?? this.kind,
    path: path ?? this.path,
    name: name ?? this.name,
    opacity: (opacity ?? this.opacity).clamp(0, 1).toDouble(),
    blurEnabled: blurEnabled ?? this.blurEnabled,
    blurRadius: (blurRadius ?? this.blurRadius).clamp(0, 30).toDouble(),
  );
  String encode() => jsonEncode({
    'enabled': enabled,
    'kind': kind.name,
    'file': path.isEmpty ? '' : File(path).uri.pathSegments.last,
    'name': name,
    'opacity': opacity,
    'blurEnabled': blurEnabled,
    'blurRadius': blurRadius,
  });
  static BackgroundPreferences decode(String? raw, Directory directory) {
    try {
      final value = jsonDecode(raw ?? '{}');
      if (value is! Map<String, dynamic>) return const BackgroundPreferences();
      final file = value['file'];
      final validFile =
          file is String &&
          RegExp(
            r'^background-\d+\.(png|jpg|jpeg|webp|gif|mp4|webm)$',
          ).hasMatch(file);
      double number(String key, double fallback, double max) {
        final raw = value[key];
        return raw is num && raw.isFinite
            ? raw.clamp(0, max).toDouble()
            : fallback;
      }

      return BackgroundPreferences(
        enabled: value['enabled'] == true && validFile,
        kind: value['kind'] == 'video'
            ? BackgroundMediaKind.video
            : BackgroundMediaKind.image,
        path: validFile
            ? '${directory.path}${Platform.pathSeparator}$file'
            : '',
        name: value['name'] is String ? value['name'] : '',
        opacity: number('opacity', 0.35, 1),
        blurEnabled: value['blurEnabled'] == true,
        blurRadius: number('blurRadius', 10, 30),
      );
    } catch (_) {
      return const BackgroundPreferences();
    }
  }
}

BackgroundMediaKind validateBackgroundFile(String name, int size) {
  final extension = name.split('.').last.toLowerCase();
  final kind = ['mp4', 'webm'].contains(extension)
      ? BackgroundMediaKind.video
      : ['png', 'jpg', 'jpeg', 'webp', 'gif'].contains(extension)
      ? BackgroundMediaKind.image
      : null;
  if (kind == null) {
    throw const FormatException('请选择 PNG、JPG、WebP、GIF 图片或 MP4、WebM 视频');
  }
  if (size <= 0) throw const FormatException('背景文件为空');
  if (size > (kind == BackgroundMediaKind.video ? 150 : 20) * 1024 * 1024) {
    throw FormatException(
      kind == BackgroundMediaKind.video ? '视频不能超过 150 MB' : '图片不能超过 20 MB',
    );
  }
  return kind;
}

final backgroundPreferencesProvider =
    StateNotifierProvider<BackgroundPreferencesNotifier, BackgroundPreferences>(
      (ref) => BackgroundPreferencesNotifier(),
    );

class BackgroundPreferencesNotifier
    extends StateNotifier<BackgroundPreferences> {
  BackgroundPreferencesNotifier({Future<Directory> Function()? directory})
    : _directoryProvider = directory ?? getApplicationSupportDirectory,
      super(const BackgroundPreferences());
  final Future<Directory> Function() _directoryProvider;
  Future<void>? _initialization;
  Future<void> _queue = Future.value();
  Directory? _directory;
  Future<void> init() => _initialization ??= _load().catchError((Object error) {
    _initialization = null;
    throw error;
  });
  Future<void> _load() async {
    final root = await _directoryProvider();
    _directory = Directory(
      '${root.path}${Platform.pathSeparator}appearance-background',
    );
    final prefs = await SharedPreferences.getInstance();
    var next = BackgroundPreferences.decode(
      prefs.getString('appearance.background.v1'),
      _directory!,
    );
    if (next.path.isNotEmpty && !await File(next.path).exists()) {
      next = next.copyWith(enabled: false, path: '', name: '');
    }
    if (mounted) state = next;
  }

  Future<void> _serialize(Future<void> Function() action) {
    final next = _queue.then((_) => action());
    _queue = next.catchError((Object _) {});
    return next;
  }

  Future<void> _save(BackgroundPreferences value) async {
    final prefs = await SharedPreferences.getInstance();
    if (!await prefs.setString('appearance.background.v1', value.encode())) {
      throw StateError('保存背景失败');
    }
    if (mounted) state = value;
  }

  Future<void> update({
    bool? enabled,
    double? opacity,
    bool? blurEnabled,
    double? blurRadius,
  }) async {
    await init();
    await _serialize(() async {
      final next = state.copyWith(
        enabled: enabled,
        opacity: opacity,
        blurEnabled: blurEnabled,
        blurRadius: blurRadius,
      );
      if (next.enabled && next.path.isEmpty) throw StateError('请先选择背景文件');
      await _save(next);
    });
  }

  Future<void> importFile(String path, String name) async {
    final source = File(path);
    final kind = validateBackgroundFile(name, await source.length());
    await init();
    await _serialize(() async {
      await _directory!.create(recursive: true);
      final target = File(
        '${_directory!.path}${Platform.pathSeparator}background-${DateTime.now().microsecondsSinceEpoch}.${name.split('.').last.toLowerCase()}',
      );
      final previous = state.path;
      try {
        await source.copy(target.path);
        await _save(
          state.copyWith(
            enabled: true,
            kind: kind,
            path: target.path,
            name: name,
          ),
        );
      } catch (_) {
        if (await target.exists()) await target.delete();
        rethrow;
      }
      await _removeOwnedFile(previous);
    });
  }

  Future<void> clear() async {
    await init();
    await _serialize(() async {
      final previous = state.path;
      await _save(state.copyWith(enabled: false, path: '', name: ''));
      await _removeOwnedFile(previous);
    });
  }

  Future<void> _removeOwnedFile(String path) async {
    if (path.isEmpty ||
        File(path).parent.absolute.path != _directory!.absolute.path) {
      return;
    }
    try {
      final file = File(path);
      if (await file.exists()) await file.delete();
    } catch (_) {}
  }
}
