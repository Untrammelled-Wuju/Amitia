import 'package:flutter/material.dart';

Future<bool> showGamePackagePreviewConfirmation(
  BuildContext context,
  Map<String, dynamic> preview, {
  required String actionLabel,
}) async {
  final risks = ((preview['highRiskCapabilities'] as List?) ?? const [])
      .map((e) => e.toString())
      .where((e) => e.isNotEmpty)
      .toList(growable: false);
  final warnings = ((preview['warnings'] as List?) ?? const [])
      .map((e) => e.toString())
      .where((e) => e.isNotEmpty)
      .toList(growable: false);
  final signature = preview['signature'];
  final signatureStatus = signature is Map ? (signature['status'] ?? 'unknown').toString() : 'unknown';
  final result = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text('$actionLabel游戏插件'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('名称：${preview['name'] ?? preview['id'] ?? '未知'}'),
            Text('扩展 ID：${preview['id'] ?? ''}'),
            Text('版本：${preview['version'] ?? ''}'),
            if ((preview['currentVersion'] ?? '').toString().isNotEmpty)
              Text('当前版本：${preview['currentVersion']}'),
            Text('签名：$signatureStatus'),
            if (risks.isNotEmpty) ...[
              const SizedBox(height: 10),
              const Text('高风险能力：', style: TextStyle(fontWeight: FontWeight.w600)),
              Text(risks.join('、')),
            ],
            if (warnings.isNotEmpty) ...[
              const SizedBox(height: 10),
              const Text('警告：', style: TextStyle(fontWeight: FontWeight.w600)),
              ...warnings.take(5).map((warning) => Text('• $warning')),
            ],
            const SizedBox(height: 10),
            const Text('继续表示你确认本次预览中列出的权限、脚本、签名及版本变更。'),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
        FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('确认$actionLabel')),
      ],
    ),
  );
  return result == true;
}

Future<bool> showGamePackageUninstallConfirmation(
  BuildContext context,
  Map<String, dynamic> preview, {
  required String displayName,
}) async {
  final dependents = ((preview['dependents'] as List?) ?? const [])
      .map((e) => e.toString())
      .where((e) => e.isNotEmpty)
      .toList(growable: false);
  final allowed = preview['uninstallable'] != false;
  final result = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('卸载游戏插件'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('插件：$displayName'),
          Text('当前版本：${preview['currentVersion'] ?? ''}'),
          Text('制品策略：${preview['artifactPolicy'] ?? 'unknown'}'),
          if (dependents.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text('依赖此扩展：${dependents.join('、')}'),
          ],
          if (!allowed) ...[
            const SizedBox(height: 8),
            const Text('后端判定当前不可卸载。'),
          ],
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
        FilledButton(
          onPressed: allowed ? () => Navigator.pop(dialogContext, true) : null,
          child: const Text('确认卸载'),
        ),
      ],
    ),
  );
  return result == true;
}
