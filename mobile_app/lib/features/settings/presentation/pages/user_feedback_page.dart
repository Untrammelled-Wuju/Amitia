import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/user_feedback_payload.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UserFeedbackPage extends ConsumerStatefulWidget {
  const UserFeedbackPage({super.key});

  @override
  ConsumerState<UserFeedbackPage> createState() => _UserFeedbackPageState();
}

class _UserFeedbackPageState extends ConsumerState<UserFeedbackPage> {
  static const List<String> _categoryValues = ['bug', 'experience', 'feature', 'other'];
  static const List<String> _categoryLabels = ['功能异常', '体验问题', '功能建议', '其他'];

  final TextEditingController _titleController = TextEditingController();
  final TextEditingController _descriptionController = TextEditingController();
  final TextEditingController _contactController = TextEditingController();
  final TextEditingController _uploadUrlController = TextEditingController();

  bool _loading = true;
  bool _submitting = false;
  bool _previewing = false;
  bool _savingUrl = false;
  bool _retrying = false;
  bool _includeLogs = true;
  String _category = 'bug';
  String _defaultUploadUrl = '';
  int _logLineLimit = 120;
  int _errorLineLimit = 40;
  int _maxScreenshotMB = 6;
  List<Map<String, dynamic>> _tickets = const [];
  Map<String, dynamic>? _screenshot;
  String? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _titleController.dispose();
    _descriptionController.dispose();
    _contactController.dispose();
    _uploadUrlController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final service = ref.read(userFeedbackServiceProvider);
      final settings = await service.settings();
      final tickets = await service.tickets();
      if (!mounted) return;
      setState(() {
        _defaultUploadUrl = (settings['defaultUploadUrl'] ?? '').toString();
        _logLineLimit = (settings['logLineLimit'] as num?)?.toInt() ?? 120;
        _errorLineLimit = (settings['errorLineLimit'] as num?)?.toInt() ?? 40;
        _maxScreenshotMB = (settings['maxScreenshotMB'] as num?)?.toInt() ?? 6;
        if (_uploadUrlController.text.trim().isEmpty) {
          final current = (settings['uploadUrl'] ?? '').toString();
          _uploadUrlController.text = current.isEmpty ? _defaultUploadUrl : current;
        }
        _tickets = tickets;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  Future<void> _reloadTickets() async {
    try {
      final tickets = await ref.read(userFeedbackServiceProvider).tickets();
      if (mounted) setState(() => _tickets = tickets);
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
// Keep the previous list when the Core is unavailable.
    }
  }

  Future<Map<String, dynamic>> _buildPayload() {
    return buildUserFeedbackPayload(
      ref: ref,
      category: _category,
      title: _titleController.text.trim(),
      description: _descriptionController.text.trim(),
      contact: _contactController.text.trim(),
      includeLogs: _includeLogs,
      uploadUrl: _uploadUrlController.text.trim(),
      screenshot: _screenshot,
    );
  }

  Future<void> _saveUploadUrl() async {
    if (_savingUrl) return;
    setState(() => _savingUrl = true);
    try {
      final result = await ref
          .read(userFeedbackServiceProvider)
          .saveUploadUrl(_uploadUrlController.text.trim());
      final saved = (result['uploadUrl'] ?? '').toString();
      if (saved.isNotEmpty) _uploadUrlController.text = saved;
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('上报地址已保存')));
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存失败：$error'), backgroundColor: context.error));
      }
    } finally {
      if (mounted) setState(() => _savingUrl = false);
    }
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
      _titleController.clear();
      _descriptionController.clear();
      _contactController.clear();
      setState(() => _screenshot = null);
      await showAmitiaInfoDialog(context, title: '提交成功', message: '反馈已提交，正在上传到管理端。');
      await _reloadTickets();
      Future<void>.delayed(const Duration(milliseconds: 1500), _reloadTickets);
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

