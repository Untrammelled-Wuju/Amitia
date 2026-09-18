import 'package:flutter/material.dart';

import '../../../../core/widgets/amitia_scaffold.dart';
import '../../presentation/pages/extension_page_host_page.dart' as runtime;
import '../engine/data_source_loader.dart';
import '../models/schema_ui_types.dart';
import '../renderer/schema_ui_renderer.dart';

/// Backward-compatible entry point for callers that still import the old
/// schema-ui host path. Remote pages are delegated to the real runtime host;
/// only explicitly preloaded schema documents render locally.
class ExtensionPageHostPage extends StatelessWidget {
  const ExtensionPageHostPage({
    super.key,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.document,
    this.dataSourceLoader,
  });

  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final SchemaUIDocument? document;
  final DataSourceLoader? dataSourceLoader;

  @override
  Widget build(BuildContext context) {
    final schema = document;
    if (schema == null) {
      return runtime.ExtensionPageHostPage(
        pageId: contributionId,
        extensionId: extensionId,
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: schema.title ?? 'Extension Page',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: SchemaUIRenderer(
          document: schema,
          extensionId: extensionId,
          contributionId: contributionId,
          moduleId: moduleId,
          permissions: permissions,
          dataSourceLoader: dataSourceLoader,
        ),
      ),
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
  void Function(FlutterErrorDetails)? _previousHandler;

  @override
  void initState() {
    super.initState();
    _previousHandler = FlutterError.onError;
    FlutterError.onError = (details) {
      if (details.library == 'widgets library' ||
          details.library?.contains('schema_ui') == true) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            setState(() => _error = details.exceptionAsString());
          }
        });
      }
      (_previousHandler ?? FlutterError.dumpErrorToConsole)(details);
    };
  }

  @override
  void dispose() {
    FlutterError.onError = _previousHandler;
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error == null) return widget.child;
    return Container(
      padding: const EdgeInsets.all(16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: 48,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 12),
          Text('Render Error', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(
            'Extension: ${widget.extensionId}\nContribution: ${widget.contributionId}',
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 8),
          Text(
            _error!,
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: Theme.of(context).colorScheme.error,
            ),
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}
