import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../models/schema_ui_types.dart';
import '../renderer/schema_ui_renderer.dart';
import 'extension_page_host.dart';

class ChatExtensionHost extends ConsumerStatefulWidget {
  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final SchemaUIDocument? document;
  final VoidCallback? onClose;

  const ChatExtensionHost({
    super.key,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.document,
    this.onClose,
  });

  @override
  ConsumerState<ChatExtensionHost> createState() => _ChatExtensionHostState();
}

class _ChatExtensionHostState extends ConsumerState<ChatExtensionHost> {
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
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildHeader(context),
        ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: 400),
          child: _buildBody(context),
        ),
      ],
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: Row(
        children: [
          Expanded(
            child: Text(
              _document?.title ?? 'Extension',
              style: Theme.of(context).textTheme.titleSmall,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          if (widget.onClose != null)
            AmitiaIconButton(
              icon: Icons.close,
              onPressed: widget.onClose,
              tooltip: 'Close',
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

class ContextActionHost extends ConsumerStatefulWidget {
  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final SchemaUIDocument? document;
  final VoidCallback? onDismiss;

  const ContextActionHost({
    super.key,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.document,
    this.onDismiss,
  });

  @override
  ConsumerState<ContextActionHost> createState() => _ContextActionHostState();
}

class _ContextActionHostState extends ConsumerState<ContextActionHost> {
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
    Future.delayed(const Duration(milliseconds: 50), () {
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
    return Card(
      margin: const EdgeInsets.all(8),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildHeader(context),
          ConstrainedBox(
            constraints: const BoxConstraints(maxHeight: 300),
            child: _buildBody(context),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Row(
        children: [
          Expanded(
            child: Text(
              _document?.title ?? 'Action',
              style: Theme.of(context).textTheme.titleSmall,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          if (widget.onDismiss != null)
            AmitiaIconButton(
              icon: Icons.close,
              onPressed: widget.onDismiss,
              tooltip: 'Dismiss',
            ),
        ],
      ),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_isLoading) {
      return const Padding(
        padding: EdgeInsets.all(12),
        child: AmitiaLoadingState(message: 'Loading...'),
      );
    }
    if (_error != null || _document == null) {
      return Padding(
        padding: const EdgeInsets.all(12),
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
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
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