  Future<void> _retry(Map<String, dynamic> ticket) async {
    final id = (ticket['id'] ?? '').toString();
    if (id.isEmpty || _retrying) return;
    setState(() => _retrying = true);
    try {
      await ref.read(userFeedbackServiceProvider).retry(id);
      await _reloadTickets();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('重试失败：$error'), backgroundColor: context.error));
      }
    } finally {
      if (mounted) setState(() => _retrying = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '问题反馈',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _loading ? null : _load)],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? AmitiaErrorState(message: '加载反馈设置失败：$_error', onRetry: _load)
          : RefreshIndicator(onRefresh: _load, child: _content()),
    );
  }

  Widget _content() {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaSectionHeader(title: '反馈上报地址'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AmitiaTextField(
                controller: _uploadUrlController,
                hintText: 'https://example.com/api/v1/feedbacks',
                keyboardType: TextInputType.url,
              ),
              SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '保存',
                      isFullWidth: true,
                      onPressed: _savingUrl ? null : _saveUploadUrl,
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: '恢复默认',
                      isSecondary: true,
                      isFullWidth: true,
                      onPressed: () => setState(() => _uploadUrlController.text = _defaultUploadUrl),
                    ),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              Text(
                '反馈由 Core 服务转发到该地址，离线时会自动排队并在网络恢复后重试。',
                style: AppTypography.caption(context),
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '报告问题'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('反馈类型', style: AppTypography.bodySmall(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaSegmentedControl(
                segments: _categoryLabels,
                selectedIndex: _categoryValues.indexOf(_category),
                onChanged: (index) => setState(() => _category = _categoryValues[index]),
              ),
              SizedBox(height: AppSpacing.md),
              AmitiaTextField(controller: _titleController, hintText: '标题：一句话描述问题', maxLength: 120),
              SizedBox(height: AppSpacing.md),
              AmitiaTextField(
                controller: _descriptionController,
                hintText: '详细描述：操作步骤、预期结果与实际结果',
                maxLines: 6,
              ),
              SizedBox(height: AppSpacing.md),
              AmitiaTextField(controller: _contactController, hintText: '联系方式（可选）', maxLength: 200),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
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
                      style: AppTypography.body(context),
                    ),
                  ),
                  if (_screenshot != null)
                    AmitiaIconButton(
                      icon: Icons.close,
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
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: AmitiaSwitchTile(
            title: '上传报错日志文件',
            subtitle:
                '提交时会一起上传本机日志文件中的报错日志（最近 $_logLineLimit 行运行日志与最多 $_errorLineLimit 条报错行），以及客户端捕获的异常堆栈。',
            value: _includeLogs,
            onChanged: (value) => setState(() => _includeLogs = value),
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Text(
            '上传前会自动脱敏：API Key、Token、密码、邮箱、手机号与用户目录路径会被替换，可在“查看将要上传的信息”中核对。',
            style: AppTypography.caption(context),
          ),
        ),
        SizedBox(height: AppSpacing.md),
        AmitiaButton(
          label: '查看将要上传的信息',
          icon: Icons.visibility_outlined,
          isSecondary: true,
          isFullWidth: true,
          onPressed: _previewing ? null : _openPreview,
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: '提交反馈',
          icon: Icons.send_outlined,
          isFullWidth: true,
          onPressed: _submitting ? null : _submit,
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '我的提交记录', actionText: '刷新', onAction: _reloadTickets),
        SizedBox(height: AppSpacing.sm),
        if (_tickets.isEmpty)
          AmitiaCard(child: Text('还没有提交过反馈', style: AppTypography.bodySmall(context)))
        else
          ..._tickets.map(_ticketCard),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }

  Widget _ticketCard(Map<String, dynamic> ticket) {
    final status = (ticket['status'] ?? 'pending').toString();
    final uploaded = status == 'uploaded';
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text((ticket['title'] ?? '未命名反馈').toString(), style: AppTypography.body(context)),
                ),
                AmitiaStatusBadge(
                  label: uploaded ? '已上传' : status == 'failed' ? '上传失败' : '待上传',
                  type: uploaded ? BadgeType.success : status == 'failed' ? BadgeType.error : BadgeType.warning,
                ),
              ],
            ),
            SizedBox(height: AppSpacing.xs),
            Text(_formatTime((ticket['createdAt'] ?? '').toString()), style: AppTypography.caption(context)),
            if (!uploaded) ...[
              SizedBox(height: AppSpacing.sm),
              AmitiaButton(
                label: '重试上传',
                isSecondary: true,
                isFullWidth: true,
                onPressed: _retrying ? null : () => _retry(ticket),
              ),
            ],
          ],
        ),
      ),
    );
  }

  String _formatTime(String value) {
    if (value.isEmpty) return '—';
    final parsed = DateTime.tryParse(value);
    if (parsed == null) return value;
    final local = parsed.toLocal();
    String two(int number) => number.toString().padLeft(2, '0');
    return '${local.year}-${two(local.month)}-${two(local.day)} ${two(local.hour)}:${two(local.minute)}';
  }
}
