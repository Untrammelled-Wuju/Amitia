import 'dart:convert';

import '../../../core/models/conversation.dart';

class AgentUIEvent {
  final int version;
  final String eventId;
  final int eventSequence;
  final String conversationId;
  final String requestId;
  final String executionId;
  final String turnId;
  final String parentTurnId;
  final String parentBlockId;
  final String agentId;
  final int turnSequence;
  final String blockId;
  final int blockSequence;
  final String messageId;
  final int messageSequence;
  final String callId;
  final int revision;
  final String type;
  final String status;
  final Map<String, dynamic> payload;
  final String createdAt;

  const AgentUIEvent({
    required this.version,
    required this.eventId,
    required this.eventSequence,
    required this.conversationId,
    required this.requestId,
    required this.executionId,
    required this.turnId,
    this.parentTurnId = '',
    this.parentBlockId = '',
    this.agentId = '',
    required this.turnSequence,
    required this.blockId,
    required this.blockSequence,
    required this.messageId,
    required this.messageSequence,
    required this.callId,
    required this.revision,
    required this.type,
    required this.status,
    required this.payload,
    required this.createdAt,
  });

  factory AgentUIEvent.fromJson(Map<String, dynamic> json) {
    final rawPayload = json['payload'];
    return AgentUIEvent(
      version: (json['version'] as num?)?.toInt() ?? 0,
      eventId: (json['eventId'] ?? '').toString(),
      eventSequence: (json['eventSequence'] as num?)?.toInt() ?? 0,
      conversationId: (json['conversationId'] ?? '').toString(),
      requestId: (json['requestId'] ?? '').toString(),
      executionId: (json['executionId'] ?? '').toString(),
      turnId: (json['turnId'] ?? '').toString(),
      parentTurnId: (json['parentTurnId'] ?? '').toString(),
      parentBlockId: (json['parentBlockId'] ?? '').toString(),
      agentId: (json['agentId'] ?? '').toString(),
      turnSequence: (json['turnSequence'] as num?)?.toInt() ?? 0,
      blockId: (json['blockId'] ?? '').toString(),
      blockSequence: (json['blockSequence'] as num?)?.toInt() ?? 0,
      messageId: (json['messageId'] ?? '').toString(),
      messageSequence: (json['messageSequence'] as num?)?.toInt() ?? 0,
      callId: (json['callId'] ?? '').toString(),
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      type: (json['type'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      payload: rawPayload is Map
          ? Map<String, dynamic>.from(rawPayload)
          : const <String, dynamic>{},
      createdAt: (json['createdAt'] ?? '').toString(),
    );
  }
}

enum AgentEventApplyResult { applied, duplicate, stale, gap, unsupported }

class AgentEventReducer {
  final Map<String, AssistantTurnDto> _turns = <String, AssistantTurnDto>{};
  final Set<String> _seenEventIds = <String>{};
  final List<String> _seenEventOrder = <String>[];
  int _lastEventSequence = 0;
  String _activeTurnId = '';
  String _activeExecutionId = '';

  int get lastEventSequence => _lastEventSequence;
  String get activeTurnId => _activeTurnId;
  String get activeExecutionId => _activeExecutionId;

  List<AssistantTurnDto> get turns {
    final result = _turns.values.toList(growable: false);
    result.sort((left, right) => left.sequence.compareTo(right.sequence));
    return result;
  }

  AssistantTurnDto? turn(String id) => _turns[id.trim()];

  void reset({
    required List<AssistantTurnDto> turns,
    required int lastEventSequence,
    Map<String, dynamic>? activeTurn,
  }) {
    _turns
      ..clear()
      ..addEntries(turns.map((turn) => MapEntry(turn.id, turn)));
    _seenEventIds.clear();
    _seenEventOrder.clear();
    _lastEventSequence = lastEventSequence;
    _activeTurnId = '';
    _activeExecutionId = '';
    if (activeTurn != null) {
      final turn = _turnFromRuntime(activeTurn);
      if (turn != null) {
        _turns[turn.id] = turn;
        _activeTurnId = turn.id;
        _activeExecutionId = turn.executionId;
      }
    } else {
      for (final turn in this.turns.reversed) {
        if (!_isTerminal(turn.status)) {
          _activeTurnId = turn.id;
          _activeExecutionId = turn.executionId;
          break;
        }
      }
    }
  }

  void mergeTurns(Iterable<AssistantTurnDto> turns) {
    for (final turn in turns) {
      final current = _turns[turn.id];
      if (current == null || _turnFreshness(turn) >= _turnFreshness(current)) {
        _turns[turn.id] = turn;
      }
    }
  }

  AgentEventApplyResult apply(AgentUIEvent event) {
    if (event.version != 1) return AgentEventApplyResult.unsupported;
    if (event.eventId.isNotEmpty && _seenEventIds.contains(event.eventId)) {
      return AgentEventApplyResult.duplicate;
    }
    if (event.eventSequence > 0) {
      if (_lastEventSequence > 0 &&
          event.eventSequence > _lastEventSequence + 1 &&
          event.payload['recoveryCheckpoint'] != true) {
        return AgentEventApplyResult.gap;
      }
      if (event.eventSequence <= _lastEventSequence) {
        _remember(event.eventId);
        return AgentEventApplyResult.stale;
      }
    }
    if (event.eventId.isNotEmpty) _remember(event.eventId);
    if (event.eventSequence > _lastEventSequence) {
      _lastEventSequence = event.eventSequence;
    }
    final turnId = event.turnId.trim();
    if (turnId.isEmpty) return AgentEventApplyResult.applied;
    var turn = _turns[turnId] ?? _newTurn(event);
    final type = event.type.trim();
    if (type == 'turn.queued' ||
        type == 'turn.started' ||
        type == 'turn.cancelling' ||
        type == 'turn.steered' ||
        type == 'approval.requested' ||
        type == 'approval.approved' ||
        type == 'approval.denied' ||
        type == 'approval.expired') {
      var nextStatus = _normalizeStatus(event.status.isNotEmpty ? event.status : turn.status);
      if (type == 'turn.queued') nextStatus = 'queued';
      if (type == 'turn.started') nextStatus = 'running';
      if (type == 'turn.cancelling') nextStatus = 'cancelling';
      if (type == 'approval.requested') nextStatus = 'waiting_approval';
      if (type == 'approval.approved' || type == 'approval.denied' || type == 'approval.expired') {
        nextStatus = 'running';
      }
      turn = _copyTurn(
        turn,
        requestId: event.requestId.isNotEmpty ? event.requestId : null,
        executionId: event.executionId.isNotEmpty ? event.executionId : null,
        sequence: event.turnSequence > 0 ? event.turnSequence : null,
        status: nextStatus,
        updatedAt: event.createdAt,
      );
      _turns[turnId] = turn;
      if (!_isTerminal(nextStatus)) {
        _activeTurnId = turnId;
        _activeExecutionId = turn.executionId;
      }
      return AgentEventApplyResult.applied;
    }
    if (type == 'turn.completed' || type == 'turn.failed' || type == 'turn.interrupted') {
      final terminalStatus = type == 'turn.completed'
          ? 'completed'
          : type == 'turn.failed'
          ? 'failed'
          : 'interrupted';
      final items = turn.items.map((item) {
        final normalized = item.status.trim().toLowerCase();
        if (terminalStatus != 'completed' &&
            normalized != 'completed' &&
            normalized != 'failed' &&
            normalized != 'interrupted') {
          return _copyItem(item, status: terminalStatus);
        }
        return item;
      }).toList();
      if (terminalStatus == 'failed' &&
          !items.any((item) => item.type == 'error')) {
        items.add(
          AssistantTurnItemDto(
            id: 'turn-error:$turnId',
            turnId: turnId,
            conversationId: turn.conversationId,
            sequence: 1 << 30,
            type: 'error',
            status: 'failed',
            revision: 1,
            content: (event.payload['userMessage'] ?? '').toString(),
            resultJson: jsonEncode(event.payload),
            errorCode: (event.payload['errorCode'] ?? '').toString(),
            createdAt: event.createdAt,
            updatedAt: event.createdAt,
          ),
        );
      }
      final messageId = event.messageId.trim();
      if (messageId.isNotEmpty) {
        for (var index = items.length - 1; index >= 0; index -= 1) {
          if (items[index].type == 'text') {
            items[index] = _copyItem(
              items[index],
              messageId: messageId,
              isFinal: terminalStatus == 'completed',
              status: terminalStatus == 'completed' ? 'completed' : items[index].status,
            );
            break;
          }
        }
      }
      turn = _copyTurn(
        turn,
        requestId: event.requestId.isNotEmpty ? event.requestId : null,
        executionId: event.executionId.isNotEmpty ? event.executionId : null,
        sequence: event.turnSequence > 0 ? event.turnSequence : null,
        status: terminalStatus,
        updatedAt: event.createdAt,
        completedAt: event.createdAt,
        items: items,
      );
      _turns[turnId] = turn;
      if (_activeTurnId == turnId) {
        _activeTurnId = '';
        _activeExecutionId = '';
      }
      return AgentEventApplyResult.applied;
    }
    if (event.blockId.trim().isEmpty) {
      _turns[turnId] = _copyTurn(turn, updatedAt: event.createdAt);
      return AgentEventApplyResult.applied;
    }
    final items = List<AssistantTurnItemDto>.from(turn.items);
    final blockId = event.blockId.trim();
    final index = items.indexWhere((item) => item.id == blockId);
    final current = index >= 0 ? items[index] : null;
    if (current != null && event.revision > 0 && current.revision > event.revision) {
      return AgentEventApplyResult.stale;
    }
    var item = current ?? _newItem(turn, event);
    final payload = event.payload;
    final delta = (payload['delta'] ?? '').toString();
    var content = item.content;
    var arguments = item.argumentsJson;
    var result = item.resultJson;
    if (type.endsWith('.delta') && delta.isNotEmpty) {
      if (type == 'tool.arguments.delta' || (payload['field'] ?? '').toString() == 'arguments') {
        arguments += delta;
      } else {
        content += delta;
      }
    }
    if (!type.endsWith('.delta') && payload.containsKey('content')) {
      content = (payload['content'] ?? '').toString();
    }
    if (type == 'tool.arguments.completed' || payload.containsKey('arguments')) {
      arguments = (payload['arguments'] ?? arguments).toString();
    }
    if (payload.containsKey('result')) {
      final raw = payload['result'];
      result = raw is String ? raw : raw?.toString() ?? '';
    }
    final blockStatus = _blockStatus(type, event.status, item.status);
    item = _copyItem(
      item,
      sequence: event.blockSequence > 0 ? event.blockSequence : null,
      type: _itemType(event),
      status: blockStatus,
      revision: event.revision > 0 ? event.revision : item.revision,
      callId: event.callId.isNotEmpty ? event.callId : null,
      toolName: (payload['toolName'] ?? '').toString().trim().isNotEmpty
          ? (payload['toolName'] ?? '').toString()
          : null,
      content: content,
      argumentsJson: arguments,
      resultJson: result,
      errorCode: payload.containsKey('errorCode')
          ? (payload['errorCode'] ?? '').toString()
          : null,
      durationMs: payload['durationMs'] is num
          ? (payload['durationMs'] as num).toInt()
          : null,
      isFinal: type == 'text.completed' || item.isFinal,
      messageId: event.messageId.isNotEmpty ? event.messageId : null,
      updatedAt: event.createdAt,
    );
    if (index >= 0) {
      items[index] = item;
    } else {
      items.add(item);
      items.sort((left, right) => left.sequence.compareTo(right.sequence));
    }
    var nextTurnStatus = turn.status;
    if (!_isTerminal(nextTurnStatus) && nextTurnStatus != 'waiting_approval' && nextTurnStatus != 'cancelling') {
      final hasRunningTool = items.any((candidate) =>
          candidate.type == 'tool_call' &&
          candidate.status != 'completed' &&
          candidate.status != 'failed' &&
          candidate.status != 'interrupted');
      if (hasRunningTool) {
        nextTurnStatus = 'waiting_tool';
      } else if (nextTurnStatus == 'waiting_tool') {
        nextTurnStatus = 'running';
      }
    }
    turn = _copyTurn(
      turn,
      requestId: event.requestId.isNotEmpty ? event.requestId : null,
      executionId: event.executionId.isNotEmpty ? event.executionId : null,
      sequence: event.turnSequence > 0 ? event.turnSequence : null,
      status: nextTurnStatus,
      updatedAt: event.createdAt,
      items: items,
    );
    _turns[turnId] = turn;
    if (!_isTerminal(turn.status)) {
      _activeTurnId = turn.id;
      _activeExecutionId = turn.executionId;
    }
    return AgentEventApplyResult.applied;
  }

  AssistantTurnDto _newTurn(AgentUIEvent event) {
    final type = event.type.trim();
    final status = switch (type) {
      'turn.queued' => 'queued',
      'turn.started' => 'running',
      'turn.cancelling' => 'cancelling',
      'turn.completed' => 'completed',
      'turn.failed' => 'failed',
      'turn.interrupted' => 'interrupted',
      'approval.requested' => 'waiting_approval',
      _ => 'running',
    };
    return AssistantTurnDto(
      id: event.turnId,
      conversationId: event.conversationId,
      requestId: event.requestId,
      executionId: event.executionId,
      parentTurnId: event.parentTurnId,
      parentBlockId: event.parentBlockId,
      agentId: event.agentId,
      sequence: event.turnSequence,
      status: status,
      createdAt: event.createdAt,
      updatedAt: event.createdAt,
      items: const <AssistantTurnItemDto>[],
    );
  }

  AssistantTurnItemDto _newItem(AssistantTurnDto turn, AgentUIEvent event) {
    return AssistantTurnItemDto(
      id: event.blockId,
      turnId: turn.id,
      conversationId: turn.conversationId,
      sequence: event.blockSequence,
      type: _itemType(event),
      status: event.status.isEmpty ? 'running' : event.status,
      revision: event.revision,
      callId: event.callId,
      toolName: (event.payload['toolName'] ?? '').toString(),
      createdAt: event.createdAt,
      updatedAt: event.createdAt,
    );
  }

  AssistantTurnDto? _turnFromRuntime(Map<String, dynamic> raw) {
    final id = (raw['turnId'] ?? '').toString().trim();
    if (id.isEmpty) return null;
    final rawBlocks = raw['blocks'];
    final items = <AssistantTurnItemDto>[];
    if (rawBlocks is List) {
      for (final value in rawBlocks.whereType<Map>()) {
        final block = Map<String, dynamic>.from(value);
        final payload = block['payload'] is Map
            ? Map<String, dynamic>.from(block['payload'] as Map)
            : const <String, dynamic>{};
        final type = (block['type'] ?? '').toString();
        items.add(
          AssistantTurnItemDto(
            id: (block['blockId'] ?? '').toString(),
            turnId: id,
            conversationId: '',
            sequence: (block['blockSequence'] as num?)?.toInt() ?? 0,
            type: type,
            status: (block['status'] ?? 'running').toString(),
            revision: (block['revision'] as num?)?.toInt() ?? 0,
            callId: (block['callId'] ?? '').toString(),
            toolName: (payload['toolName'] ?? '').toString(),
            content: (block['content'] ?? '').toString(),
            argumentsJson: (payload['arguments'] ?? '').toString(),
            resultJson: (payload['result'] ?? '').toString(),
            errorCode: (payload['errorCode'] ?? '').toString(),
            durationMs: (payload['durationMs'] as num?)?.toInt() ?? 0,
            createdAt: (raw['startedAt'] ?? '').toString(),
            updatedAt: (raw['lastModifiedAt'] ?? '').toString(),
          ),
        );
      }
    }
    items.sort((left, right) => left.sequence.compareTo(right.sequence));
    return AssistantTurnDto(
      id: id,
      requestId: (raw['requestId'] ?? '').toString(),
      executionId: (raw['executionId'] ?? '').toString(),
      parentTurnId: (raw['parentTurnId'] ?? '').toString(),
      parentBlockId: (raw['parentBlockId'] ?? '').toString(),
      agentId: (raw['agentId'] ?? '').toString(),
      sequence: (raw['turnSequence'] as num?)?.toInt() ?? 0,
      status: _normalizeStatus((raw['status'] ?? 'running').toString()),
      createdAt: (raw['startedAt'] ?? '').toString(),
      updatedAt: (raw['lastModifiedAt'] ?? '').toString(),
      items: items,
    );
  }

  String _itemType(AgentUIEvent event) {
    var type = (event.payload['blockType'] ?? '').toString().trim();
    if (type.isEmpty) {
      type = event.type.split('.').first;
    }
    if (type == 'tool') return 'tool_call';
    return type;
  }

  String _blockStatus(String type, String eventStatus, String current) {
    if (type == 'tool.arguments.completed') {
      return eventStatus.isNotEmpty ? eventStatus : (current.isEmpty ? 'running' : current);
    }
    if (type.endsWith('.failed')) return 'failed';
    if (type.endsWith('.interrupted')) return 'interrupted';
    if (type.endsWith('.completed')) return 'completed';
    if (type == 'tool.running') return 'running';
    if (eventStatus.isNotEmpty) return eventStatus;
    return current.isEmpty ? 'running' : current;
  }

  AssistantTurnDto _copyTurn(
    AssistantTurnDto turn, {
    String? requestId,
    String? executionId,
    int? sequence,
    String? status,
    String? updatedAt,
    String? completedAt,
    List<AssistantTurnItemDto>? items,
  }) {
    return AssistantTurnDto(
      id: turn.id,
      conversationId: turn.conversationId,
      characterId: turn.characterId,
      userMessageId: turn.userMessageId,
      requestId: requestId ?? turn.requestId,
      executionId: executionId ?? turn.executionId,
      parentTurnId: turn.parentTurnId,
      parentBlockId: turn.parentBlockId,
      agentId: turn.agentId,
      sequence: sequence ?? turn.sequence,
      status: status ?? turn.status,
      createdAt: turn.createdAt,
      updatedAt: updatedAt?.isNotEmpty == true ? updatedAt! : turn.updatedAt,
      completedAt: completedAt?.isNotEmpty == true ? completedAt! : turn.completedAt,
      items: items ?? turn.items,
    );
  }

  AssistantTurnItemDto _copyItem(
    AssistantTurnItemDto item, {
    int? sequence,
    String? type,
    String? status,
    int? revision,
    String? callId,
    String? toolName,
    String? content,
    String? argumentsJson,
    String? resultJson,
    String? errorCode,
    int? durationMs,
    bool? isFinal,
    String? messageId,
    String? updatedAt,
  }) {
    return AssistantTurnItemDto(
      id: item.id,
      turnId: item.turnId,
      conversationId: item.conversationId,
      sequence: sequence ?? item.sequence,
      type: type ?? item.type,
      status: status ?? item.status,
      revision: revision ?? item.revision,
      callId: callId ?? item.callId,
      toolName: toolName ?? item.toolName,
      content: content ?? item.content,
      argumentsJson: argumentsJson ?? item.argumentsJson,
      resultJson: resultJson ?? item.resultJson,
      errorCode: errorCode ?? item.errorCode,
      durationMs: durationMs ?? item.durationMs,
      isFinal: isFinal ?? item.isFinal,
      messageId: messageId ?? item.messageId,
      createdAt: item.createdAt,
      updatedAt: updatedAt?.isNotEmpty == true ? updatedAt! : item.updatedAt,
    );
  }

  int _turnFreshness(AssistantTurnDto turn) {
    var value = turn.sequence * 1000000;
    for (final item in turn.items) {
      value += item.revision;
    }
    return value;
  }

  void _remember(String id) {
    if (id.isEmpty || _seenEventIds.contains(id)) return;
    _seenEventIds.add(id);
    _seenEventOrder.add(id);
    if (_seenEventOrder.length > 8192) {
      final removed = _seenEventOrder.sublist(0, 2048);
      _seenEventOrder.removeRange(0, 2048);
      _seenEventIds.removeAll(removed);
    }
  }

  bool _isTerminal(String status) {
    final value = _normalizeStatus(status);
    return value == 'completed' || value == 'failed' || value == 'interrupted';
  }

  String _normalizeStatus(String status) {
    final value = status.trim().toLowerCase();
    return value;
  }
}
