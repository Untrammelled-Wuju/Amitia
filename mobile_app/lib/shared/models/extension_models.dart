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
    this.isEnabled = true,
    this.version = '1.0.0',
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
