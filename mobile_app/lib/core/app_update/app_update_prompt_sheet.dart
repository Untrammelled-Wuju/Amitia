import 'dart:async';

import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../features/conversation/rendering/markdown/amitia_markdown.dart';
import '../widgets/amitia_button.dart';
import 'app_update_models.dart';
import 'app_update_service.dart';

class AppUpdateDownloadController extends ChangeNotifier {
  AppUpdateDownloadController({
    required AppUpdateService service,
    required this.update,
  }) : _service = service,
       super();

  final AppUpdateService _service;
  final AppUpdateManifest update;
  final Completer<bool> _terminal = Completer<bool>();

  Timer? _timer;
  bool _polling = false;
  bool _disposed = false;
  bool downloading = false;
  bool completed = false;
  int? downloadId;
  int downloadedBytes = 0;
  int totalBytes = 0;
  int reasonCode = 0;
  String? errorMessage;

  Future<bool> get terminal => _terminal.future;

  double get progress {
    if (totalBytes <= 0) return 0;
    return (downloadedBytes / totalBytes).clamp(0, 1).toDouble();
  }

  Future<void> start() async {
    if (downloading || completed) return;
    downloading = true;
    errorMessage = null;
    downloadedBytes = 0;
    totalBytes = update.apk.size;
    _notify();
    try {
      downloadId = await _service.download(update);
      _startPolling();
      await _poll();
    } catch (error) {
      _fail(error.toString());
    }
  }

  Future<void> restore() async {
    if (downloading || completed) return;
    try {
      final status = await _service.getCurrentDownload(update);
      if (_disposed || status == null) return;
      downloadId = status.downloadId;
      _applyStatus(status);
      if (status.successful) {
        _complete();
      } else if (status.failed) {
        _fail(status.errorDetail);
      } else {
        downloading = true;
        _startPolling();
        _notify();
      }
    } catch (error) {
      _fail(error.toString());
    }
  }

  Future<void> _poll() async {
    if (_polling || _disposed) return;
    final id = downloadId;
    if (id == null) return;
    _polling = true;
    try {
      final status = await _service.getDownloadStatus(id);
      _applyStatus(status);
      if (status.successful) {
        _complete();
      } else if (status.failed) {
        _fail(status.errorDetail);
      } else {
        _notify();
      }
    } catch (error) {
      _fail(error.toString());
    } finally {
      _polling = false;
    }
  }

  void _startPolling() {
    _timer?.cancel();
    _timer = Timer.periodic(
      const Duration(seconds: 1),
      (_) => unawaited(_poll()),
    );
  }

  void _applyStatus(AppUpdateDownloadStatus status) {
    downloadedBytes = status.downloadedBytes;
    totalBytes = status.totalBytes > 0 ? status.totalBytes : update.apk.size;
    reasonCode = status.reasonCode;
  }

  void _complete() {
    _timer?.cancel();
    _timer = null;
    downloading = false;
    completed = true;
    _notify();
    if (!_terminal.isCompleted) _terminal.complete(true);
  }

  void _fail(String message) {
    if (_disposed) return;
    _timer?.cancel();
    _timer = null;
    downloading = false;
    errorMessage = message;
    _notify();
    if (!_terminal.isCompleted) _terminal.complete(false);
  }

  void _notify() {
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    _timer?.cancel();
    super.dispose();
  }
}

class AppUpdatePromptSheet extends StatefulWidget {
  const AppUpdatePromptSheet({
    super.key,
    required this.update,
    required this.currentVersion,
    required this.controller,
    required this.mandatory,
  });

  final AppUpdateManifest update;
  final String currentVersion;
  final AppUpdateDownloadController controller;
  final bool mandatory;

  @override
  State<AppUpdatePromptSheet> createState() => _AppUpdatePromptSheetState();
}

