import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AndroidAutomationPage extends ConsumerStatefulWidget {
  const AndroidAutomationPage({super.key});

  @override
  ConsumerState<AndroidAutomationPage> createState() =>
      _AndroidAutomationPageState();
}

class _AndroidAutomationPageState
    extends ConsumerState<AndroidAutomationPage> {
  bool _loading = true;
  String? _error;
  String _providerHealth = 'unknown';
  DateTime? _probedAt;
  List<_AutomationCapability> _capabilities = const [];

  @override
  void initState() {
    super.initState();
    Future<void>.microtask(_refresh);
  }

  Future<void> _refresh() async {
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }

    try {
      final api = ref.read(rawDeviceLocalBackendServiceApiProvider);
      Map<String, dynamic> backend = const <String, dynamic>{};
      if (api != null) {
        backend = await api.get<Map<String, dynamic>>(
              '/api/android-automation/status',
            ) ??
            const <String, dynamic>{};
      }

      final screenCapture = await _nativeStatus('screen_capture.status');
      final capabilities = _buildCapabilities(backend, screenCapture);
      if (!mounted) return;
      setState(() {
        _providerHealth = backend['providerHealth']?.toString() ??
            (backend.isEmpty ? 'unavailable' : 'unknown');
        _probedAt = DateTime.tryParse(backend['probedAt']?.toString() ?? '') ??
            DateTime.now();
        _capabilities = capabilities;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = error.toString();
      });
    }
  }

  Future<Map<String, dynamic>> _nativeStatus(String operation) async {
    try {
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId':
            'automation_${DateTime.now().microsecondsSinceEpoch}_$operation',
        'platform': 'android',
        'operation': operation,
        'payload': const <String, dynamic>{},
      });
      return response;
    } catch (error) {
      return <String, dynamic>{
        'status': 'error',
        'error': <String, dynamic>{
          'code': 'NATIVE_STATUS_FAILED',
          'message': error.toString(),
        },
      };
    }
  }

  List<_AutomationCapability> _buildCapabilities(
    Map<String, dynamic> backend,
    Map<String, dynamic> screenCapture,
  ) {
    final probes = _asMap(backend['probes']);
    final interaction = _probeResult(probes, 'interaction.status');
    final providers = _asMap(interaction['providers']);

    return <_AutomationCapability>[
      _fromProvider(
        name: 'Accessibility',
        icon: Icons.accessibility_new,
        provider: _asMap(providers['accessibility']),
        description: '语义节点、窗口树与原生节点动作',
        repairOperation: 'accessibility.open_settings',
      ),
      _fromProvider(
        name: 'Shizuku',
        icon: Icons.security_outlined,
        provider: _asMap(providers['shizuku']),
        description: '授权后的高权限系统执行通道',
        repairOperation: 'shizuku.request_permission',
      ),
      _fromProvider(
        name: 'ADB',
        icon: Icons.terminal_outlined,
        provider: _asMap(providers['adb']),
        description: 'ADB / Wireless ADB 自动化 fallback',
      ),
      _fromProvider(
        name: 'Root',
        icon: Icons.admin_panel_settings_outlined,
        provider: _asMap(providers['root']),
        description: 'Root shell 与 UIAutomator fallback',
        repairOperation: 'root.request',
      ),
      _fromRawProbe(
        name: 'Screen Capture',
        icon: Icons.screenshot_monitor_outlined,
        probe: screenCapture,
        description: 'Display-aware 截图，为 OCR / Vision 提供输入',
      ),
      _fromProvider(
        name: 'OCR',
        icon: Icons.document_scanner_outlined,
        provider: _asMap(providers['ocr']),
        description: '文本识别、置信度与坐标定位',
      ),
      _fromProvider(
        name: 'Vision',
        icon: Icons.visibility_outlined,
        provider: _asMap(providers['vision']),
        description: '自然语言目标的视觉元素定位',
      ),
      _fromRawProbe(
        name: 'Notification',
        icon: Icons.notifications_active_outlined,
        probe: _probeEnvelope(probes, 'notification.status'),
        description: '通知读取、动作执行与通知事件',
      ),
      _fromRawProbe(
        name: 'Overlay',
        icon: Icons.picture_in_picture_alt_outlined,
        probe: _probeEnvelope(probes, 'system.overlay.status'),
        description: '悬浮窗与跨应用交互界面能力',
        repairOperation: 'system.overlay.permission.request',
      ),
      _fromRawProbe(
        name: 'Virtual Display',
        icon: Icons.splitscreen_outlined,
        probe: _probeEnvelope(probes, 'virtual_display.status'),
        description: '真实系统 VirtualDisplay 生命周期与多 Display',
      ),
    ];
  }

  _AutomationCapability _fromProvider({
    required String name,
    required IconData icon,
    required Map<String, dynamic> provider,
    required String description,
    String? repairOperation,
  }) {
    if (provider.isEmpty) {
      return _AutomationCapability(
        name: name,
        icon: icon,
        state: 'UNAVAILABLE',
        description: description,
        reason: '设备 Runtime 尚未返回该 Provider 的健康状态',
        repairOperation: repairOperation,
      );
    }
    return _AutomationCapability(
      name: name,
      icon: icon,
      state: _normalizeState(provider['state']?.toString()),
      description: description,
      provider: provider['provider']?.toString(),
      permission: provider['permission']?.toString(),
      reason: provider['reason']?.toString(),
      lastProbeAt: DateTime.tryParse(provider['lastProbeAt']?.toString() ?? ''),
      lastSuccessAt:
          DateTime.tryParse(provider['lastSuccessAt']?.toString() ?? ''),
      recoverable: provider['recoverable'] == true,
      repairOperation: repairOperation,
    );
  }

  _AutomationCapability _fromRawProbe({
    required String name,
    required IconData icon,
    required Map<String, dynamic> probe,
    required String description,
    String? repairOperation,
  }) {
    final status = probe['status']?.toString().toLowerCase();
    final result = _asMap(probe['result']);
    final error = _asMap(probe['error']);
    final rawState = result['state']?.toString();
    String state;
    if (status == 'error') {
      state = 'FAILED';
    } else {
      state = _normalizeRawState(rawState, result);
    }
    final reason = result['reason']?.toString().trim();
    final errorMessage = error['message']?.toString().trim();
    return _AutomationCapability(
      name: name,
      icon: icon,
      state: state,
      description: description,
      provider: result['provider']?.toString(),
      permission: result['permissionState']?.toString() ??
          result['authorizationState']?.toString(),
      reason: reason?.isNotEmpty == true
          ? reason
          : (errorMessage?.isNotEmpty == true ? errorMessage : null),
      recoverable: state != 'UNAVAILABLE',
      repairOperation: repairOperation,
    );
  }

  String _normalizeState(String? state) {
    final value = state?.trim().toUpperCase() ?? '';
    const known = <String>{
      'SUPPORTED',
      'UNAVAILABLE',
      'PERMISSION_REQUIRED',
      'STARTING',
      'READY',
      'DEGRADED',
      'FAILED',
    };
    return known.contains(value) ? value : 'UNAVAILABLE';
  }

  String _normalizeRawState(
    String? rawState,
    Map<String, dynamic> result,
  ) {
    final value = rawState?.trim().toLowerCase() ?? '';
    if (result['userActionRequired'] == true ||
        value.contains('permission') ||
        value.contains('authorization_required')) {
      return 'PERMISSION_REQUIRED';
    }
    if (result['supported'] == false || value.contains('unsupported')) {
      return 'UNAVAILABLE';
    }
    if (result['available'] == true ||
        result['connected'] == true ||
        result['listenerConnected'] == true ||
        result['permissionGranted'] == true ||
        value == 'ready' ||
        value == 'available' ||
        value == 'connected' ||
        value == 'authorized' ||
        value == 'granted') {
      return 'READY';
    }
    if (value.contains('starting') ||
        value.contains('binding') ||
        value.contains('pending')) {
      return 'STARTING';
    }
    if (value.contains('degraded') || value.contains('offline')) {
      return 'DEGRADED';
    }
    if (result.isNotEmpty) return 'SUPPORTED';
    return 'UNAVAILABLE';
  }

  Map<String, dynamic> _probeResult(
    Map<String, dynamic> probes,
    String operation,
  ) =>
      _asMap(_asMap(probes[operation])['result']);

  Map<String, dynamic> _probeEnvelope(
    Map<String, dynamic> probes,
    String operation,
  ) =>
      _asMap(probes[operation]);

  Map<String, dynamic> _asMap(dynamic value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    return <String, dynamic>{};
  }

  BadgeType _badgeType(String state) {
    switch (state) {
      case 'READY':
        return BadgeType.success;
      case 'SUPPORTED':
      case 'STARTING':
      case 'DEGRADED':
      case 'PERMISSION_REQUIRED':
        return BadgeType.warning;
      case 'FAILED':
      case 'UNAVAILABLE':
      default:
        return BadgeType.error;
    }
  }

  String _displayState(String state) {
    switch (state) {
      case 'READY':
        return 'Ready';
      case 'SUPPORTED':
        return 'Supported';
      case 'PERMISSION_REQUIRED':
        return '需授权';
      case 'STARTING':
        return 'Starting';
      case 'DEGRADED':
        return 'Degraded';
      case 'FAILED':
        return 'Failed';
      default:
        return 'Unavailable';
    }
  }

  Future<void> _repair(_AutomationCapability capability) async {
    final operation = capability.repairOperation;
    if (operation == null || operation.isEmpty) {
      if (mounted) context.push(AppRoutes.settingsPermissions);
      return;
    }
    final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
    try {
      final result = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId': 'automation_repair_${DateTime.now().microsecondsSinceEpoch}',
        'platform': 'android',
        'operation': operation,
        'payload': const <String, dynamic>{},
      });
      if (result['status']?.toString() != 'success') {
        final error = _asMap(result['error']);
        throw StateError(
          error['message']?.toString() ?? '修复操作未成功执行',
        );
      }
      await Future<void>.delayed(const Duration(milliseconds: 300));
      await _refresh();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('操作失败：$error')),
      );
    }
  }

  void _showDetails(_AutomationCapability capability) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (context) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.lg,
            AppSpacing.lg,
            AppSpacing.xl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      capability.name,
                      style: AppTypography.sectionTitle(context),
                    ),
                  ),
                  AmitiaStatusBadge(
                    label: _displayState(capability.state),
                    type: _badgeType(capability.state),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              _DetailRow(label: '状态', value: capability.state),
              _DetailRow(
                label: 'Provider',
                value: capability.provider ?? '未报告',
              ),
              _DetailRow(
                label: '权限',
                value: capability.permission ?? '无额外状态',
              ),
              if (capability.lastProbeAt != null)
                _DetailRow(
                  label: '最近探测',
                  value: capability.lastProbeAt!.toLocal().toString(),
                ),
              if (capability.lastSuccessAt != null)
                _DetailRow(
                  label: '最近成功',
                  value: capability.lastSuccessAt!.toLocal().toString(),
                ),
              _DetailRow(
                label: '可恢复',
                value: capability.recoverable ? '是' : '否',
              ),
              if (capability.reason?.trim().isNotEmpty == true) ...[
                SizedBox(height: AppSpacing.sm),
                Text('原因', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                Text(
                  capability.reason!,
                  style: AppTypography.bodySmall(context),
                ),
              ],
              SizedBox(height: AppSpacing.lg),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: () async {
                    Navigator.of(context).pop();
                    await _repair(capability);
                  },
                  icon: const Icon(Icons.build_outlined),
                  label: Text(
                    capability.repairOperation == null ? '前往系统权限' : '修复 / 授权',
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Android Automation',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [
          IconButton(
            tooltip: '刷新状态',
            onPressed: _loading ? null : _refresh,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _refresh,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.xl,
          ),
          children: [
            _AutomationSummaryCard(
              loading: _loading,
              providerHealth: _providerHealth,
              probedAt: _probedAt,
              capabilities: _capabilities,
            ),
            SizedBox(height: AppSpacing.md),
            if (_error != null)
              Container(
                margin: EdgeInsets.only(bottom: AppSpacing.md),
                padding: EdgeInsets.all(AppSpacing.md),
                decoration: BoxDecoration(
                  color: context.error.withValues(alpha: 0.08),
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(
                    color: context.error.withValues(alpha: 0.25),
                  ),
                ),
                child: Text(
                  '状态探测失败：$_error',
                  style: AppTypography.bodySmall(context),
                ),
              ),
            if (_loading && _capabilities.isEmpty)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 64),
                child: Center(child: CircularProgressIndicator()),
              )
            else
              ..._capabilities.map(
                (capability) => Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.sm),
                  child: _AutomationCapabilityCard(
                    capability: capability,
                    badgeType: _badgeType(capability.state),
                    stateLabel: _displayState(capability.state),
                    onTap: () => _showDetails(capability),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _AutomationCapability {
  final String name;
  final IconData icon;
  final String state;
  final String description;
  final String? provider;
  final String? permission;
  final String? reason;
  final DateTime? lastProbeAt;
  final DateTime? lastSuccessAt;
  final bool recoverable;
  final String? repairOperation;

  const _AutomationCapability({
    required this.name,
    required this.icon,
    required this.state,
    required this.description,
    this.provider,
    this.permission,
    this.reason,
    this.lastProbeAt,
    this.lastSuccessAt,
    this.recoverable = false,
    this.repairOperation,
  });
}

class _AutomationSummaryCard extends StatelessWidget {
  final bool loading;
  final String providerHealth;
  final DateTime? probedAt;
  final List<_AutomationCapability> capabilities;

  const _AutomationSummaryCard({
    required this.loading,
    required this.providerHealth,
    required this.probedAt,
    required this.capabilities,
  });

  @override
  Widget build(BuildContext context) {
    final ready = capabilities.where((item) => item.state == 'READY').length;
    final attention = capabilities
        .where((item) => item.state != 'READY' && item.state != 'SUPPORTED')
        .length;
    return Container(
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.hub_outlined, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  '设备自动化能力',
                  style: AppTypography.cardTitle(context),
                ),
              ),
              if (loading)
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            'Provider Health: $providerHealth · Ready $ready/${capabilities.length}'
            '${attention > 0 ? ' · $attention 项需处理' : ''}',
            style: AppTypography.bodySmall(context),
          ),
          if (probedAt != null) ...[
            const SizedBox(height: 3),
            Text(
              '最近探测：${probedAt!.toLocal()}',
              style: AppTypography.caption(context),
            ),
          ],
        ],
      ),
    );
  }
}

class _AutomationCapabilityCard extends StatelessWidget {
  final _AutomationCapability capability;
  final BadgeType badgeType;
  final String stateLabel;
  final VoidCallback onTap;

  const _AutomationCapabilityCard({
    required this.capability,
    required this.badgeType,
    required this.stateLabel,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: AppRadius.brMedium,
      child: Container(
        padding: EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                capability.icon,
                size: 20,
                color: context.accentPrimary,
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    capability.name,
                    style: AppTypography.cardTitle(context),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    capability.reason?.trim().isNotEmpty == true
                        ? capability.reason!
                        : capability.description,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.label(context),
                  ),
                ],
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            AmitiaStatusBadge(label: stateLabel, type: badgeType),
          ],
        ),
      ),
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 84,
            child: Text(label, style: AppTypography.label(context)),
          ),
          Expanded(
            child: Text(value, style: AppTypography.bodySmall(context)),
          ),
        ],
      ),
    );
  }
}
