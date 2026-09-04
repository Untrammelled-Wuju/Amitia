import 'package:flutter/material.dart';

enum ModelType { llm, voice, embedding, vision, imagegen }

class ModelConfig {
  final String id;
  final String name;
  final ModelType type;
  final String provider;
  final String baseUrl;
  final String modelName;
  final String apiKey;
  final bool isActive;
  final String? assignedScene;
  final Map<String, dynamic> params;

  ModelConfig({
    required this.id,
    required this.name,
    required this.type,
    required this.provider,
    this.baseUrl = '',
    this.modelName = '',
    this.apiKey = '',
    this.isActive = false,
    this.assignedScene,
    this.params = const {},
  });
}

class SafetySettings {
  final String permissionMode;
  final bool sensitiveOperationApproval;
  final bool dataAccessApproval;
  final bool modelOutputBoundary;
  final int safetyEventCount;
  final int privacyIssueCount;

  SafetySettings({
    this.permissionMode = '手动审批',
    this.sensitiveOperationApproval = true,
    this.dataAccessApproval = true,
    this.modelOutputBoundary = true,
    this.safetyEventCount = 3,
    this.privacyIssueCount = 1,
  });
}

class MaintenanceCheck {
  final String name;
  final String status;
  final String? detail;

  MaintenanceCheck({required this.name, required this.status, this.detail});
}

class StorageInfo {
  final String category;
  final String size;
  final double percentage;
  final Color color;

  StorageInfo({
    required this.category,
    required this.size,
    required this.percentage,
    required this.color,
  });
}

class ThemeSettings {
  final ThemeMode mode;
  final double fontScale;
  final bool animationEnabled;

  ThemeSettings({
    this.mode = ThemeMode.light,
    this.fontScale = 1.0,
    this.animationEnabled = true,
  });
}

class UserSettings {
  final String avatar;
  final String username;
  final String nickname;
  final String userLabel;
  final String bio;

  UserSettings({
    this.avatar = 'U',
    this.username = 'user',
    this.nickname = '用户',
    this.userLabel = '主人',
    this.bio = '',
  });
}

class PrivacyScanResult {
  final String category;
  final int riskCount;
  final String riskLevel;

  PrivacyScanResult({
    required this.category,
    required this.riskCount,
    required this.riskLevel,
  });
}

class TimeAnchor {
  final String id;
  final String scopeType;
  final String characterId;
  final String anchorType;
  final String title;
  final String description;
  final String timeKind;
  final String instantAtUtc;
  final String endAtUtc;
  final String localDate;
  final String localTime;
  final String timezone;
  final String rrule;
  final int durationSeconds;
  final int preWindowSeconds;
  final int postWindowSeconds;
  final String sensitivityLevel;
  final int importance;
  final int confidence;
  final bool allowPromptMention;
  final bool allowProactiveMention;
  final bool requiresConfirmation;
  final String source;
  final String status;
  final String nextOccurrenceAtUtc;

  const TimeAnchor({
    required this.id,
    this.scopeType = 'user',
    this.characterId = '',
    this.anchorType = 'custom',
    required this.title,
    this.description = '',
    required this.timeKind,
    this.instantAtUtc = '',
    this.endAtUtc = '',
    this.localDate = '',
    this.localTime = '',
    this.timezone = 'Asia/Shanghai',
    this.rrule = '',
    this.durationSeconds = 0,
    this.preWindowSeconds = 259200,
    this.postWindowSeconds = 86400,
    this.sensitivityLevel = 'internal',
    this.importance = 70,
    this.confidence = 100,
    this.allowPromptMention = true,
    this.allowProactiveMention = false,
    this.requiresConfirmation = false,
    this.source = 'manual',
    this.status = 'active',
    this.nextOccurrenceAtUtc = '',
  });

  factory TimeAnchor.fromJson(Map<String, dynamic> json) {
    return TimeAnchor(
      id: (json['id'] ?? '').toString(),
      scopeType: (json['scopeType'] ?? 'user').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      anchorType: (json['anchorType'] ?? 'custom').toString(),
      title: (json['title'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      timeKind: (json['timeKind'] ?? 'local_date').toString(),
      instantAtUtc: (json['instantAtUtc'] ?? '').toString(),
      endAtUtc: (json['endAtUtc'] ?? '').toString(),
      localDate: (json['localDate'] ?? '').toString(),
      localTime: (json['localTime'] ?? '').toString(),
      timezone: (json['timezone'] ?? 'Asia/Shanghai').toString(),
      rrule: (json['rrule'] ?? '').toString(),
      durationSeconds: (json['durationSeconds'] as num?)?.toInt() ?? 0,
      preWindowSeconds: (json['preWindowSeconds'] as num?)?.toInt() ?? 259200,
      postWindowSeconds: (json['postWindowSeconds'] as num?)?.toInt() ?? 86400,
      sensitivityLevel: (json['sensitivityLevel'] ?? 'internal').toString(),
      importance: (json['importance'] as num?)?.toInt() ?? 70,
      confidence: (json['confidence'] as num?)?.toInt() ?? 100,
      allowPromptMention: json['allowPromptMention'] as bool? ?? true,
      allowProactiveMention: json['allowProactiveMention'] as bool? ?? false,
      requiresConfirmation: json['requiresConfirmation'] as bool? ?? false,
      source: (json['source'] ?? 'manual').toString(),
      status: (json['status'] ?? 'active').toString(),
      nextOccurrenceAtUtc: (json['nextOccurrenceAtUtc'] ?? '').toString(),
    );
  }

  bool get isPeriodic => timeKind == 'recurring';

  bool get isSpecialDate =>
      timeKind == 'annual_date' ||
      timeKind == 'local_date' ||
      timeKind == 'local_datetime';

  String get displayType {
    switch (timeKind) {
      case 'recurring':
        return '周期锚点';
      case 'annual_date':
        return '每年日期';
      case 'local_date':
      case 'local_datetime':
        return '单次日期';
      case 'instant':
        return '时间点';
      case 'range':
        return '时间范围';
      default:
        return '其他锚点';
    }
  }

  String get displayValue {
    if (timeKind == 'instant') return instantAtUtc.isEmpty ? '—' : instantAtUtc;
    if (timeKind == 'range') {
      final start = instantAtUtc.isEmpty ? '—' : instantAtUtc;
      final end = endAtUtc.isEmpty ? '—' : endAtUtc;
      return '$start → $end';
    }
    final parts = <String>[
      if (localDate.isNotEmpty) localDate,
      if (localTime.isNotEmpty) localTime,
    ];
    if (timeKind == 'recurring' && rrule.isNotEmpty) parts.add(rrule);
    return parts.isEmpty ? '—' : parts.join(' · ');
  }
}

class DeploymentConfig {
  final String mode;
  final String description;

  DeploymentConfig({
    this.mode = '本地',
    this.description = '完整功能本地运行',
  });
}

class AiConfig {
  final String defaultCharacter;
  final String defaultModel;
  final String contextStrategy;
  final bool streamingOutput;
  final bool messageSplitting;
  final bool toolCalls;
  final String errorFallback;

  AiConfig({
    this.defaultCharacter = 'Amitia',
    this.defaultModel = 'GPT-4',
    this.contextStrategy = '滑动窗口',
    this.streamingOutput = true,
    this.messageSplitting = true,
    this.toolCalls = true,
    this.errorFallback = '简单回复',
  });
}