class _AppUpdatePromptSheetState extends State<AppUpdatePromptSheet> {
  bool _closingForCompletion = false;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_handleControllerChange);
    unawaited(widget.controller.restore());
  }

  @override
  void didUpdateWidget(covariant AppUpdatePromptSheet oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller.removeListener(_handleControllerChange);
      widget.controller.addListener(_handleControllerChange);
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_handleControllerChange);
    super.dispose();
  }

  void _handleControllerChange() {
    if (!mounted) return;
    if (widget.controller.completed && !_closingForCompletion) {
      _closingForCompletion = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) Navigator.of(context).pop('downloaded');
      });
      return;
    }
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final controller = widget.controller;
    final height = MediaQuery.sizeOf(context).height * 0.9;
    return PopScope(
      canPop:
          !widget.mandatory ||
          controller.downloading ||
          controller.completed ||
          controller.errorMessage != null,
      child: SafeArea(
        top: false,
        child: Container(
          height: height,
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: BorderRadius.vertical(
              top: Radius.circular(AppRadius.large),
            ),
          ),
          child: Column(
            children: [
              const SizedBox(height: 8),
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(999),
                ),
              ),
              _buildHeader(context, controller),
              Divider(height: 1, color: context.borderSecondary),
              Expanded(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(18, 14, 18, 24),
                  child: _buildBody(context, controller),
                ),
              ),
              _buildFooter(context, controller),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(
    BuildContext context,
    AppUpdateDownloadController controller,
  ) {
    final canClose = !widget.mandatory || controller.downloading;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 12, 14),
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: BorderRadius.circular(AppRadius.small),
              border: Border.all(color: context.borderSecondary, width: 0.6),
            ),
            child: Icon(
              Icons.system_update_alt_rounded,
              color: context.accentPrimary,
              size: 22,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  controller.downloading ? '正在下载更新' : '发现新版本',
                  style: AppTypography.cardTitle(
                    context,
                  ).copyWith(fontSize: 18, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 2),
                Text(
                  controller.downloading
                      ? '关闭弹窗不会中止下载，完成后会提示确认安装'
                      : 'Amitia 可以更新了！',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          if (canClose)
            IconButton(
              onPressed: () => Navigator.of(context).pop(),
              tooltip: '关闭更新弹窗',
              icon: Icon(Icons.close_rounded, color: context.textSecondary),
            ),
        ],
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    AppUpdateDownloadController controller,
  ) {
    final update = widget.update;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          _versionText(update.versionName),
          style: AppTypography.pageLargeTitle(context).copyWith(
            fontSize: 30,
            fontWeight: FontWeight.w700,
            letterSpacing: -0.8,
          ),
        ),
        const SizedBox(height: 8),
        Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: 7,
          children: [
            Text(
              _versionText(widget.currentVersion),
              key: const ValueKey('app-update-current-version'),
              style: AppTypography.caption(context),
            ),
            Icon(
              Icons.arrow_forward_rounded,
              size: 14,
              color: context.textTertiary,
            ),
            Text(
              _versionText(update.versionName),
              key: const ValueKey('app-update-latest-version'),
              style: AppTypography.caption(
                context,
              ).copyWith(fontWeight: FontWeight.w600),
            ),
          ],
        ),
        const SizedBox(height: 16),
        Container(
          padding: const EdgeInsets.symmetric(vertical: 12),
          decoration: BoxDecoration(
            border: Border(
              top: BorderSide(color: context.borderSecondary),
              bottom: BorderSide(color: context.borderSecondary),
            ),
          ),
          child: Row(
            children: [
              Expanded(
                child: _metaItem(context, '安装包', _formatBytes(update.apk.size)),
              ),
              Expanded(
                child: _metaItem(
                  context,
                  '发布日期',
                  _formatDate(update.publishedAt),
                ),
              ),
              Expanded(
                child: _metaItem(context, '通道', _channelLabel(update.channel)),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        Text(
          '升级后首次启动可能需要一小段时间完成运行环境和数据库检查。',
          style: AppTypography.label(
            context,
          ).copyWith(fontSize: 10, height: 1.5),
        ),
        const SizedBox(height: 2),
        Text(
          '如遇到升级后不可用的情况，可在设置-问题反馈中提交问题，或添加QQ群聊193879020进行反馈',
          style: AppTypography.label(
            context,
          ).copyWith(fontSize: 10, height: 1.5),
        ),
        const SizedBox(height: 22),
        Text(
          '本次更新',
          style: AppTypography.cardTitle(
            context,
          ).copyWith(fontSize: 15, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 7),
        Text(
          update.updateTitle.isEmpty ? '查看本次版本的主要变化。' : update.updateTitle,
          style: AppTypography.body(
            context,
          ).copyWith(color: context.textSecondary),
        ),
        if (update.highlights.isNotEmpty) ...[
          const SizedBox(height: 20),
          Container(
            padding: const EdgeInsets.fromLTRB(14, 13, 14, 14),
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: BorderRadius.circular(AppRadius.small),
              border: Border.all(
                color: context.accentPrimary.withValues(alpha: 0.22),
                width: 0.7,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(
                      Icons.auto_awesome,
                      color: context.accentPrimary,
                      size: 15,
                    ),
                    const SizedBox(width: 7),
                    Text(
                      '本版亮点',
                      style: AppTypography.caption(context).copyWith(
                        color: context.accentPrimary,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 9),
                for (final highlight in update.highlights)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 7),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(
                          width: 5,
                          height: 5,
                          margin: const EdgeInsets.only(top: 7),
                          decoration: BoxDecoration(
                            color: context.accentPrimary,
                            borderRadius: BorderRadius.circular(999),
                          ),
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            highlight,
                            style: AppTypography.caption(context).copyWith(
                              color: context.textSecondary,
                              height: 1.6,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
        if (update.releaseNotes.trim().isNotEmpty) ...[
          const SizedBox(height: 22),
          Text(
            '完整更新内容',
            style: AppTypography.cardTitle(
              context,
            ).copyWith(fontSize: 15, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 10),
          AmitiaMarkdownView(source: update.releaseNotes),
        ],
        if (controller.errorMessage != null) ...[
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.error.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(AppRadius.small),
              border: Border.all(
                color: context.error.withValues(alpha: 0.3),
                width: 0.7,
              ),
            ),
            child: Text(
              controller.errorMessage!,
              style: AppTypography.caption(
                context,
              ).copyWith(color: context.error),
            ),
          ),
        ],
      ],
    );
  }

  Widget _metaItem(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: AppTypography.label(context).copyWith(fontSize: 10),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.caption(
              context,
            ).copyWith(color: context.textPrimary, fontWeight: FontWeight.w600),
          ),
        ],
      ),
    );
  }

  Widget _buildFooter(
    BuildContext context,
    AppUpdateDownloadController controller,
  ) {
    return Container(
      padding: EdgeInsets.fromLTRB(
        18,
        12,
        18,
        18 + MediaQuery.paddingOf(context).bottom,
      ),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(top: BorderSide(color: context.borderSecondary)),
      ),
      child: controller.downloading
          ? _buildProgress(context, controller)
          : _buildActions(context, controller),
    );
  }

  Widget _buildActions(
    BuildContext context,
    AppUpdateDownloadController controller,
  ) {
    if (controller.errorMessage != null) {
      return AmitiaButton(
        label: '关闭',
        isSecondary: true,
        isFullWidth: true,
        height: 50,
        onPressed: () => Navigator.of(context).pop(),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        AmitiaButton(
          label: '下载并安装',
          icon: Icons.download_rounded,
          isFullWidth: true,
          height: 50,
          onPressed: () => unawaited(controller.start()),
        ),
        if (!widget.mandatory) ...[
          const SizedBox(height: 7),
          Row(
            children: [
              Expanded(
                child: TextButton(
                  onPressed: () => Navigator.of(context).pop('later'),
                  child: Text('稍后提醒', style: AppTypography.caption(context)),
                ),
              ),
              Expanded(
                child: TextButton(
                  onPressed: () => Navigator.of(context).pop('skip'),
                  child: Text('跳过此版本', style: AppTypography.caption(context)),
                ),
              ),
            ],
          ),
        ],
      ],
    );
  }

  Widget _buildProgress(
    BuildContext context,
    AppUpdateDownloadController controller,
  ) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                '正在下载更新',
                style: AppTypography.caption(context).copyWith(
                  color: context.textPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            Text(
              '${(controller.progress * 100).round()}%',
              style: AppTypography.caption(context).copyWith(
                color: context.accentPrimary,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        ClipRRect(
          borderRadius: BorderRadius.circular(999),
          child: LinearProgressIndicator(
            value: controller.progress,
            minHeight: 7,
            color: context.accentPrimary,
            backgroundColor: context.accentSoft,
          ),
        ),
        const SizedBox(height: 7),
        Row(
          children: [
            Expanded(
              child: Text(
                '${_formatBytes(controller.downloadedBytes)} / '
                '${_formatBytes(controller.totalBytes)}',
                style: AppTypography.label(context).copyWith(fontSize: 10),
              ),
            ),
            Text(
              '可关闭弹窗，后台继续下载',
              style: AppTypography.label(context).copyWith(fontSize: 10),
            ),
          ],
        ),
      ],
    );
  }
}

