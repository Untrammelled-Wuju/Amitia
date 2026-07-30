import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class _ModelConfig {
  final String provider;
  final String name;
  final String apiBase;
  final String apiKey;
  final double temperature;
  final int maxTokens;

  const _ModelConfig({
    required this.provider,
    required this.name,
    required this.apiBase,
    required this.apiKey,
    required this.temperature,
    required this.maxTokens,
  });
}

class ModelSettingsPage extends ConsumerStatefulWidget {
  const ModelSettingsPage({super.key});

  @override
  ConsumerState<ModelSettingsPage> createState() => _ModelSettingsPageState();
}

class _ModelSettingsPageState extends ConsumerState<ModelSettingsPage> {
  int _selectedSegment = 0;
  int _testState = 0;

  final _configs = const <_ModelConfig>[
    _ModelConfig(
      provider: 'OpenAI',
      name: 'GPT-4o',
      apiBase: 'https://api.openai.com/v1',
      apiKey: 'sk-abc123def45678gh',
      temperature: 0.7,
      maxTokens: 4096,
    ),
    _ModelConfig(
      provider: 'Anthropic',
      name: 'Claude 3.5 Sonnet',
      apiBase: 'https://api.anthropic.com',
      apiKey: 'sk-ant-xyz9988mnbv',
      temperature: 0.6,
      maxTokens: 8192,
    ),
    _ModelConfig(
      provider: 'DeepSeek',
      name: 'DeepSeek-Voice',
      apiBase: 'https://api.deepseek.com',
      apiKey: 'sk-dsv-aa1122bb334',
      temperature: 0.5,
      maxTokens: 2048,
    ),
    _ModelConfig(
      provider: '火山方舟',
      name: 'Doubao Embedding',
      apiBase: 'https://ark.cn-beijing.volces.com',
      apiKey: 'ark-vol-cc44dd55ee',
      temperature: 0.3,
      maxTokens: 512,
    ),
  ];

  String _maskKey(String key) {
    if (key.length <= 7) return '****';
    final head = key.substring(0, 3);
    final tail = key.substring(key.length - 4);
    return '$head****$tail';
  }

  Future<void> _testConnection() async {
    setState(() => _testState = 1);
    await Future.delayed(const Duration(milliseconds: 1200));
    if (mounted) setState(() => _testState = 2);
  }

  @override
  Widget build(BuildContext context) {
    final config = _configs[_selectedSegment];
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '模型设置', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          AmitiaSegmentedControl(
            segments: const ['文本模型', '视觉模型', '语音模型', '向量模型'],
            selectedIndex: _selectedSegment,
            onChanged: (i) => setState(() {
              _selectedSegment = i;
              _testState = 0;
            }),
          ),
          const SizedBox(height: AppSpacing.lg),
          _ModelCard(config: config, maskKey: _maskKey),
          const SizedBox(height: AppSpacing.lg),
          AmitiaButton(
            label: _testState == 1 ? '测试中...' : '测试连接',
            icon: _testState == 2 ? Icons.check_circle : Icons.bolt,
            isFullWidth: true,
            onPressed: _testState == 1 ? null : _testConnection,
          ),
          if (_testState == 2) ...[
            const SizedBox(height: AppSpacing.md),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(Icons.check_circle, size: 16, color: context.success),
                const SizedBox(width: 6),
                Text(
                  '连接成功',
                  style: AppTypography.caption(context).copyWith(color: context.success),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _ModelCard extends StatelessWidget {
  final _ModelConfig config;
  final String Function(String) maskKey;

  const _ModelCard({required this.config, required this.maskKey});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _InfoRow(label: '服务商', value: config.provider),
          _InfoRow(label: '模型名称', value: config.name),
          _InfoRow(label: 'API 地址', value: config.apiBase),
          _InfoRow(label: 'API Key', value: maskKey(config.apiKey)),
          const SizedBox(height: AppSpacing.sm),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(child: _InfoRow(label: '温度', value: config.temperature.toString())),
              const SizedBox(width: AppSpacing.lg),
              Expanded(child: _InfoRow(label: '最大 Token', value: config.maxTokens.toString())),
            ],
          ),
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 7),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72,
            child: Text(label, style: AppTypography.label(context)),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(
              value,
              style: AppTypography.bodySmall(context),
              textAlign: TextAlign.end,
            ),
          ),
        ],
      ),
    );
  }
}
