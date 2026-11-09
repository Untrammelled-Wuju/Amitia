import 'package:flutter/material.dart';

import '../../../../app/app_routes.dart';
import '../../../dashboard/presentation/pages/dashboard_page.dart';
import '../../../permissions/presentation/pages/android_automation_page.dart';
import '../../../permissions/presentation/pages/permissions_page.dart';
import '../../../permissions/presentation/pages/virtual_display_settings_page.dart';
import '../../../runtime/presentation/pages/runtime_page.dart';
import '../../../toolbox/presentation/pages/toolbox_database_status_page.dart';
import '../../../toolbox/presentation/pages/toolbox_device_status_page.dart';
import '../../../toolbox/presentation/pages/toolbox_file_browser_page.dart';
import '../../../toolbox/presentation/pages/toolbox_log_page.dart';
import '../../../toolbox/presentation/pages/toolbox_prompt_trace_page.dart';
import '../../../toolbox/presentation/pages/toolbox_runtime_status_page.dart';
import '../../../toolbox/presentation/pages/toolbox_task_log_page.dart';
import '../../../toolbox/presentation/pages/toolbox_workspace_page.dart';
import 'about_page_new.dart';
import 'advanced_system_page.dart';
import 'app_update_page.dart';
import 'appearance_settings_page.dart';
import 'asr_page.dart';
import 'backup_page.dart';
import 'deployment_page.dart';
import 'device_add_page.dart';
import 'device_settings_page.dart';
import 'devices_page.dart';
import 'maintenance_page.dart';
import 'model_config_page.dart';
import 'model_settings_page.dart';
import 'notification_settings_page.dart';
import 'privacy_policy_page.dart';
import 'privacy_scan_page.dart';
import 'runtime_mode_page.dart';
import 'safety_page.dart';
import 'settings_page.dart';
import 'storage_page.dart';
import 'system_settings_page.dart';
import 'temporal_settings_page.dart';
import 'user_agreement_page.dart';
import 'user_feedback_page.dart';
import 'user_settings_page.dart';

class MobileUserSpaceHost extends StatelessWidget {
  const MobileUserSpaceHost({super.key, required this.context});

  final Map<String, dynamic> context;

  String get _path {
    final route = (context['route'] ?? '').toString();
    return Uri.tryParse(route)?.path ?? route;
  }

  String get _rootUri {
    final query = context['queryParameters'];
    if (query is Map) return (query['rootUri'] ?? '').toString();
    return '';
  }

  Widget _view(String path) {
    if (path == AppRoutes.settings || path == AppRoutes.settingsSystem) {
      return const SettingsPage();
    }
    if (path == AppRoutes.settingsOverview) {
      return const DashboardPage(initialTab: 0);
    }
    if (path == AppRoutes.settingsData) {
      return const DashboardPage(initialTab: 1);
    }
    if (path == AppRoutes.settingsModels) {
      return const ModelSettingsPage();
    }
    if (path.startsWith('${AppRoutes.settingsModels}/')) {
      return ModelConfigPage(modelType: path.split('/').last);
    }
    if (path == AppRoutes.settingsAppearance ||
        path == AppRoutes.settingsTheme) {
      return const AppearanceSettingsPage();
    }
    if (path == AppRoutes.settingsNotifications) {
      return const NotificationSettingsPage();
    }
    if (path == AppRoutes.settingsRuntime) {
      return const RuntimePage();
    }
    if (path == AppRoutes.settingsPermissions) {
      return const PermissionsPage();
    }
    if (path == AppRoutes.settingsAndroidAutomation) {
      return const AndroidAutomationPage();
    }
    if (path == AppRoutes.settingsVirtualDisplay) {
      return const VirtualDisplaySettingsPage();
    }
    if (path == AppRoutes.settingsBackup) {
      return const BackupPage();
    }
    if (path == AppRoutes.settingsAsr) {
      return const AsrPage();
    }
    if (path == AppRoutes.settingsDeployment) {
      return const DeploymentPage();
    }
    if (path == AppRoutes.settingsSystem) {
      return const SystemSettingsPage();
    }
    if (path == AppRoutes.settingsTemporal) {
      return const TemporalSettingsPage();
    }
    if (path == AppRoutes.settingsSafety) {
      return const SafetyPage();
    }
    if (path == AppRoutes.settingsMaintenance) {
      return const MaintenancePage();
    }
    if (path == AppRoutes.settingsUserFeedback) {
      return const UserFeedbackPage();
    }
    if (path == AppRoutes.settingsAdvanced) {
      return const AdvancedSystemPage();
    }
    if (path == AppRoutes.settingsStorage) {
      return const StoragePage();
    }
    if (path == AppRoutes.settingsUser) {
      return const UserSettingsPage();
    }
    if (path == AppRoutes.settingsDevices) {
      return const RuntimeModePage();
    }
    if (path == AppRoutes.settingsBoundDevices) {
      return const DevicesPage();
    }
    if (path == AppRoutes.settingsDeviceAdd) {
      return const DeviceAddPage();
    }
    if (path == AppRoutes.settingsDeviceSettings) {
      return const DeviceSettingsPage();
    }
    if (path == AppRoutes.settingsPrivacyScan) {
      return const PrivacyScanPage();
    }
    if (path == AppRoutes.settingsPrivacyPolicy) {
      return const PrivacyPolicyPage();
    }
    if (path == AppRoutes.settingsUserAgreement) {
      return const UserAgreementPage();
    }
    if (path == AppRoutes.settingsAppUpdate) {
      return const AppUpdatePage();
    }
    if (path == AppRoutes.settingsAbout) {
      return const AboutPageNew();
    }
    if (path == AppRoutes.settingsToolbox ||
        path == AppRoutes.settingsMaintenanceCategory) {
      return const MaintenancePage();
    }
    if (path == AppRoutes.toolboxFileBrowser) {
      return ToolboxFileBrowserPage(initialRootUri: _rootUri);
    }
    if (path == AppRoutes.toolboxWorkspace) {
      return const ToolboxWorkspacePage();
    }
    if (path == AppRoutes.toolboxTaskLog) {
      return const ToolboxTaskLogPage();
    }
    if (path == AppRoutes.toolboxLog) {
      return const ToolboxLogPage();
    }
    if (path == AppRoutes.toolboxPromptTrace) {
      return const ToolboxPromptTracePage();
    }
    if (path == AppRoutes.toolboxRuntimeStatus) {
      return const ToolboxRuntimeStatusPage();
    }
    if (path == AppRoutes.toolboxDatabaseStatus) {
      return const ToolboxDatabaseStatusPage();
    }
    if (path == AppRoutes.toolboxDeviceStatus) {
      return const ToolboxDeviceStatusPage();
    }
    return const SettingsPage();
  }

  @override
  Widget build(BuildContext context) => _view(_path);
}
