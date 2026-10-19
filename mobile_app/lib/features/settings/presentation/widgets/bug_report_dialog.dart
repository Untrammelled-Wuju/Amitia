import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/user_feedback_payload.dart';
import '../../../../core/widgets/amitia_misc.dart';

Future<bool?> showBugReportDialog(BuildContext context, {String? attemptedPath}) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (dialogContext) => BugReportDialog(attemptedPath: attemptedPath),
  );
}

class BugReportDialog extends ConsumerStatefulWidget {
  final String? attemptedPath;

  const BugReportDialog({super.key, this.attemptedPath});

  @override
  ConsumerState<BugReportDialog> createState() => _BugReportDialogState();
}

class _BugReportDialogState extends ConsumerState<BugReportDialog> {
  final TextEditingController _titleController = TextEditingController();
  final TextEditingController _descriptionController = TextEditingController();
  final TextEditingController _contactController = TextEditingController();

  bool _submitting = false;
  bool _previewing = false;
  bool _includeLogs = true;
  int _logLineLimit = 120;
  int _errorLineLimit = 40;
  int _maxScreenshotMB = 6;
  String _uploadUrl = '';
  Map<String, dynamic>? _screenshot;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadSettings());
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    _contactController.dispose();
    super.dispose();
  }

  Future<void> _loadSettings() async {
    try {
      final settings = await ref.read(userFeedbackServiceProvider).settings();
      if (!mounted) return;
      setState(() {
        _uploadUrl = (settings['uploadUrl'] ?? settings['defaultUploadUrl'] ?? '').toString();
        _logLineLimit = (settings['logLineLimit'] as num?)?.toInt() ?? 120;
        _errorLineLimit = (settings['errorLineLimit'] as num?)?.toInt() ?? 40;
        _maxScreenshotMB = (settings['maxScreenshotMB'] as num?)?.toInt() ?? 6;
      });
    } catch (_) {}
  }

  Future<Map<String, dynamic>> _buildPayload() {
    return buildUserFeedbackPayload(
      ref: ref,
      category: 'bug',
      title: _titleController.text.trim(),
      description: _descriptionController.text.trim(),
      contact: _contactController.text.trim(),
      includeLogs: _includeLogs,
      uploadUrl: _uploadUrl,
      screenshot: _screenshot,
    );
  }

  Future<void> _pickScreenshot() async {
    try {
      final picked = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        maxWidth: 2048,
        maxHeight: 2048,
        imageQuality: 85,
      );
      if (picked == null) return;
      final bytes = await picked.readAsBytes();
      if (bytes.length > _maxScreenshotMB * 1024 * 1024) {
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('截图不能超过 $_maxScreenshotMB MB'), backgroundColor: context.error));
        }
        return;
      }
      if (!mounted) return;
      setState(() {
        _screenshot = <String, dynamic>{
          'name': picked.name.isEmpty ? 'screenshot.png' : picked.name,
          'mediaType': userFeedbackMediaTypeForPath(picked.path),
          'dataBase64': base64Encode(bytes),
        };
      });
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('选择截图失败：$error'), backgroundColor: context.error));
      }
    }
  }

  Future<void> _openPreview() async {
    if (_previewing) return;
    setState(() => _previewing = true);
    try {
      final payload = await _buildPayload();
      final result = await ref.read(userFeedbackServiceProvider).preview(payload);
      final text = (result['text'] ?? '').toString();
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('将要上传的信息'),
          content: SizedBox(
            width: double.maxFinite,
            child: SingleChildScrollView(
              child: Text(text.isEmpty ? '没有可预览的内容' : text, style: AppTypography.bodySmall(dialogContext)),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.of(dialogContext).pop(), child: const Text('关闭')),
          ],
        ),
      );
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('生成预览失败：$error'), backgroundColor: context.error));
      }
    } finally {
      if (mounted) setState(() => _previewing = false);
    }
  }

  Future<void> _submit() async {
    if (_submitting) return;
    if (_titleController.text.trim().isEmpty || _descriptionController.text.trim().isEmpty) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('请填写标题与详细描述')));
      return;
    }
    setState(() => _submitting = true);
    try {
      final payload = await _buildPayload();
      await ref.read(userFeedbackServiceProvider).submit(payload);
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('提交失败：$error'), backgroundColor: context.error));
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final viewportHeight = MediaQuery.of(context).size.height;
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: 560, maxHeight: viewportHeight * 0.86),
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brLarge,
            border: Border.all(color: context.borderPrimary),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _header(),
              Divider(height: 1, color: context.borderPrimary),
              Flexible(
                child: SingleChildScrollView(
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _hintCard(),
                      SizedBox(height: AppSpacing.md),
                      _typeRow(),
                      SizedBox(height: AppSpacing.lg),
                      _label('标题'),
                      SizedBox(height: AppSpacing.xs),
                      AmitiaTextField(
                        controller: _titleController,
                        hintText: '一句话描述问题，例如：打开设置页后页面空白',
                        maxLength: 120,
                      ),
                      SizedBox(height: AppSpacing.md),
                      _label('详细描述'),
                      SizedBox(height: AppSpacing.xs),
                      AmitiaTextField(
                        controller: _descriptionController,
                        hintText: '请补充操作步骤、预期结果与实际结果',
                        maxLines: 6,
                      ),
                      SizedBox(height: AppSpacing.md),
                      _label('联系方式（可选）'),
                      SizedBox(height: AppSpacing.xs),
                      AmitiaTextField(
                        controller: _contactController,
                        hintText: '邮箱或其它可联系到你的方式',
                        maxLength: 200,
                      ),
                      SizedBox(height: AppSpacing.md),
                      _screenshotCard(),
                      SizedBox(height: AppSpacing.md),
                      _logCard(),
                      SizedBox(height: AppSpacing.md),
                      Container(
                        width: double.infinity,
                        padding: EdgeInsets.all(AppSpacing.md),
                        decoration: BoxDecoration(
                          color: context.surfaceSecondary,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Text(
                          '上传前会自动脱敏：API Key、Token、密码、邮箱、手机号与用户目录路径会被替换，可在“查看将要上传的信息”中核对。',
                          style: AppTypography.caption(context),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              Divider(height: 1, color: context.borderPrimary),
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  AppSpacing.md,
                  AppSpacing.pagePadding,
                  AppSpacing.pagePadding,
                ),
                child: Column(
                  children: [
                    AmitiaButton(
                      label: '查看将要上传的信息',
                      icon: Icons.visibility_outlined,
                      isSecondary: true,
                      isFullWidth: true,
                      onPressed: _previewing ? null : _openPreview,
                    ),
                    SizedBox(height: AppSpacing.sm),
                    Row(
                      children: [
                        Expanded(
                          child: AmitiaButton(
                            label: '取消',
                            isSecondary: true,
                            isFullWidth: true,
                            onPressed: _submitting ? null : () => Navigator.of(context).pop(false),
                          ),
                        ),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(
                          flex: 2,
                          child: AmitiaButton(
                            label: '提交报错信息',
                            icon: Icons.send_outlined,
                            isFullWidth: true,
                            onPressed: _submitting ? null : _submit,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _header() {
    return Padding(
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.sm, AppSpacing.sm),
      child: Row(
        children: [
          Expanded(child: Text('提交报错信息', style: AppTypography.pageTitle(context))),
          IconButton(
            icon: const Icon(Icons.close),
            tooltip: '关闭',
            onPressed: _submitting ? null : () => Navigator.of(context).pop(false),
          ),
        ],
      ),
    );
  }

  Widget _hintCard() {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brMedium,
      ),
      child: Text(
        '请描述遇到的功能异常，补充操作步骤、预期结果与实际结果能帮助我们更快定位问题。提交内容会通过本机 Core 服务转发，离线时自动排队重试。',
        style: AppTypography.bodySmall(context).copyWith(color: context.textSecondary),
      ),
    );
  }

  Widget _typeRow() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text('反馈类型', style: AppTypography.bodySmall(context)),
        SizedBox(width: AppSpacing.sm),
        Container(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: 2),
          decoration: BoxDecoration(
            color: context.error.withValues(alpha: 0.1),
            borderRadius: AppRadius.brSmall,
            border: Border.all(color: context.error.withValues(alpha: 0.35)),
          ),
          child: Text(
            '功能异常',
            style: AppTypography.caption(context).copyWith(color: context.error),
          ),
        ),
        if (widget.attemptedPath != null && widget.attemptedPath!.isNotEmpty) ...[
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              '当前页面：${widget.attemptedPath}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.caption(context),
            ),
          ),
        ],
      ],
    );
  }

  Widget _label(String text) {
    return Text(text, style: AppTypography.bodySmall(context));
  }

  Widget _screenshotCard() {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.image_outlined, size: 18, color: context.textSecondary),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  _screenshot == null ? '截图（可选）' : '已选择：${_screenshot!['name']}',
                  style: AppTypography.bodySmall(context),
                ),
              ),
              if (_screenshot != null)
                IconButton(
                  icon: const Icon(Icons.close, size: 18),
                  tooltip: '移除截图',
                  onPressed: () => setState(() => _screenshot = null),
                ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          AmitiaButton(
            label: '选择图片',
            icon: Icons.add_photo_alternate_outlined,
            isSecondary: true,
            isFullWidth: true,
            onPressed: _pickScreenshot,
          ),
          SizedBox(height: AppSpacing.xs),
          Text('支持 PNG、JPG、WebP、GIF，最大 $_maxScreenshotMB MB', style: AppTypography.caption(context)),
        ],
      ),
    );
  }

  Widget _logCard() {
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary),
      ),
      child: AmitiaSwitchTile(
        title: '上传报错日志文件',
        subtitle:
            '提交时会一起上传本机日志文件中的报错日志（最近 $_logLineLimit 行运行日志与最多 $_errorLineLimit 条报错行），以及客户端捕获的异常堆栈。',
        value: _includeLogs,
        onChanged: _submitting ? null : (value) => setState(() => _includeLogs = value),
      ),
    );
  }
}
