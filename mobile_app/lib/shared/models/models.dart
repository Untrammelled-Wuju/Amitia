import 'package:flutter/material.dart';

export 'character_models.dart';
export 'memory_models.dart';
export 'extension_models.dart';
export 'workshop_models.dart';
export 'settings_models.dart';
export 'channel_models.dart';
export 'dashboard_models.dart';
export 'kernel_models.dart';

enum MessageRole { user, assistant, system }
enum MessageStatus { sending, sent, delivered, error }
enum MessageType { text, file, image, video, audio, emote, code, agentTask, toolCall, systemNotice }

class ChatMessage {
  final String id;
  final MessageRole role;
  final MessageType type;
  final String content;
  final DateTime time;
  final MessageStatus status;
  final String? agentTaskTitle;
  final List<String>? agentTaskSteps;
  final int? agentTaskProgress;
  final String? agentTaskElapsed;
  final String? fileName;
  final int? fileSizeKB;
  final String? resourceUri;
  final String? mediaUrl;
  final String? mimeType;
  final int? durationMs;
  final String? toolName;
  final String? toolResult;
  final String? replyToMessageId;
  final String? replyToExcerpt;

  ChatMessage({
    required this.id,
    required this.role,
    required this.type,
    required this.content,
    required this.time,
    this.status = MessageStatus.sent,
    this.agentTaskTitle,
    this.agentTaskSteps,
    this.agentTaskProgress,
    this.agentTaskElapsed,
    this.fileName,
    this.fileSizeKB,
    this.resourceUri,
    this.mediaUrl,
    this.mimeType,
    this.durationMs,
    this.toolName,
    this.toolResult,
    this.replyToMessageId,
    this.replyToExcerpt,
  });
}

class Conversation {
  final String id;
  final String title;
  final String lastMessage;
  final DateTime lastTime;
  final bool isPinned;
  final String characterId;

  Conversation({
    required this.id,
    required this.title,
    required this.lastMessage,
    required this.lastTime,
    required this.characterId,
    this.isPinned = false,
  });
}

class Character {
  final String id;
  final String name;
  final String avatarColor;
  final String avatarInitial;
  final String status;
  final String mood;
  final String identity;
  final String description;
  final int relationshipDays;
  final int messageCount;
  final String personality;
  final String speakingStyle;
  final String userRelation;
  final String prompt;
  final String currentActivity;
  final String location;

  Character({
    required this.id,
    required this.name,
    required this.avatarColor,
    required this.avatarInitial,
    required this.status,
    required this.mood,
    required this.identity,
    required this.description,
    required this.relationshipDays,
    required this.messageCount,
    required this.personality,
    required this.speakingStyle,
    required this.userRelation,
    required this.prompt,
    required this.currentActivity,
    required this.location,
  });
}

enum AgentTaskStatus { running, pending, completed, paused }
enum AgentTaskPriority { low, medium, high }

class AgentTask {
  final String id;
  final String title;
  final String currentStep;
  final int progress;
  final AgentTaskStatus status;
  final String? elapsed;
  final String? category;
  final List<String>? requiredPermissions;
  final List<AgentTaskStep>? steps;
  final String? result;
  final DateTime createdAt;

  AgentTask({
    required this.id,
    required this.title,
    required this.currentStep,
    required this.progress,
    required this.status,
    this.elapsed,
    this.category,
    this.requiredPermissions,
    this.steps,
    this.result,
    required this.createdAt,
  });
}

class AgentTaskStep {
  final String name;
  final String status;
  final String? duration;

  AgentTaskStep({required this.name, required this.status, this.duration});
}

class Memory {
  final String id;
  final String content;
  final String source;
  final String importance;
  final DateTime time;
  final String category;
  bool isPinned;

  Memory({
    required this.id,
    required this.content,
    required this.source,
    required this.importance,
    required this.time,
    required this.category,
    this.isPinned = false,
  });
}

enum ExtensionType { mcp, skill, plugin, theme }
enum ExtensionStatus { installed, notInstalled, enabled, disabled }

class Extension {
  final String id;
  final String name;
  final String description;
  final ExtensionType type;
  final IconData icon;
  final bool isInstalled;
  final bool isEnabled;
  final bool isRecommended;

  Extension({
    required this.id,
    required this.name,
    required this.description,
    required this.type,
    required this.icon,
    required this.isInstalled,
    required this.isEnabled,
    this.isRecommended = false,
  });
}

class RuntimeInfo {
  final String status;
  final String version;
  final String backendStatus;
  final String storageUsage;
  final List<RuntimeComponent> components;

  RuntimeInfo({
    required this.status,
    required this.version,
    required this.backendStatus,
    required this.storageUsage,
    required this.components,
  });
}

class RuntimeComponent {
  final String name;
  final String status;

  RuntimeComponent({required this.name, required this.status});
}

class PermissionItem {
  final String name;
  final IconData icon;
  final String status;
  final String description;

  PermissionItem({
    required this.name,
    required this.icon,
    required this.status,
    required this.description,
  });
}

class SettingItem {
  final String title;
  final IconData icon;
  final String? value;
  final String? subtitle;
  final String route;

  SettingItem({required this.title, required this.icon, this.value, this.subtitle, required this.route});
}

class SettingGroup {
  final String title;
  final List<SettingItem> items;

  SettingGroup({required this.title, required this.items});
}

class ToolCallRecord {
  final String toolName;
  final String input;
  final String output;
  final String duration;
  final String status;

  ToolCallRecord({
    required this.toolName,
    required this.input,
    required this.output,
    required this.duration,
    required this.status,
  });
}

class QuickTask {
  final String title;
  final IconData icon;
  final String category;

  QuickTask({required this.title, required this.icon, required this.category});
}
