import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import '../services/shared_core_data_access.dart';
import '../errors/original_error.dart';

void showSharedCoreAccessReason(
  BuildContext context,
  AsyncValue<SharedCoreDataAccess> state,
) {
  final reason = state.valueOrNull?.reason;
  final message = state.isLoading
      ? '正在确认统筹中心数据权限，请稍后重试。'
      : reason == null || reason.isEmpty
      ? '当前设备只能查看统筹中心数据。'
      : reason;
  final details = state.hasError
      ? '\n\n原始错误：\n${state.error}\n${originalErrorText(state.error!)}\n${state.stackTrace}'
      : '';
  showDialog<void>(
    context: context,
    builder: (context) => AlertDialog(
      title: const Text('暂时无法操作'),
      content: SingleChildScrollView(child: SelectableText('$message$details')),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('知道了'),
        ),
      ],
    ),
  );
}

class SharedCoreReadOnlyBanner extends StatelessWidget {
  final SharedCoreDataAccess access;

  const SharedCoreReadOnlyBanner({super.key, required this.access});

  @override
  Widget build(BuildContext context) {
    if (!access.readOnly) return const SizedBox.shrink();
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        0,
      ),
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.accentPrimary.withValues(alpha: .35)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.visibility_outlined, color: context.accentPrimary),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('当前为中心只读模式', style: AppTypography.label(context)),
                SizedBox(height: 2),
                Text(
                  access.reason.isEmpty
                      ? '当前设备未获统筹中心管理员授权，只能查看中心数据。'
                      : access.reason,
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
