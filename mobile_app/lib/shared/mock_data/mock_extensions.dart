import 'package:flutter/material.dart';
import '../models/models.dart';

class MockExtensions {
  MockExtensions._();

  static List<ExtensionPackage> packages = [
    ExtensionPackage(id: 'ep1', name: '文件系统', description: '提供文件读写、目录管理等能力', version: '1.2.0', status: '运行中', permissions: ['文件读写', '目录访问'], icon: Icons.folder_outlined),
    ExtensionPackage(id: 'ep2', name: 'Web 搜索', description: '搜索互联网获取最新信息', version: '2.0.1', status: '运行中', permissions: ['网络访问'], icon: Icons.language, hasUpdate: true),
    ExtensionPackage(id: 'ep3', name: '代码执行', description: '在沙箱环境中执行代码', version: '1.5.0', status: '已暂停', permissions: ['代码执行', '网络访问'], icon: Icons.code),
    ExtensionPackage(id: 'ep4', name: '图像理解', description: '识别和分析图片内容', version: '0.9.0', status: '已安装', permissions: ['图像访问'], icon: Icons.image_outlined),
    ExtensionPackage(id: 'ep5', name: 'PDF 分析器', description: '解析和分析 PDF 文档内容', version: '1.0.0', status: '运行中', permissions: ['文件读取'], icon: Icons.picture_as_pdf_outlined),
  ];

  static List<McpServer> mcpServers = [
    McpServer(
      id: 'mcp1', name: '文件系统 MCP', transport: McpTransport.stdio, address: 'filesystem-server', status: McpStatus.connected,
      toolCount: 8, promptCount: 2, resourceCount: 5, hasSampling: false, hasTasks: true, hasRoots: true, hasOAuth: false,
      tools: [
        McpTool(name: 'read_file', description: '读取文件内容'),
        McpTool(name: 'write_file', description: '写入文件内容'),
        McpTool(name: 'list_directory', description: '列出目录内容'),
        McpTool(name: 'create_directory', description: '创建目录'),
        McpTool(name: 'move_file', description: '移动文件'),
        McpTool(name: 'search_files', description: '搜索文件'),
        McpTool(name: 'delete_file', description: '删除文件', isEnabled: false),
        McpTool(name: 'get_file_info', description: '获取文件信息'),
      ],
      prompts: [
        McpPrompt(name: 'summarize_file', description: '总结文件内容', content: '请总结以下文件的核心内容：{{file_content}}'),
        McpPrompt(name: 'compare_files', description: '比较文件差异', content: '请比较以下两个文件的差异：{{file1}} {{file2}}'),
      ],
      resources: [
        McpResource(uri: 'file:///downloads', name: '下载目录', mimeType: 'directory'),
        McpResource(uri: 'file:///documents', name: '文档目录', mimeType: 'directory'),
        McpResource(uri: 'file:///images', name: '图片目录', mimeType: 'directory'),
        McpResource(uri: 'file:///config.json', name: '配置文件', mimeType: 'application/json', content: '{"key": "value"}'),
        McpResource(uri: 'file:///log.txt', name: '日志文件', mimeType: 'text/plain', content: '系统日志...'),
      ],
    ),
    McpServer(
      id: 'mcp2', name: 'Web 搜索 MCP', transport: McpTransport.sse, address: 'https://search.example.com/sse', status: McpStatus.connected,
      toolCount: 3, promptCount: 1, resourceCount: 0, hasSampling: true, hasTasks: false, hasRoots: false, hasOAuth: true,
      tools: [
        McpTool(name: 'web_search', description: '搜索互联网'),
        McpTool(name: 'fetch_page', description: '获取网页内容'),
        McpTool(name: 'extract_content', description: '提取网页正文'),
      ],
      prompts: [
        McpPrompt(name: 'search_and_summarize', description: '搜索并总结', content: '搜索 {{query}} 并总结结果'),
      ],
      resources: [],
    ),
    McpServer(
      id: 'mcp3', name: 'GitHub MCP', transport: McpTransport.websocket, address: 'wss://github.example.com/ws', status: McpStatus.disconnected,
      toolCount: 12, promptCount: 3, resourceCount: 8, hasSampling: false, hasTasks: true, hasRoots: false, hasOAuth: true,
      tools: [
        McpTool(name: 'create_issue', description: '创建 Issue'),
        McpTool(name: 'list_prs', description: '列出 Pull Request'),
        McpTool(name: 'merge_pr', description: '合并 Pull Request'),
      ],
      prompts: [
        McpPrompt(name: 'review_pr', description: '审查 PR', content: '请审查 PR #{{pr_number}}'),
      ],
      resources: [],
    ),
  ];

