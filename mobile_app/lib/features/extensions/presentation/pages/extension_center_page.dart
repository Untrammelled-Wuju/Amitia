import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/extension_service.dart';
import '../../../../core/services/providers.dart';

final installedExtensionViewProvider = FutureProvider.autoDispose<ExtensionCenterView>((ref) async {
  final svc = ref.read(extensionServiceProvider);
  return await svc.getExtensionCenterView();
});

class ExtensionCenterPage extends ConsumerWidget {
  const ExtensionCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final viewAsync = ref.watch(installedExtensionViewProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('扩展中心'),
      ),
      body: viewAsync.when(
        data: (view) => _buildContent(context, ref, view),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(child: Text('加载失败: $err')),
      ),
    );
  }

  Widget _buildContent(BuildContext context, WidgetRef ref, ExtensionCenterView view) {
    if (view.all.isEmpty) {
      return const Center(child: Text('暂无扩展'));
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        if (view.installed.isNotEmpty) ...[
          Text('已安装', style: AppTypography.pageTitle(context).copyWith(fontSize: 16)),
          const SizedBox(height: 8),
          ...view.installed.map((card) => AmitiaExtensionCard(
                name: card.displayName,
                description: card.description,
                icon: Icons.extension_outlined,
                typeLabel: card.status,
                isInstalled: true,
                isEnabled: card.enabled,
                isRecommended: false,
                onToggle: (enabled) async {
                  final svc = ref.read(extensionServiceProvider);
                  await svc.setKernelExtensionEnabled(card.extensionId, enabled);
                  ref.invalidate(installedExtensionViewProvider);
                },
              )),
        ],
        if (view.discover.isNotEmpty) ...[
          const SizedBox(height: 16),
          Text('发现', style: AppTypography.pageTitle(context).copyWith(fontSize: 16)),
          const SizedBox(height: 8),
          ...view.discover.map((card) => AmitiaExtensionCard(
                name: card.displayName,
                description: card.description,
                icon: Icons.extension_outlined,
                typeLabel: card.status,
                isInstalled: false,
                isEnabled: false,
                isRecommended: card.contributionTags.contains('recommended'),
                onAction: () async {
                  final svc = ref.read(extensionServiceProvider);
                  await svc.installKernelExtensionUpdate(card.extensionId, 'latest');
                  ref.invalidate(installedExtensionViewProvider);
                },
              )),
        ],
      ],
    );
  }
}

class AmitiaExtensionCard extends StatelessWidget {
  final String name;
  final String description;
  final IconData icon;
  final String typeLabel;
  final bool isInstalled;
  final bool isEnabled;
  final bool isRecommended;
  final VoidCallback? onAction;
  final ValueChanged<bool>? onToggle;

  const AmitiaExtensionCard({
    super.key,
    required this.name,
    required this.description,
    required this.icon,
    required this.typeLabel,
    required this.isInstalled,
    required this.isEnabled,
    this.isRecommended = false,
    this.onAction,
    this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(icon, size: 22, color: context.accentPrimary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: context.borderSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(
                        typeLabel,
                        style: TextStyle(
                          fontSize: 10,
                          color: context.textTertiary,
                        ),
                      ),
                    ),
                    if (isRecommended) ...[
                      const SizedBox(width: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text(
                          '推荐',
                          style: TextStyle(
                            fontSize: 10,
                            color: context.accentPrimary,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  description,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isInstalled)
            SizedBox(
              width: 44,
              child: Switch(
                value: isEnabled,
                onChanged: onToggle,
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
            )
          else
            GestureDetector(
              onTap: onAction,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(
                  '安装',
                  style: TextStyle(
                    fontSize: 13,
                    color: Colors.white,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
