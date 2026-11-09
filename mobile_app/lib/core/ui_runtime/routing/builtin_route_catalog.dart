import 'package:go_router/go_router.dart';

import '../../../app/amitia_page_route.dart';
import '../../../app/app_routes.dart';
import '../../../app/route_transitions.dart';
import '../../../features/chat/presentation/pages/chat_page.dart';
import '../../../features/chat/presentation/pages/realtime_call_page.dart';
import '../../../features/conversations/presentation/pages/conversation_list_page.dart';
import '../../../features/conversations/presentation/pages/conversation_search_page.dart';
import '../../../features/agent/presentation/pages/agent_page.dart';
import '../../../features/agent/presentation/pages/agent_task_detail_page.dart';
import '../../../features/channels/presentation/pages/channel_messages_page.dart';
import '../../../features/chat_logs/presentation/pages/chat_logs_page.dart';
import '../../../features/extensions/presentation/pages/extension_center_page.dart';
import '../../../features/extensions/presentation/pages/extension_page_host_page.dart';
import '../../../features/tasks/presentation/pages/task_hub_page.dart';
import '../../../features/memory/presentation/pages/memory_hub_page.dart';
import '../../../features/profile/presentation/pages/character_world_book_hub_page.dart';
import '../../../features/profile/presentation/pages/profile_hub_page.dart';
import '../../../features/settings/presentation/pages/mobile_user_space_host.dart';
import '../../../features/settings/presentation/pages/settings_page.dart';
import '../../../features/settings/presentation/pages/storage_page.dart';
import '../../../features/settings/presentation/pages/long_running_page.dart';
import '../../../features/settings/presentation/pages/decision_viz_page.dart';
import '../../../features/runtime/presentation/pages/runtime_page.dart';
import '../../../features/toolbox/presentation/pages/toolbox_workspace_page.dart';
import '../../../features/reminders/presentation/pages/reminders_page.dart';
import '../../../features/continuity/presentation/pages/continuity_page.dart';
import '../../../features/continuity/presentation/pages/continuity_detail_page.dart';
import '../mobile_native_page_host.dart';
import '../../../core/widgets/main_tab_scaffold.dart';
import '../provider_tab_host.dart';

MobileNativePageHost _nativePage(GoRouterState state) => MobileNativePageHost(
  context: {
    'route': state.uri.toString(),
    'pathParameters': state.pathParameters,
    'queryParameters': state.uri.queryParameters,
  },
);

MobileUserSpaceHost _userSpacePage(GoRouterState state) => MobileUserSpaceHost(
  context: {
    'route': state.uri.toString(),
    'queryParameters': state.uri.queryParameters,
  },
);

