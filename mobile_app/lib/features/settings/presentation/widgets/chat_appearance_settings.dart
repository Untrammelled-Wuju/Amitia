import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/settings/chat_appearance_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ChatAppearanceSettings extends ConsumerStatefulWidget {
  const ChatAppearanceSettings({super.key});

  @override
  ConsumerState<ChatAppearanceSettings> createState() =>
      _ChatAppearanceSettingsState();
}

class _ChatAppearanceSettingsState
    extends ConsumerState<ChatAppearanceSettings> {
  bool _busy = true;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      await ref.read(chatAppearancePreferencesProvider.notifier).init();
    } catch (_) {
      _failed = true;
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _update(int index) async {
    setState(() => _busy = true);
    try {
      await ref
          .read(chatAppearancePreferencesProvider.notifier)
          .setMessageStyle(ChatMessageStyle.values[index]);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存聊天界面风格失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final style = ref.watch(chatAppearancePreferencesProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        IgnorePointer(
          ignoring: _busy || _failed,
          child: AmitiaSegmentedControl(
            segments: const ['流式消息', '气泡消息'],
            selectedIndex: style.index,
            onChanged: _update,
          ),
        ),
        const SizedBox(height: 10),
        Text(
          _failed
              ? '无法读取聊天界面风格，请重新打开设置。'
              : '${style == ChatMessageStyle.flow ? '保留当前的连续阅读布局。' : '用户消息靠右，AI 消息靠左，以气泡区分。'}两种风格都支持实时生成。',
          style: AppTypography.caption(
            context,
          ).copyWith(color: context.textSecondary),
        ),
      ],
    );
  }
}
