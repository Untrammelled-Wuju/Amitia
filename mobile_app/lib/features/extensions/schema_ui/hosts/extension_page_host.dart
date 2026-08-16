import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../models/schema_ui_types.dart';
import '../renderer/schema_ui_renderer.dart';
import '../engine/data_source_loader.dart';

class ExtensionPageHostPage extends ConsumerStatefulWidget {
  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final SchemaUIDocument? document;
  final DataSourceLoader? dataSourceLoader;

  const ExtensionPageHostPage({
    super.key,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.document,
    this.dataSourceLoader,
  });

  @override
  ConsumerState<ExtensionPageHostPage> createState() => _ExtensionPageHostPageState();
}

class _ExtensionPageHostPageState extends ConsumerState<ExtensionPageHostPage> {
  bool _isLoading = true;
  String? _error;
  SchemaUIDocument? _document;

  @override
  void initState() {
    super.initState();
    _loadDocument();
  }

  void _loadDocument() {
    setState(() => _isLoading = true);
    Future.delayed(const Duration(milliseconds: 300), () {
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
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _document?.title ?? 'Extension Page',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _loadDocument,
            tooltip: 'Refresh',
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_isLoading) {
      return const AmitiaLoadingState(message: 'Loading...');
    }
    if (_error != null) {
      return _buildErrorState(context);
    }
    if (_document == null) {
      return _buildErrorState(context);
    }
    return ErrorBoundary(
      contributionId: widget.contributionId,
      extensionId: widget.extensionId,
      child: SchemaUIRenderer(
        document: _document!,
        extensionId: widget.extensionId,
        contributionId: widget.contributionId,
        moduleId: widget.moduleId,
        permissions: widget.permissions,
        dataSourceLoader: widget.dataSourceLoader,
      ),
    );
  }

  Widget _buildErrorState(BuildContext context) {
    return AmitiaEmptyState(
      icon: Icons.error_outline,
      title: 'Failed to load',
      subtitle: _error,
      actionText: 'Retry',
      onAction: _loadDocument,
    );
  }
}

class ErrorBoundary extends StatefulWidget {
  final Widget child;
  final String extensionId;
  final String contributionId;
  final String? moduleId;

  const ErrorBoundary({
    super.key,
    required this.child,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
  });

  @override
  State<ErrorBoundary> createState() => _ErrorBoundaryState();
}

class _ErrorBoundaryState extends State<ErrorBoundary> {
  String? _error;
  StackTrace? _stackTrace;
  late FlutterErrorExceptionHandler _previousHandler;

  @override
  void initState() {
    super.initState();
    _previousHandler = FlutterError.onError!;
    FlutterError.onError = (FlutterErrorDetails details) {
      if (details.library == 'widgets library' || details.library?.contains('schema_ui') == true) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            setState(() {
              _error = details.exceptionAsString();
              _stackTrace = details.stack;
            });
          }
        });
      }
      _previousHandler(details);
    };
  }

  @override
  void dispose() {
    FlutterError.onError = _previousHandler;
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Container(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: 12),
            Text(
              'Render Error',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              'Extension: ${widget.extensionId}\nContribution: ${widget.contributionId}',
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(color: context.error),
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ],
        ),
      );
    }
    return widget.child;
  }
}