  static List<AgentSkill> agentSkills = [
    AgentSkill(id: 'as1', name: '代码审查', description: '自动审查代码变更', skillMd: '# 代码审查\n\n审查代码质量和安全性', requiredMcp: ['GitHub MCP'], compatibility: '完全兼容', isEnabled: true, version: '1.2.0'),
    AgentSkill(id: 'as2', name: '文档生成', description: '根据代码生成文档', skillMd: '# 文档生成\n\n自动生成 API 文档', requiredMcp: ['文件系统 MCP'], compatibility: '兼容', isEnabled: true, version: '1.0.0'),
    AgentSkill(id: 'as3', name: '数据分析', description: '分析数据并生成报告', skillMd: '# 数据分析\n\n分析 CSV/JSON 数据', requiredMcp: ['Web 搜索 MCP'], compatibility: '部分兼容', isEnabled: false, version: '0.9.0'),
  ];

  static List<SystemPlugin> systemPlugins = [
    SystemPlugin(id: 'sp1', name: '消息处理插件', description: '处理消息的接收和发送', runtimeStatus: '运行中', hooks: ['message.receive', 'message.send'], events: ['message.queued', 'message.delivered'], schedules: ['cleanup.messages'], registeredSkills: ['消息搜索'], version: '2.1.0'),
    SystemPlugin(id: 'sp2', name: '记忆管理插件', description: '管理记忆的存储和检索', runtimeStatus: '运行中', hooks: ['memory.store', 'memory.retrieve'], events: ['memory.created', 'memory.updated'], schedules: ['consolidate.memories'], registeredSkills: ['记忆搜索'], version: '1.5.0'),
    SystemPlugin(id: 'sp3', name: '情感引擎插件', description: '处理情感分析和状态', runtimeStatus: '已暂停', hooks: ['emotion.update'], events: ['emotion.changed'], schedules: [], registeredSkills: [], isEnabled: false, version: '1.0.0'),
  ];

  static List<CompatibleSkill> compatibleSkills = [
    CompatibleSkill(id: 'cs1', name: '文件搜索', description: '搜索本地文件', version: '1.1.0', previousVersion: '1.0.0', isEnabled: true, lastTestResult: '通过'),
    CompatibleSkill(id: 'cs2', name: '网页摘要', description: '自动提取网页核心内容', version: '2.0.0', previousVersion: '1.5.0', isEnabled: true, lastTestResult: '通过'),
    CompatibleSkill(id: 'cs3', name: '日程管理', description: '管理日程和提醒', version: '1.3.0', previousVersion: '1.2.0', isEnabled: false, lastTestResult: '失败'),
  ];

  static List<ExecutionRun> executionRuns = [
    ExecutionRun(id: 'er1', name: '整理下载目录', status: '运行中', duration: '00:18', input: '整理 ~/Downloads', output: '处理中...', toolCalls: [
      ToolCallEntry(toolName: '文件系统 · 读取目录', input: '~/Downloads', output: '1,247 个文件', duration: '0.8秒', status: '成功'),
      ToolCallEntry(toolName: '去重分析', input: '1,247 个文件', output: '23 个重复', duration: '执行中', status: '运行中'),
    ], startTime: DateTime(2026, 7, 30, 9, 18)),
    ExecutionRun(id: 'er2', name: '生成周报摘要', status: '已完成', duration: '02:15', input: '生成本周工作摘要', output: '已生成包含5个进展的摘要', toolCalls: [
      ToolCallEntry(toolName: '文件系统 · 读取', input: '工作记录', output: '89 个文档', duration: '1.2秒', status: '成功'),
      ToolCallEntry(toolName: 'LLM · 生成', input: '工作记录摘要', output: '5个主要进展', duration: '45秒', status: '成功'),
    ], startTime: DateTime(2026, 7, 29, 14, 0)),
    ExecutionRun(id: 'er3', name: '备份工作文档', status: '失败', duration: '00:05', input: '备份到指定目录', output: '', error: '目标目录没有写入权限', toolCalls: [
      ToolCallEntry(toolName: '文件系统 · 写入', input: '备份目录', output: '权限拒绝', duration: '0.3秒', status: '失败'),
    ], startTime: DateTime(2026, 7, 28, 18, 0)),
  ];
}
