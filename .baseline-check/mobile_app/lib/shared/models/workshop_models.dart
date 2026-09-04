import 'package:flutter/material.dart';

enum PetTaskStatus { pending, processing, completed, cancelled }
enum ProcessingStatus { pending, reviewing, approved, rejected }

class SkillDraft {
  final String id;
  final String name;
  final String description;
  final String metadata;
  final String inputSchema;
  final String outputSchema;
  final String riskAssessment;
  final String testResult;
  final String status;
  final DateTime updated;

  SkillDraft({
    required this.id,
    required this.name,
    required this.description,
    this.metadata = '',
    this.inputSchema = '',
    this.outputSchema = '',
    this.riskAssessment = '低风险',
    this.testResult = '未测试',
    this.status = '草稿',
    required this.updated,
  });
}

class PetTask {
  final String id;
  final String name;
  final String characterName;
  final int totalActions;
  final int completedActions;
  final PetTaskStatus status;
  final int progress;
  final DateTime createdAt;

  PetTask({
    required this.id,
    required this.name,
    required this.characterName,
    this.totalActions = 8,
    this.completedActions = 0,
    this.status = PetTaskStatus.pending,
    this.progress = 0,
    required this.createdAt,
  });
}

class ProcessingTask {
  final String id;
  final String petTaskId;
  final String actionKey;
  final String actionName;
  final int totalFrames;
  final int completedFrames;
  final ProcessingStatus status;
  final String qualityStatus;
  final List<FrameEntry> frames;
  final List<AttemptEntry> attempts;

  ProcessingTask({
    required this.id,
    required this.petTaskId,
    required this.actionKey,
    required this.actionName,
    this.totalFrames = 8,
    this.completedFrames = 0,
    this.status = ProcessingStatus.pending,
    this.qualityStatus = '待审核',
    this.frames = const [],
    this.attempts = const [],
  });
}

class FrameEntry {
  final int index;
  final String status;
  final String? qualityLabel;

  FrameEntry({required this.index, required this.status, this.qualityLabel});
}

class AttemptEntry {
  final String id;
  final String label;
  final bool isSelected;

  AttemptEntry({required this.id, required this.label, this.isSelected = false});
}

class PetInstallation {
  final String id;
  final String name;
  final String characterName;
  final bool isEnabled;
  final bool isRunning;
  final double scale;
  final String defaultAction;
  final List<String> actions;

  PetInstallation({
    required this.id,
    required this.name,
    required this.characterName,
    this.isEnabled = true,
    this.isRunning = false,
    this.scale = 1.0,
    this.defaultAction = 'idle',
    this.actions = const ['idle', 'wave', 'happy', 'speaking'],
  });
}

class WorkshopSession {
  final String id;
  final String title;
  final String type;
  final String status;
  final DateTime updated;

  WorkshopSession({
    required this.id,
    required this.title,
    required this.type,
    this.status = '进行中',
    required this.updated,
  });
}
