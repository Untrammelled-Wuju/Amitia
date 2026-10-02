import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/settings/appearance_preferences.dart';
import '../../../../core/settings/custom_palette.dart';
import '../../../../core/widgets/amitia_misc.dart';

class CustomThemeSettings extends ConsumerStatefulWidget {
  const CustomThemeSettings({super.key});
  @override
  ConsumerState<CustomThemeSettings> createState() =>
      _CustomThemeSettingsState();
}

class _CustomThemeSettingsState extends ConsumerState<CustomThemeSettings> {
  bool _saving = false;
  Future<void> _save(CustomPalette value) async {
    setState(() => _saving = true);
    try {
      await ref
          .read(appearancePreferencesProvider.notifier)
          .setCustomPalette(value);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存配色失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _pick(String label, Color initial, String field) async {
    final color = await showDialog<Color>(
      context: context,
      builder: (_) => _ColorEditor(title: label, initial: initial),
    );
    if (color == null || !mounted) return;
    final palette = ref.read(appearancePreferencesProvider).customPalette;
    await _save(switch (field) {
      'primary' => palette.copyWith(primary: color),
      'secondary' => palette.copyWith(secondary: color),
      _ => palette.copyWith(text: color, textMode: ThemeTextMode.custom),
    });
  }

  @override
  Widget build(BuildContext context) {
    final palette = ref.watch(appearancePreferencesProvider).customPalette;
    final enabled = palette.enabled && !_saving;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: context.borderPrimary),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AmitiaSwitchTile(
            title: '自定义配色',
            subtitle: '关闭后恢复预设配色，保留自定义颜色',
            value: palette.enabled,
            onChanged: _saving
                ? null
                : (value) => _save(palette.copyWith(enabled: value)),
          ),
          for (final field in [
            ('主色', 'primary', palette.primary),
            ('副色', 'secondary', palette.secondary),
            ('文字颜色', 'text', palette.text),
          ])
            ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(field.$1),
              subtitle: Text(_hex(field.$3)),
              trailing: Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: field.$3,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: context.borderPrimary),
                ),
              ),
              enabled: enabled,
              onTap: enabled ? () => _pick(field.$1, field.$3, field.$2) : null,
            ),
          const SizedBox(height: 8),
          const Text('文字颜色模式'),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final mode in [
                ('自动', ThemeTextMode.auto),
                ('深色', ThemeTextMode.dark),
                ('浅色', ThemeTextMode.light),
                ('自选', ThemeTextMode.custom),
              ])
                ChoiceChip(
                  label: Text(mode.$1),
                  selected: palette.textMode == mode.$2,
                  onSelected: enabled
                      ? (_) => _save(palette.copyWith(textMode: mode.$2))
                      : null,
                ),
            ],
          ),
          const SizedBox(height: 12),
          const Text('自动按背景选择高对比文字；页面主题仍由上方亮色、暗色或跟随系统控制。'),
          const SizedBox(height: 16),
          const Text('配色对比预览'),
          const SizedBox(height: 8),
          for (final sample in [
            ('浅色背景', Colors.white, paletteForeground(palette, Colors.white)),
            (
              '深色背景',
              const Color(0xFF121214),
              paletteForeground(palette, const Color(0xFF121214)),
            ),
            ('主色操作', palette.primary, readableForeground(palette.primary)),
            ('副色操作', palette.secondary, readableForeground(palette.secondary)),
          ])
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: sample.$2,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: context.borderPrimary),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      sample.$1,
                      style: TextStyle(
                        color: sample.$3,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text('这是文字与背景的预览', style: TextStyle(color: sample.$3)),
                    Text(
                      '对比度 ${colorContrast(sample.$3, sample.$2).toStringAsFixed(2)}:1 · ${colorContrast(sample.$3, sample.$2) >= 4.5 ? '正文清晰' : '建议提高对比度'}',
                      style: TextStyle(color: sample.$3, fontSize: 12),
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

String _hex(Color color) =>
    '#${(color.toARGB32() & 0xFFFFFF).toRadixString(16).padLeft(6, '0').toUpperCase()}';

class _ColorEditor extends StatefulWidget {
  final String title;
  final Color initial;
  const _ColorEditor({required this.title, required this.initial});
  @override
  State<_ColorEditor> createState() => _ColorEditorState();
}

class _ColorEditorState extends State<_ColorEditor> {
  late Color _color = widget.initial;
  late final TextEditingController _hexController = TextEditingController(
    text: _hex(_color),
  );
  String? _error;
  @override
  void dispose() {
    _hexController.dispose();
    super.dispose();
  }

  void _channel(int index, double value) {
    final channels = [
      (_color.toARGB32() >> 16) & 255,
      (_color.toARGB32() >> 8) & 255,
      _color.toARGB32() & 255,
    ];
    channels[index] = value.round();
    setState(() {
      _color = Color.fromARGB(255, channels[0], channels[1], channels[2]);
      _hexController.text = _hex(_color);
      _error = null;
    });
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('选择${widget.title}'),
    content: SizedBox(
      width: 320,
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              height: 64,
              width: double.infinity,
              color: _color,
              alignment: Alignment.center,
              child: Text(
                '颜色预览',
                style: TextStyle(color: readableForeground(_color)),
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _hexController,
              maxLength: 7,
              decoration: InputDecoration(
                labelText: 'HEX 颜色',
                hintText: '#6C8FEA',
                errorText: _error,
              ),
              onChanged: (value) {
                final raw = value.trim().replaceFirst('#', '');
                setState(() {
                  if (RegExp(r'^[0-9a-fA-F]{6}$').hasMatch(raw)) {
                    _color = Color(0xFF000000 | int.parse(raw, radix: 16));
                    _error = null;
                  } else {
                    _error = '请输入六位十六进制颜色';
                  }
                });
              },
            ),
            for (var i = 0; i < 3; i++)
              Row(
                children: [
                  Text(['红', '绿', '蓝'][i]),
                  Expanded(
                    child: Slider(
                      value: ((_color.toARGB32() >> ((2 - i) * 8)) & 255)
                          .toDouble(),
                      min: 0,
                      max: 255,
                      divisions: 255,
                      label: '${(_color.toARGB32() >> ((2 - i) * 8)) & 255}',
                      onChanged: (value) => _channel(i, value),
                    ),
                  ),
                ],
              ),
          ],
        ),
      ),
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.pop(context),
        child: const Text('取消'),
      ),
      TextButton(
        onPressed: _error == null ? () => Navigator.pop(context, _color) : null,
        child: const Text('应用'),
      ),
    ],
  );
}