/// All built-in business routes live in this catalog instead of the app
/// bootstrap router. The router only owns recovery/authentication boundaries,
/// the shell, and extension supplied routes.
List<RouteBase> buildBuiltinBusinessRoutes() => <RouteBase>[
  StatefulShellRoute.indexedStack(
    pageBuilder: (context, state, navigationShell) => AmitiaSlidePage<void>(
      key: state.pageKey,
      rootLevel: true,
      child: MainTabScaffold(navigationShell: navigationShell),
    ),
    branches: <StatefulShellBranch>[
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.conversations,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ConversationListPage(),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.tasks,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ProviderTabHost(
                routePath: AppRoutes.tasks,
                fallback: TaskHubPage(),
              ),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.memoryHub,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ProviderTabHost(
                routePath: AppRoutes.memoryHub,
                fallback: MemoryHubPage(),
              ),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.extensions,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ProviderTabHost(
                routePath: AppRoutes.extensions,
                fallback: ExtensionCenterPage(),
              ),
            ),
          ),
        ],
      ),
      StatefulShellBranch(
        routes: <RouteBase>[
          GoRoute(
            path: AppRoutes.me,
            pageBuilder: (context, state) => amitiaSlidePage(
              context: context,
              state: state,
              rootLevel: true,
              child: const ProfileHubPage(),
            ),
          ),
        ],
      ),
    ],
  ),
  GoRoute(
    path: AppRoutes.conversationSearch,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ConversationSearchPage(),
    ),
  ),
  GoRoute(
    path: AppRoutes.meCharacters,
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const CharacterWorldBookHubPage()),
  ),
  GoRoute(
    path: AppRoutes.settings,
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _userSpacePage(state)),
  ),
  GoRoute(
    path: '/settings/category/:categoryId',
    pageBuilder: (context, state) => amitiaSlidePage(
      state: state,
      child: SettingsCategoryPage(
        categoryId: state.pathParameters['categoryId']!,
      ),
    ),
  ),
  GoRoute(
    path: '/settings/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _userSpacePage(state)),
  ),
  GoRoute(
    path: AppRoutes.chatLogs,
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ChatLogsPage()),
  ),
  GoRoute(
    path: '/characters',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/characters/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/memory',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/memory/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  for (final path in <String>[
    '/knowledge',
    '/profiles',
    '/episodic',
    '/graph',
    '/memory-manager',
    '/memory-timeline',
    '/world-book',
  ])
    GoRoute(
      path: path,
      pageBuilder: (context, state) =>
          amitiaSlidePage(state: state, child: _nativePage(state)),
    ),
  GoRoute(
    path: '/reminders',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const RemindersPage()),
  ),
  GoRoute(
    path: '/continuity',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ContinuityPage()),
  ),
  GoRoute(
    path: '/continuity/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      state: state,
      child: ContinuityDetailPage(threadId: state.pathParameters['id']!),
    ),
  ),
  GoRoute(
    path: '/workshop',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/workshop/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/game-center',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/game-center/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/channel-center',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/chat-import',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/developer',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/developer/:pathMatch(.*)*',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: _nativePage(state)),
  ),
  GoRoute(
    path: '/workspaces',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const ToolboxWorkspacePage()),
  ),
  GoRoute(
    path: '/storage',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const StoragePage()),
  ),
  GoRoute(
    path: '/runtime-debug',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const RuntimePage()),
  ),
  GoRoute(
    path: '/decision-viz',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const DecisionVizPage()),
  ),
  GoRoute(
    path: '/long-running',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const LongRunningPage()),
  ),
  GoRoute(
    path: '/chat',
    pageBuilder: (context, state) => amitiaSlidePage(
      state: state,
      child: ChatPage(
        initialConversationId: state.uri.queryParameters['conversationId'],
        initialCharacterId: state.uri.queryParameters['characterId'],
        initialProjectId: state.uri.queryParameters['projectId'],
        initialDraftId: state.uri.queryParameters['draftId'],
      ),
    ),
  ),
  GoRoute(
    path: '/call/:conversationId',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: RealtimeCallPage(
        conversationId: state.pathParameters['conversationId']!,
        characterName: state.uri.queryParameters['caller'] ?? 'Amitia',
        initialMode: switch (state.uri.queryParameters['type']) {
          'video' => RealtimeCallMode.video,
          'screen' => RealtimeCallMode.screen,
          _ => RealtimeCallMode.voice,
        },
        incomingCallId: state.uri.queryParameters['call'] ?? '',
        initialAction: state.uri.queryParameters['action'] ?? '',
        characterId: state.uri.queryParameters['characterId'] ?? '',
        ownedInvitation: state.uri.queryParameters['owned'] == '1',
      ),
    ),
  ),
  GoRoute(path: '/dashboard', redirect: (_, _) => AppRoutes.settingsOverview),
  GoRoute(
    path: '/agent',
    pageBuilder: (context, state) =>
        amitiaSlidePage(state: state, child: const AgentPage()),
  ),
  GoRoute(
    path: '/agent/task/:id',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: AgentTaskDetailPage(
        taskId: state.pathParameters['id']!,
        initialRun: state.extra is Map
            ? Map<String, dynamic>.from(state.extra as Map)
            : null,
      ),
    ),
  ),
  GoRoute(
    path: AppRoutes.channelMessages,
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: const ChannelMessagesPage(),
    ),
  ),
  GoRoute(
    path: '/extension/page/:pageId',
    pageBuilder: (context, state) => amitiaSlidePage(
      context: context,
      state: state,
      child: ExtensionPageHostPage(
        pageId: state.pathParameters['pageId']!,
        extensionId: state.uri.queryParameters['extensionId'] ?? '',
      ),
    ),
  ),
];
