import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/ui_runtime/channel_plugin_navigation.dart';
import '../../../../core/ui_runtime/ui_provider.dart';
import '../../../../core/ui_runtime/ui_provider_host.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/ui_runtime/ui_workspace_tabs.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import 'channel_messages_page.dart';
import 'channel_plugin_settings_view.dart';

class ChannelPluginWorkspacePage extends ConsumerStatefulWidget {
  final String providerId;
  final String capability;
  final String route;
  final Map<String, dynamic> context;

  const ChannelPluginWorkspacePage({
    super.key,
    required this.providerId,
    required this.capability,
    required this.route,
    required this.context,
  });

  @override
  ConsumerState<ChannelPluginWorkspacePage> createState() =>
      _ChannelPluginWorkspacePageState();
}

class _ChannelPluginWorkspacePageState
    extends ConsumerState<ChannelPluginWorkspacePage> {
  String _selectedTabId = 'plugin';
  final Set<String> _visitedTabIds = <String>{'plugin'};

  void _selectTab(String tabId) {
    setState(() {
      _selectedTabId = tabId;
      _visitedTabIds.add(tabId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final channel = ChannelPluginNavigationRegistry.findByRoute(
      snapshot,
      widget.route,
    );
    final workspaceContext = <String, dynamic>{
      ...widget.context,
      'channelId': channel?.channelId ?? '',
      'channelExtensionId': channel?.extensionId ?? '',
      'workspace': 'channel.workspace',
    };
    final workspaceTabs = channel == null
        ? const <UIWorkspaceTabDefinition>[]
        : UIWorkspaceTabRegistry.resolve(
            snapshot,
            workspace: 'channel.workspace',
            context: workspaceContext,
            route: widget.route,
            reservedTabIds: const <String>{'plugin', 'messages', 'settings'},
          );
    final tabIds = <String>[
      'plugin',
      'messages',
      'settings',
      ...workspaceTabs.map((tab) => tab.tabId),
    ];
    final selectedTabId = tabIds.contains(_selectedTabId)
        ? _selectedTabId
        : 'plugin';
    _selectedTabId = selectedTabId;
    _visitedTabIds.add(selectedTabId);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: channel?.label ?? '渠道插件',
        showBackButton: true,
      ),
      body: channel == null
          ? AmitiaEmptyState(
              icon: Icons.extension_off_outlined,
              title: '渠道插件不可用',
              subtitle: '插件已停用、卸载或未完成手机端页面注册。',
            )
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.pagePadding,
                  ),
                  child: Row(
                    children: [
                      _ChannelWorkspaceTab(
                        label: '插件页面',
                        selected: selectedTabId == 'plugin',
                        onTap: () => _selectTab('plugin'),
                      ),
                      _ChannelWorkspaceTab(
                        label: '消息统计',
                        selected: selectedTabId == 'messages',
                        onTap: () => _selectTab('messages'),
                      ),
                      _ChannelWorkspaceTab(
                        label: '设置',
                        selected: selectedTabId == 'settings',
                        onTap: () => _selectTab('settings'),
                      ),
                      for (final tab in workspaceTabs)
                        _ChannelWorkspaceTab(
                          label: tab.label,
                          selected: selectedTabId == tab.tabId,
                          onTap: () => _selectTab(tab.tabId),
                        ),
                    ],
                  ),
                ),
                Divider(
                  height: 0.5,
                  thickness: 0.5,
                  color: context.borderPrimary,
                ),
                Expanded(
                  child: IndexedStack(
                    index: tabIds.indexOf(selectedTabId),
                    children: [
                      _ChannelTabSlot(
                        visible: _isTabVisible('plugin'),
                        child: UIProviderHost(
                          capability: widget.capability,
                          providerId: widget.providerId,
                          context: {
                            ...workspaceContext,
                            'surface': 'channel.plugin',
                          },
                          fallback: const AmitiaEmptyState(
                            icon: Icons.extension_off_outlined,
                            title: '插件页面暂不可用',
                          ),
                        ),
                      ),
                      _ChannelTabSlot(
                        visible: _isTabVisible('messages'),
                        child: ChannelHostMessagesView(
                          channelId: channel.channelId,
                          showSummary: true,
                        ),
                      ),
                      _ChannelTabSlot(
                        visible: _isTabVisible('settings'),
                        child: ChannelPluginSettingsView(
                          channelId: channel.channelId,
                          title: '${channel.label}设置',
                        ),
                      ),
                      for (final tab in workspaceTabs)
                        _ChannelTabSlot(
                          visible:
                              selectedTabId == tab.tabId ||
                              tab.keepMounted &&
                                  _visitedTabIds.contains(tab.tabId),
                          child: UIProviderHost(
                            capability: UICapability.workspaceTab,
                            providerId: tab.providerId,
                            context: {
                              ...workspaceContext,
                              'workspaceTabId': tab.tabId,
                            },
                            fallback: const AmitiaEmptyState(
                              icon: Icons.widgets_outlined,
                              title: '此插件 Tab 暂不可用',
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
              ],
            ),
    );
  }

  bool _isTabVisible(String tabId) =>
      _selectedTabId == tabId || _visitedTabIds.contains(tabId);
}

class _ChannelWorkspaceTab extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _ChannelWorkspaceTab({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 9),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(
                color: selected ? context.accentPrimary : Colors.transparent,
                width: 2,
              ),
            ),
          ),
          child: Text(
            label,
            style: AppTypography.label(context).copyWith(
              color: selected ? context.textPrimary : context.textSecondary,
              fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
            ),
          ),
        ),
      ),
    );
  }
}

class _ChannelTabSlot extends StatelessWidget {
  final bool visible;
  final Widget child;

  const _ChannelTabSlot({required this.visible, required this.child});

  @override
  Widget build(BuildContext context) {
    return visible ? child : const SizedBox.shrink();
  }
}
