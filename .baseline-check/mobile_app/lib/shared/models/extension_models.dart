import 'package:flutter/material.dart';

enum McpTransport { stdio, sse, websocket }
enum McpStatus { connected, disconnected, error, connecting }

class McpServer {
  final String id;
  final String name;
  final McpTransport transport;
  final String address;
  final McpStatus status;
  final int toolCount;
  final int promptCount;
  final int resourceCount;
  final bool hasSampling;
  final bool hasTasks;
  final bool hasRoots;
  final bool hasOAuth;
  final Map<String, String> envVars;
  final List<McpTool> tools;
  final List<McpPrompt> prompts;
  final List<McpResource> resources;

  McpServer({
    required this.id,
    required this.name,
    required this.transport,
    required this.address,
    required this.status,
    this.toolCount = 0,
    this.promptCount = 0,
    this.resourceCount = 0,
    this.hasSampling = false,
    this.hasTasks = false,
    this.hasRoots = false,
    this.hasOAuth = false,
    this.envVars = const {},
    this.tools = const [],
    this.prompts = const [],
    this.resources = const [],
  });
}

class McpTool {
  final String name;
  final String description;
  final bool isEnabled;

  McpTool({required this.name, required this.description, this.isEnabled = true});
}

class McpPrompt {
  final String name;
  final String description;
  final String content;

  McpPrompt({required this.name, required this.description, required this.content});
}

class McpResource {
  final String uri;
  final String name;
  final String mimeType;
  final String? content;

  McpResource({required this.uri, required this.name, required this.mimeType, this.content});
}

class AgentSkill {
  final String id;
  final String name;
  final String description;
  final String skillMd;
  final List<String> requiredMcp;
  final String compatibility;
  final bool isEnabled;
  final String version;

  AgentSkill({
    required this.id,
    required this.name,
    required this.description,
    this.skillMd = '',
    this.requiredMcp = const [],
    this.compatibility = '兼容',
    this.isEnabled = true,
    this.version = '1.0.0',
  });
}

class SystemPlugin {
  final String id;
  final String name;
  final String description;
  final String runtimeStatus;
  final List<String> hooks;
  final List<String> events;
  final List<String> schedules;
  final List<String> registeredSkills;
  final bool isEnabled;
  final String version;

  SystemPlugin({
    required this.id,
    required this.name,
    required this.description,
    this.runtimeStatus = '运行中',
    this.hooks = const [],
    this.events = const [],
    this.schedules = const [],
    this.registeredSkills = const [],
    this.isEnabled = true,
    this.version = '1.0.0',
  });
}

class CompatibleSkill {
  final String id;
  final String name;
  final String description;
  final String version;
  final String? previousVersion;
  final bool isEnabled;
  final String? lastTestResult;

  CompatibleSkill({
    required this.id,
    required this.name,
    required this.description,
    this.version = '1.0.0',
    this.previousVersion,
    this.isEnabled = true,
    this.lastTestResult,
  });
}

class ExecutionRun {
  final String id;
  final String name;
  final String status;
  final String duration;
  final String input;
  final String output;
  final String? error;
  final List<ToolCallEntry> toolCalls;
  final DateTime startTime;

  ExecutionRun({
    required this.id,
    required this.name,
    required this.status,
    required this.duration,
    required this.input,
    required this.output,
    this.error,
    this.toolCalls = const [],
    required this.startTime,
  });
}

class ToolCallEntry {
  final String toolName;
  final String input;
  final String output;
  final String duration;
  final String status;

  ToolCallEntry({
    required this.toolName,
    required this.input,
    required this.output,
    required this.duration,
    required this.status,
  });
}

class ExtensionPackage {
  final String id;
  final String name;
  final String description;
  final String version;
  final String status;
  final List<String> permissions;
  final IconData icon;
  final bool hasUpdate;

  ExtensionPackage({
    required this.id,
    required this.name,
    required this.description,
    required this.version,
    required this.status,
    this.permissions = const [],
    required this.icon,
    this.hasUpdate = false,
  });
}
