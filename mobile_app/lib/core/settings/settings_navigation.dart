import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import '../../app/app_routes.dart';
import '../../shared/models/models.dart';

class SettingsCategory {
  const SettingsCategory({
    required this.id,
    required this.icon,
    required this.group,
  });
  final String id;
  final IconData icon;
  final SettingGroup group;
}

List<SettingsCategory> buildSettingsCategories({
  required String modelSummary,
  required String appearanceSummary,
  bool isDeveloperMode = false,
}) => <SettingsCategory>[
  SettingsCategory(
    id: 'ai',
    icon: Icons.psychology_outlined,
    group: SettingGroup(
      title: 'AI 与对话',
      items: [
        SettingItem(
          title: '模型设置',
          icon: Icons.psychology_outlined,
          value: modelSummary,
          route: AppRoutes.settingsModels,
        ),
        SettingItem(
          title: '搜索 API',
          icon: Icons.manage_search_outlined,
          route: AppRoutes.settingsSearchApi,
        ),
      ],
    ),
  ),
  SettingsCategory(
    id: 'appearance',
    icon: Icons.palette_outlined,
    group: SettingGroup(
      title: '使用偏好',
      items: [
        SettingItem(
          title: '通用',
          icon: Icons.settings_outlined,
          route: AppRoutes.settingsSystem,
        ),
        SettingItem(
          title: '外观设置',
          icon: Icons.palette_outlined,
          value: appearanceSummary,
          route: AppRoutes.settingsAppearance,
        ),
        SettingItem(
          title: '通知',
          icon: Icons.notifications_outlined,
          route: AppRoutes.settingsNotifications,
        ),
        SettingItem(
          title: '时间与地区',
          icon: Icons.schedule_outlined,
          route: AppRoutes.settingsTemporal,
        ),
      ],
    ),
  ),
  SettingsCategory(
    id: 'privacy',
    icon: Icons.shield_outlined,
    group: SettingGroup(
      title: '隐私',
      items: [
        SettingItem(
          title: '存储管理',
          icon: Icons.storage_outlined,
          route: AppRoutes.settingsStorage,
        ),
        SettingItem(
          title: '归档对话',
          icon: Icons.archive_outlined,
          route: AppRoutes.chatLogs,
        ),
        SettingItem(
          title: '备份与恢复',
          icon: Icons.backup_outlined,
          route: AppRoutes.settingsBackup,
        ),
        SettingItem(
          title: '安全设置',
          icon: Icons.security_outlined,
          route: AppRoutes.settingsSafety,
        ),
        SettingItem(
          title: '隐私说明',
          icon: Icons.policy_outlined,
          route: AppRoutes.settingsPrivacyPolicy,
        ),
        SettingItem(
          title: '使用边界',
          icon: Icons.description_outlined,
          route: AppRoutes.settingsUserAgreement,
        ),
        SettingItem(
          title: '隐私扫描',
          icon: Icons.privacy_tip_outlined,
          route: AppRoutes.settingsPrivacyScan,
        ),
      ],
    ),
  ),
  SettingsCategory(
    id: 'devices',
    icon: Icons.devices_outlined,
    group: SettingGroup(
      title: '设备与运行',
      items: [
        SettingItem(
          title: '我的设备',
          icon: Icons.devices_outlined,
          route: AppRoutes.settingsDevices,
        ),
        SettingItem(
          title: '部署配置',
          icon: Icons.cloud_upload_outlined,
          route: AppRoutes.settingsDeployment,
        ),
        SettingItem(
          title: '系统权限',
          icon: Icons.lock_outlined,
          route: AppRoutes.settingsPermissions,
        ),
        if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android)
          SettingItem(
            title: 'Android 自动化',
            icon: Icons.smartphone_outlined,
            route: AppRoutes.settingsAndroidAutomation,
          ),
        SettingItem(
          title: '运行概览',
          icon: Icons.monitor_heart_outlined,
          route: AppRoutes.settingsOverview,
        ),
        SettingItem(
          title: '运行数据',
          icon: Icons.insights_outlined,
          route: AppRoutes.settingsData,
        ),
        SettingItem(
          title: '运行环境',
          icon: Icons.terminal,
          route: AppRoutes.settingsRuntime,
        ),
      ],
    ),
  ),
  SettingsCategory(
    id: 'maintenance',
    icon: Icons.build_outlined,
    group: SettingGroup(
      title: '维护与高级工具',
      items: [
        SettingItem(
          title: '界面提供者',
          icon: Icons.dashboard_customize_outlined,
          route: AppRoutes.settingsUIProviders,
        ),
        SettingItem(
          title: 'Core 运行模式',
          icon: Icons.hub_outlined,
          route: AppRoutes.settingsRuntimeMode,
        ),
        SettingItem(
          title: '长期运行维护',
          icon: Icons.schedule_send_outlined,
          route: AppRoutes.settingsLongRunning,
        ),
        SettingItem(
          title: '维护诊断',
          icon: Icons.build_circle_outlined,
          route: AppRoutes.settingsMaintenance,
        ),
        SettingItem(
          title: '文件浏览',
          icon: Icons.folder_outlined,
          route: AppRoutes.toolboxFileBrowser,
        ),
        SettingItem(
          title: '工作区',
          icon: Icons.work_outline,
          route: AppRoutes.toolboxWorkspace,
        ),
        SettingItem(
          title: '任务日志',
          icon: Icons.task_alt,
          route: AppRoutes.toolboxTaskLog,
        ),
        SettingItem(
          title: '运行日志',
          icon: Icons.terminal,
          route: AppRoutes.toolboxLog,
        ),
        SettingItem(
          title: 'Prompt Trace',
          icon: Icons.code,
          route: AppRoutes.toolboxPromptTrace,
        ),
        SettingItem(
          title: 'Runtime 状态',
          icon: Icons.memory,
          route: AppRoutes.toolboxRuntimeStatus,
        ),
        SettingItem(
          title: '数据库状态',
          icon: Icons.storage,
          route: AppRoutes.toolboxDatabaseStatus,
        ),
        SettingItem(
          title: '设备状态',
          icon: Icons.devices,
          route: AppRoutes.toolboxDeviceStatus,
        ),
        SettingItem(
          title: '高级系统',
          icon: Icons.admin_panel_settings_outlined,
          route: AppRoutes.settingsAdvanced,
        ),
        SettingItem(
          title: 'BDI 决策可视化',
          icon: Icons.account_tree_outlined,
          route: AppRoutes.settingsDecisionViz,
        ),
        SettingItem(
          title: '开发者模式',
          icon: Icons.developer_mode_outlined,
          route: AppRoutes.kernelPage('dev-mode'),
        ),
        if (isDeveloperMode)
          SettingItem(
            title: '开发者选项',
            icon: Icons.developer_mode,
            route: AppRoutes.developer,
          ),
      ],
    ),
  ),
  SettingsCategory(
    id: 'about',
    icon: Icons.info_outline,
    group: SettingGroup(
      title: '关于',
      items: [
        SettingItem(
          title: '版本与更新',
          icon: Icons.system_update_outlined,
          route: AppRoutes.settingsAppUpdate,
        ),
        SettingItem(
          title: '关于 Amitia',
          icon: Icons.info_outline,
          route: AppRoutes.settingsAbout,
        ),
      ],
    ),
  ),
];