Future<bool?> showAppUpdateReadyDialog(
  BuildContext context,
  AppUpdateManifest update,
) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (dialogContext) => PopScope(
      canPop: false,
      child: Dialog(
        backgroundColor: dialogContext.surfacePrimary,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.large),
        ),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 28, 24, 22),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 50,
                height: 50,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: dialogContext.success.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(AppRadius.small),
                ),
                child: Icon(
                  Icons.check_rounded,
                  color: dialogContext.success,
                  size: 26,
                ),
              ),
              const SizedBox(height: 16),
              Text(
                '新版本已下载完成',
                style: AppTypography.pageTitle(
                  dialogContext,
                ).copyWith(fontWeight: FontWeight.w700),
              ),
              const SizedBox(height: 8),
              Text(
                '${_versionText(update.versionName)} 已准备好安装。点击“确定”后开始安装。',
                textAlign: TextAlign.center,
                style: AppTypography.bodySmall(
                  dialogContext,
                ).copyWith(color: dialogContext.textSecondary, height: 1.6),
              ),
              const SizedBox(height: 22),
              AmitiaButton(
                label: '确定',
                isFullWidth: true,
                height: 50,
                onPressed: () => Navigator.of(dialogContext).pop(true),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

String _versionText(String value) {
  final normalized = value.trim();
  if (normalized.isEmpty) return '—';
  return normalized.startsWith('v') ? normalized : 'v$normalized';
}

String _formatBytes(int bytes) {
  if (bytes <= 0) return '0 MB';
  if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
  if (bytes < 1024 * 1024 * 1024) {
    return '${(bytes / 1024 / 1024).toStringAsFixed(1)} MB';
  }
  return '${(bytes / 1024 / 1024 / 1024).toStringAsFixed(2)} GB';
}

String _formatDate(DateTime? value) {
  if (value == null) return '—';
  final local = value.toLocal();
  return '${local.month} 月 ${local.day} 日';
}

String _channelLabel(String channel) {
  switch (channel.trim().toLowerCase()) {
    case 'beta':
      return '测试版';
    case 'alpha':
      return '内测版';
    default:
      return '正式版';
  }
}
