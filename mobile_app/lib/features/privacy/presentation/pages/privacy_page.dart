import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../widgets/privacy_sections.dart';

class PrivacyPage extends ConsumerStatefulWidget {
  const PrivacyPage({super.key});

  @override
  ConsumerState<PrivacyPage> createState() => _PrivacyPageState();
}

class _PrivacyPageState extends ConsumerState<PrivacyPage> {
  final _scrollController = ScrollController();
  bool _hasScrolledToBottom = false;
  bool _agreed = false;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(() {
      final maxScroll = _scrollController.position.maxScrollExtent;
      final currentScroll = _scrollController.position.pixels;
      if (maxScroll - currentScroll <= 50 && !_hasScrolledToBottom) {
        setState(() => _hasScrolledToBottom = true);
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _agree() {
    if (!_agreed) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: const Text('请先勾选同意条款'),
          duration: const Duration(seconds: 1),
          backgroundColor: context.warning,
        ),
      );
      return;
    }
    context.go(AppRoutes.chat);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '隐私与使用边界', showBackButton: true),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              controller: _scrollController,
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                for (var index = 0; index < privacySections.length; index++) ...[
                  if (index > 0) SizedBox(height: AppSpacing.lg),
                  PrivacySectionCard(data: privacySections[index]),
                ],
                SizedBox(height: AppSpacing.lg),
                if (!_hasScrolledToBottom)
                  Center(
                    child: Padding(
                      padding: EdgeInsets.all(AppSpacing.md),
                      child: Column(
                        children: [
                          Icon(
                            Icons.arrow_downward,
                            size: 20,
                            color: context.textTertiary,
                          ),
                          SizedBox(height: AppSpacing.xs),
                          Text(
                            '向下滚动查看完整条款',
                            style: AppTypography.label(context),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
          Container(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              border: Border(
                top: BorderSide(color: context.borderSecondary, width: 1),
              ),
            ),
            child: SafeArea(
              top: false,
              child: Column(
                children: [
                  GestureDetector(
                    onTap: () => setState(() => _agreed = !_agreed),
                    child: Row(
                      children: [
                        Icon(
                          _agreed
                              ? Icons.check_box
                              : Icons.check_box_outline_blank,
                          size: 22,
                          color: _agreed
                              ? context.accentPrimary
                              : context.textTertiary,
                        ),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Text(
                            '我已阅读并同意以上隐私条款与使用边界',
                            style: AppTypography.bodySmall(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: '同意并继续',
                    icon: Icons.check_circle_outline,
                    isFullWidth: true,
                    onPressed: _agreed ? _agree : null,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
