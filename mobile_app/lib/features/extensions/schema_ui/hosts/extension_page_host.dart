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
