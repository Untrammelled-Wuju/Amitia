import 'package:flutter/material.dart';

class CharacterVoiceConfig {
  final String id;
  final String name;
  final String preset;
  final double speed;
  final double pitch;
  final double volume;
  final bool isCurrent;

  CharacterVoiceConfig({
    required this.id,
    required this.name,
    required this.preset,
    this.speed = 1.0,
    this.pitch = 1.0,
    this.volume = 0.8,
    this.isCurrent = false,
  });
}

class ProactiveRule {
  final String id;
  final String name;
  final String trigger;
  final String time;
  final int probability;
  final int cooldown;
  final bool isEnabled;
  final String category;

  ProactiveRule({
    required this.id,
    required this.name,
    required this.trigger,
    required this.time,
    this.probability = 80,
    this.cooldown = 60,
    this.isEnabled = true,
    this.category = '日常',
  });
}

class FixedSchedule {
  final String id;
  final String title;
  final String startTime;
  final String endTime;
  final bool repeat;
  final String category;

  FixedSchedule({
    required this.id,
    required this.title,
    required this.startTime,
    required this.endTime,
    this.repeat = true,
    this.category = '日常',
  });
}

class SpecialState {
  final String id;
  final String name;
  final String description;
  final bool isActive;

  SpecialState({
    required this.id,
    required this.name,
    required this.description,
    this.isActive = false,
  });
}

class PsycheState {
  final String emotion;
  final int intensity;
  final int stability;
  final String influence;
  final String relationshipStatus;
  final DateTime time;

  PsycheState({
    required this.emotion,
    required this.intensity,
    required this.stability,
    required this.influence,
    required this.relationshipStatus,
    required this.time,
  });
}

class TimelineEvent {
  final String id;
  final DateTime time;
  final String type;
  final String title;
  final String description;
  final String? emotion;

  TimelineEvent({
    required this.id,
    required this.time,
    required this.type,
    required this.title,
    required this.description,
    this.emotion,
  });
}

class CharacterLifeRules {
  final String prompt;
  final String personality;
  final int personalityScore;
  final String relationshipTime;
  final String workStatus;
  final String sleepSettings;
  final String dailyTendency;
  final List<FixedSchedule> fixedSchedules;
  final List<SpecialState> specialStates;
  final bool timeAwareness;
  final String emoteSettings;

  CharacterLifeRules({
    required this.prompt,
    required this.personality,
    this.personalityScore = 50,
    this.relationshipTime = '128天',
    this.workStatus = '工作中',
    this.sleepSettings = '23:00 - 07:00',
    this.dailyTendency = '积极',
    this.fixedSchedules = const [],
    this.specialStates = const [],
    this.timeAwareness = true,
    this.emoteSettings = '默认表情包',
  });
}
