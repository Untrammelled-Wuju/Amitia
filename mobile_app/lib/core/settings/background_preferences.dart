import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum BackgroundMediaKind { image, video }

enum BackgroundDisplayMode { defaultMode, color, gradient, image, video }

@immutable
class BackgroundPreferences {
  final bool enabled;
  final BackgroundMediaKind kind;
  final String path;
  final String name;
  final double opacity;
  final bool blurEnabled;
  final double blurRadius;
  final BackgroundDisplayMode mode;
  final Color color;
  final Color gradientStart;
  final Color gradientEnd;
  final double gradientAngle;
  final String sourceUri;
  final String fit;
  final Color overlayColor;
  final double overlayOpacity;
  const BackgroundPreferences({
    this.enabled = false,
    this.kind = BackgroundMediaKind.image,
    this.path = '',
    this.name = '',
    this.opacity = 0.35,
    this.blurEnabled = false,
    this.blurRadius = 10,
    this.mode = BackgroundDisplayMode.defaultMode,
    this.color = const Color(0xFFF7F8FA),
    this.gradientStart = const Color(0xFFF7F8FA),
    this.gradientEnd = const Color(0xFFE8EDF5),
    this.gradientAngle = 180,
    this.sourceUri = '',
    this.fit = 'cover',
    this.overlayColor = const Color(0xFF000000),
    this.overlayOpacity = 0,
  });
  BackgroundDisplayMode get effectiveMode =>
      mode == BackgroundDisplayMode.defaultMode &&
          (path.isNotEmpty || sourceUri.isNotEmpty)
      ? kind == BackgroundMediaKind.video
            ? BackgroundDisplayMode.video
            : BackgroundDisplayMode.image
      : mode;
  bool get active =>
      enabled &&
      switch (effectiveMode) {
        BackgroundDisplayMode.defaultMode => false,
        BackgroundDisplayMode.color || BackgroundDisplayMode.gradient => true,
        BackgroundDisplayMode.image ||
        BackgroundDisplayMode.video => path.isNotEmpty || sourceUri.isNotEmpty,
      };
  BackgroundPreferences copyWith({
    bool? enabled,
    BackgroundMediaKind? kind,
    String? path,
    String? name,
    double? opacity,
    bool? blurEnabled,
    double? blurRadius,
    BackgroundDisplayMode? mode,
    Color? color,
    Color? gradientStart,
    Color? gradientEnd,
    double? gradientAngle,
    String? sourceUri,
    String? fit,
    Color? overlayColor,
    double? overlayOpacity,
  }) => BackgroundPreferences(
    enabled: enabled ?? this.enabled,
    kind: kind ?? this.kind,
    path: path ?? this.path,
    name: name ?? this.name,
    opacity: (opacity ?? this.opacity).clamp(0, 1).toDouble(),
    blurEnabled: blurEnabled ?? this.blurEnabled,
    blurRadius: (blurRadius ?? this.blurRadius).clamp(0, 30).toDouble(),
    mode: mode ?? this.mode,
    color: color ?? this.color,
    gradientStart: gradientStart ?? this.gradientStart,
    gradientEnd: gradientEnd ?? this.gradientEnd,
    gradientAngle: (gradientAngle ?? this.gradientAngle)
        .clamp(0, 360)
        .toDouble(),
    sourceUri: sourceUri ?? this.sourceUri,
    fit: fit ?? this.fit,
    overlayColor: overlayColor ?? this.overlayColor,
    overlayOpacity: (overlayOpacity ?? this.overlayOpacity)
        .clamp(0, 1)
        .toDouble(),
  );
  String encode() => jsonEncode({
    'enabled': enabled,
    'kind': kind.name,
    'mode': mode.name,
    'file': path.isEmpty ? '' : File(path).uri.pathSegments.last,
    'name': name,
    'opacity': opacity,
    'blurEnabled': blurEnabled,
    'blurRadius': blurRadius,
    'color': color.toARGB32(),
    'gradientStart': gradientStart.toARGB32(),
    'gradientEnd': gradientEnd.toARGB32(),
    'gradientAngle': gradientAngle,
    'sourceUri': sourceUri,
    'fit': fit,
    'overlayColor': overlayColor.toARGB32(),
    'overlayOpacity': overlayOpacity,
  });
  Map<String, dynamic> toMap() => <String, dynamic>{
    'enabled': enabled,
    'kind': kind.name,
    'mode': mode.name,
    'path': path,
    'name': name,
    'opacity': opacity,
    'blurEnabled': blurEnabled,
    'blurRadius': blurRadius,
    'color': color.toARGB32(),
    'gradientStart': gradientStart.toARGB32(),
    'gradientEnd': gradientEnd.toARGB32(),
    'gradientAngle': gradientAngle,
    'sourceUri': sourceUri,
    'fit': fit,
    'overlayColor': overlayColor.toARGB32(),
    'overlayOpacity': overlayOpacity,
  };
  factory BackgroundPreferences.fromMap(Map<String, dynamic> raw) {
    Color color(String key, Color fallback) {
      final value = raw[key];
      return value is int && value >= 0 && value <= 0xFFFFFFFF
          ? Color(value | 0xFF000000)
          : fallback;
    }

    const defaults = BackgroundPreferences();
    return BackgroundPreferences(
      enabled: raw['enabled'] == true,
      kind: raw['kind'] == 'video'
          ? BackgroundMediaKind.video
          : BackgroundMediaKind.image,
      mode: BackgroundDisplayMode.values.firstWhere(
        (item) => item.name == raw['mode'],
        orElse: () => BackgroundDisplayMode.defaultMode,
      ),
      path: raw['path']?.toString() ?? '',
      name: raw['name']?.toString() ?? '',
      opacity: ((raw['opacity'] as num?)?.toDouble() ?? defaults.opacity)
          .clamp(0, 1)
          .toDouble(),
      blurEnabled: raw['blurEnabled'] == true,
      blurRadius:
          ((raw['blurRadius'] as num?)?.toDouble() ?? defaults.blurRadius)
              .clamp(0, 30)
              .toDouble(),
      color: color('color', defaults.color),
      gradientStart: color('gradientStart', defaults.gradientStart),
      gradientEnd: color('gradientEnd', defaults.gradientEnd),
      gradientAngle:
          ((raw['gradientAngle'] as num?)?.toDouble() ?? defaults.gradientAngle)
              .clamp(0, 360)
              .toDouble(),
      sourceUri: raw['sourceUri']?.toString() ?? '',
      fit: const ['cover', 'contain', 'stretch', 'repeat'].contains(raw['fit'])
          ? raw['fit'].toString()
          : defaults.fit,
      overlayColor: color('overlayColor', defaults.overlayColor),
      overlayOpacity:
          ((raw['overlayOpacity'] as num?)?.toDouble() ??
                  defaults.overlayOpacity)
              .clamp(0, 1)
              .toDouble(),
    );
  }
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

