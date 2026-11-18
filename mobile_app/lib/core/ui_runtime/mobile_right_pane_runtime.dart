import 'package:flutter_riverpod/flutter_riverpod.dart';

class MobileRightPaneState {
  const MobileRightPaneState({
    this.open = false,
    this.address,
    this.title,
    this.params = const {},
    this.sessionId,
  });

  final bool open;
  final String? address;
  final String? title;
  final Map<String, dynamic> params;
  final String? sessionId;

  Map<String, dynamic> toContext() => <String, dynamic>{
    'rightPane': <String, dynamic>{
      'open': open,
      'address': address,
      'title': title,
      'params': params,
      'sessionId': sessionId,
    },
    if (address?.trim().isNotEmpty == true) 'address': address,
    if (params.isNotEmpty) 'params': params,
    if (sessionId?.trim().isNotEmpty == true) 'sessionId': sessionId,
  };
}

class MobileRightPaneController extends StateNotifier<MobileRightPaneState> {
  MobileRightPaneController() : super(const MobileRightPaneState());

  void open(
    String address, {
    String? title,
    Map<String, dynamic> params = const {},
    String? sessionId,
  }) {
    final value = address.trim();
    if (value.isEmpty) {
      throw StateError('right pane address is required');
    }
    state = MobileRightPaneState(
      open: true,
      address: value,
      title: title?.trim().isEmpty == false ? title!.trim() : null,
      params: Map<String, dynamic>.unmodifiable(params),
      sessionId: sessionId?.trim().isEmpty == false ? sessionId!.trim() : null,
    );
  }

  void close() {
    if (!state.open) return;
    state = const MobileRightPaneState();
  }
}

final mobileRightPaneProvider =
    StateNotifierProvider<MobileRightPaneController, MobileRightPaneState>(
      (ref) => MobileRightPaneController(),
    );
