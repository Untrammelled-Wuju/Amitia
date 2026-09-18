import 'dart:async';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../app/theme/app_typography.dart';
import '../services/extension_service.dart';
import '../widgets/amitia_button.dart';
import 'mobile_ui_host_event_client.dart';

/// Applies server-authoritative UI Host commands at the application shell.
///
/// Keeping this host above individual routes makes AI/extension navigation,
/// dialogs and notifications available even when no feature page is mounted.
class MobileUIHostCommandHost extends StatefulWidget {
  const MobileUIHostCommandHost({
    super.key,
    required this.router,
    required this.navigatorKey,
    required this.extensionService,
    required this.child,
  });

  final GoRouter router;
  final GlobalKey<NavigatorState> navigatorKey;
  final ExtensionService extensionService;
  final Widget child;

  @override
  State<MobileUIHostCommandHost> createState() => _MobileUIHostCommandHostState();
}

class _MobileUIHostCommandHostState extends State<MobileUIHostCommandHost> {
  StreamSubscription<MobileUIHostCommand>? _subscription;
  Future<void> _dialogQueue = Future<void>.value();

  @override
  void initState() {
    super.initState();
    _subscription = MobileUIHostCommandBus.commands.listen(_handleCommand);
  }

  @override
  void dispose() {
    final subscription = _subscription;
    if (subscription != null) unawaited(subscription.cancel());
    super.dispose();
  }

  void _handleCommand(MobileUIHostCommand command) {
    if (!mounted) return;
    switch (command.eventType) {
      case 'ui_notify':
        _showNotification(command.payload);
        return;
      case 'ui_navigate':
        _navigate(command.payload);
        return;
      case 'ui_dialog':
        _dialogQueue = _dialogQueue.then((_) => _showDialog(command));
        return;
    }
  }

  void _showNotification(Map<String, dynamic> payload) {
    final title = (payload['title'] ?? '').toString().trim();
    final body = (payload['body'] ?? payload['message'] ?? '').toString().trim();
    if (title.isEmpty && body.isEmpty) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final hostContext = widget.navigatorKey.currentState?.overlay?.context ?? context;
      final messenger = ScaffoldMessenger.maybeOf(hostContext);
      if (messenger == null) return;
      final message = title.isEmpty
          ? body
          : body.isEmpty
              ? title
              : '$title\n$body';
      messenger
        ..clearSnackBars()
        ..showSnackBar(
          SnackBar(
            content: Text(message),
            behavior: SnackBarBehavior.floating,
            duration: const Duration(seconds: 5),
          ),
        );
    });
  }

  void _navigate(Map<String, dynamic> payload) {
    final target = (payload['target'] ?? payload['path'] ?? '').toString().trim();
    if (!_isSafeInternalTarget(target)) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      try {
        widget.router.go(target);
      } catch (_) {
        // Unknown/removed extension routes are ignored. The host must remain alive.
      }
    });
  }

  Future<void> _showDialog(MobileUIHostCommand command) async {
    if (!mounted) return;
    final payload = command.payload;
    final dialogId = (payload['dialogId'] ?? '').toString().trim();
    if (dialogId.isEmpty) return;
    final title = (payload['title'] ?? '对话框').toString().trim();
    final message = (payload['message'] ?? payload['body'] ?? '').toString();
    final buttons = _dialogButtons(payload['buttons']);
    final dialogContext = widget.navigatorKey.currentState?.overlay?.context;
    if (dialogContext == null) {
      await _respondDialog(command, dialogId, 'closed');
      return;
    }
    final result = await showDialog<String>(
      context: dialogContext,
      barrierDismissible: true,
      builder: (alertContext) => AlertDialog(
        title: Text(title.isEmpty ? '对话框' : title, style: AppTypography.cardTitle(alertContext)),
        content: Text(message, style: AppTypography.body(alertContext)),
        actions: [
          for (var index = 0; index < buttons.length; index++)
            AmitiaButton(
              label: buttons[index],
              isSecondary: index != 0,
              height: 40,
              onPressed: () => Navigator.of(alertContext).pop(buttons[index]),
            ),
        ],
      ),
    );
    if (!mounted) return;
    await _respondDialog(command, dialogId, result ?? 'closed');
  }

  Future<void> _respondDialog(
    MobileUIHostCommand command,
    String dialogId,
    String result,
  ) async {
    final payload = command.payload;
    final responseHostClientId =
        (command.envelope['hostClientId'] ?? payload['hostClientId'] ?? '').toString().trim();
    final responseHostSessionId =
        (command.envelope['hostSessionId'] ?? payload['hostSessionId'] ?? '').toString().trim();
    try {
      await widget.extensionService.sendUIDialogResponse(
        dialogId: dialogId,
        result: result,
        hostClientId: responseHostClientId,
        hostSessionId: responseHostSessionId,
      );
    } catch (_) {
      // The server owns timeout/cancellation. A lost response must not crash UI.
    }
  }

  static List<String> _dialogButtons(dynamic raw) {
    final values = raw is List
        ? raw.map((item) => item.toString().trim()).where((item) => item.isNotEmpty).take(4).toList()
        : <String>[];
    return values.isEmpty ? const <String>['确定'] : values;
  }

  static bool _isSafeInternalTarget(String target) {
    if (target.isEmpty || target.length > 2048) return false;
    if (!target.startsWith('/') || target.startsWith('//') || target.contains('\\')) {
      return false;
    }
    if (target.codeUnits.any((unit) => unit < 0x20 || unit == 0x7f)) return false;
    final uri = Uri.tryParse(target);
    if (uri == null || uri.hasScheme || uri.hasAuthority) return false;
    return uri.path.startsWith('/');
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