      Color colorValue(String key, Color fallback) {
        final raw = value[key];
        return raw is int && raw >= 0 && raw <= 0xFFFFFFFF
            ? Color(raw | 0xFF000000)
            : fallback;
      }

      return BackgroundPreferences(
        enabled:
            value['enabled'] == true &&
            (validFile ||
                value['mode'] == 'color' ||
                value['mode'] == 'gradient' ||
                (value['sourceUri'] is String &&
                    (value['sourceUri'] as String).isNotEmpty)),
        kind: value['kind'] == 'video'
            ? BackgroundMediaKind.video
            : BackgroundMediaKind.image,
        mode: BackgroundDisplayMode.values.firstWhere(
          (item) => item.name == value['mode'],
          orElse: () => value['kind'] == 'video'
              ? BackgroundDisplayMode.video
              : BackgroundDisplayMode.image,
        ),
        path: validFile
            ? '${directory.path}${Platform.pathSeparator}$file'
            : '',
        name: value['name'] is String ? value['name'] : '',
        opacity: number('opacity', 0.35, 1),
        blurEnabled: value['blurEnabled'] == true,
        blurRadius: number('blurRadius', 10, 30),
        color: colorValue('color', const Color(0xFFF7F8FA)),
        gradientStart: colorValue('gradientStart', const Color(0xFFF7F8FA)),
        gradientEnd: colorValue('gradientEnd', const Color(0xFFE8EDF5)),
        gradientAngle: number('gradientAngle', 180, 360),
        sourceUri: value['sourceUri'] is String
            ? value['sourceUri'] as String
            : '',
        fit:
            const [
              'cover',
              'contain',
              'stretch',
              'repeat',
            ].contains(value['fit'])
            ? value['fit'] as String
            : 'cover',
        overlayColor: colorValue('overlayColor', const Color(0xFF000000)),
        overlayOpacity: number('overlayOpacity', 0, 1),
      );
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
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
    _queue = next.catchError((Object error, StackTrace stackTrace) {
  print("[original-error] $error\n$stackTrace");
});
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
      if (next.enabled && !next.active) throw StateError('请先选择背景文件');
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
            mode: kind == BackgroundMediaKind.video
                ? BackgroundDisplayMode.video
                : BackgroundDisplayMode.image,
            path: target.path,
            sourceUri: '',
            name: name,
          ),
        );
      } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
if (await target.exists()) await target.delete();
        rethrow;
      }
      await _removeOwnedFile(previous);
    });
  }

  Future<void> importBytes(
    Uint8List bytes,
    String name,
    BackgroundMediaKind kind, {
    String sourceUri = '',
  }) async {
    if (bytes.isEmpty) throw const FormatException('背景文件为空');
    final checkedKind = validateBackgroundFile(name, bytes.length);
    if (checkedKind != kind) throw const FormatException('背景文件类型不匹配');
    await init();
    await _serialize(() async {
      await _directory!.create(recursive: true);
      final target = File(
        '${_directory!.path}${Platform.pathSeparator}background-${DateTime.now().microsecondsSinceEpoch}.${name.split('.').last.toLowerCase()}',
      );
      try {
        await target.writeAsBytes(bytes, flush: true);
        await _save(
          state.copyWith(
            enabled: true,
            kind: kind,
            mode: kind == BackgroundMediaKind.video
                ? BackgroundDisplayMode.video
                : BackgroundDisplayMode.image,
            path: target.path,
            sourceUri: sourceUri,
            name: name,
          ),
        );
      } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
if (await target.exists()) await target.delete();
        rethrow;
      }
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

  Future<void> replace(BackgroundPreferences value) async {
    await init();
    await _serialize(() async {
      await _save(value);
    });
  }

  Future<void> reset() => replace(const BackgroundPreferences());

  Future<void> _removeOwnedFile(String path) async {
    if (path.isEmpty ||
        File(path).parent.absolute.path != _directory!.absolute.path) {
      return;
    }
    try {
      final file = File(path);
      if (await file.exists()) await file.delete();
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
}
  }
}
