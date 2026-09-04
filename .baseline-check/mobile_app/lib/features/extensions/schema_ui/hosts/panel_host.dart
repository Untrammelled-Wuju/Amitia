import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../models/schema_ui_types.dart';
import '../renderer/schema_ui_renderer.dart';
import 'extension_page_host.dart';

class PanelHost extends ConsumerStatefulWidget {
  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final SchemaUIDocument? document;
  final PanelConstraints? constraints;

  const PanelHost({
    super.key,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.document,
    this.constraints,
  });

  @override
  ConsumerState<PanelHost> createState() => _PanelHostState();
}

class PanelConstraints {
  final double? minWidth;
  final double? maxWidth;
  final double? preferredHeight;

  const PanelConstraints({this.minWidth, this.maxWidth, this.preferredHeight});
}

class _PanelHostState extends ConsumerState<PanelHost> {
  SchemaUIDocument? _document;
  String? _error;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadDocument();
  }

  void _loadDocument() {
    setState(() => _isLoading = true);
    Future.delayed(const Duration(milliseconds: 100), () {
      if (!mounted) return;
      setState(() {
        _document = widget.document;
        _isLoading = false;
        if (_document == null) {
          _error = 'No schema document available';
        }
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.constraints;
    return ConstrainedBox(
      constraints: BoxConstraints(
        minWidth: c?.minWidth ?? 280,
        maxWidth: c?.maxWidth ?? 400,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildHeader(context),
          Flexible(child: _buildBody(context)),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Row(
        children: [
          Expanded(
            child: Text(
              _document?.title ?? 'Panel',
              style: Theme.of(context).textTheme.titleSmall,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _loadDocument,
            tooltip: 'Refresh',
          ),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_isLoading) {
      return const Padding(
        padding: EdgeInsets.all(16),
        child: AmitiaLoadingState(message: 'Loading...'),
      );
    }
    if (_error != null || _document == null) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: AmitiaEmptyState(
          icon: Icons.error_outline,
          title: 'Failed',
          subtitle: _error,
          actionText: 'Retry',
          onAction: _loadDocument,
        ),
      );
    }
    return ErrorBoundary(
      contributionId: widget.contributionId,
      extensionId: widget.extensionId,
      moduleId: widget.moduleId,
      child: SingleChildScrollView(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        child: SchemaUIRenderer(
          document: _document!,
          extensionId: widget.extensionId,
          contributionId: widget.contributionId,
          moduleId: widget.moduleId,
          permissions: widget.permissions,
        ),
      ),
    );
  }
}
