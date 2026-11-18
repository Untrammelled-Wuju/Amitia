import 'package:flutter_riverpod/flutter_riverpod.dart';

class MobileWorkspaceSidebarState {
  const MobileWorkspaceSidebarState({this.open = false});

  final bool open;

  Map<String, dynamic> toContext() => <String, dynamic>{
    'workspaceSidebar': <String, dynamic>{'open': open},
  };
}

class MobileWorkspaceSidebarController
    extends StateNotifier<MobileWorkspaceSidebarState> {
  MobileWorkspaceSidebarController()
    : super(const MobileWorkspaceSidebarState());

  void open() {
    if (state.open) return;
    state = const MobileWorkspaceSidebarState(open: true);
  }

  void close() {
    if (!state.open) return;
    state = const MobileWorkspaceSidebarState();
  }
}

final mobileWorkspaceSidebarProvider =
    StateNotifierProvider<
      MobileWorkspaceSidebarController,
      MobileWorkspaceSidebarState
    >((ref) => MobileWorkspaceSidebarController());
