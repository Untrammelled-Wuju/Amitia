import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../ui_runtime/mobile_style_runtime.dart';

@immutable
class UIStylePatchState {
  final int revision;
  final Map<String, dynamic> patch;
  final List<AmitiaStyleRule> rules;
  final List<AmitiaStyleTransaction> transactions;

  const UIStylePatchState({
    this.revision = 0,
    this.patch = const <String, dynamic>{},
    this.rules = const <AmitiaStyleRule>[],
    this.transactions = const <AmitiaStyleTransaction>[],
  });

  AmitiaStyleDocument get document => AmitiaStyleDocument(
    revision: revision,
    rules: rules,
    transactions: transactions,
  );
}

final uiStylePatchProvider =
    StateNotifierProvider<UIStylePatchNotifier, UIStylePatchState>(
      (ref) => UIStylePatchNotifier(),
    );

final amitiaStyleDocumentProvider = Provider<AmitiaStyleDocument>((ref) {
  return ref.watch(uiStylePatchProvider).document;
});

class UIStylePatchNotifier extends StateNotifier<UIStylePatchState> {
  UIStylePatchNotifier() : super(const UIStylePatchState());

  static const _patchKey = 'ui.style.patch.v1';
  static const _revisionKey = 'ui.style.revision.v1';
  static const _historyKey = 'ui.style.history.v1';
  static const _rulesKey = 'ui.style.rules.v1';
  static const _transactionsKey = 'ui.style.transactions.v1';
  static const maxHistory = 30;

  bool _initialized = false;

  Future<void> init() async {
    if (_initialized) return;
    final prefs = await SharedPreferences.getInstance();
    Map<String, dynamic> patch = const <String, dynamic>{};
    try {
      final decoded = jsonDecode(prefs.getString(_patchKey) ?? '{}');
      if (decoded is Map) patch = decoded.cast<String, dynamic>();
    } catch (_) {}
    var rules = const <AmitiaStyleRule>[];
    try {
      final decoded = jsonDecode(prefs.getString(_rulesKey) ?? '[]');
      if (decoded is List) {
        rules = decoded
            .whereType<Map>()
            .map(
              (item) => AmitiaStyleRule.fromMap(item.cast<String, dynamic>()),
            )
            .toList(growable: false);
      }
    } catch (_) {}
    var transactions = const <AmitiaStyleTransaction>[];
    try {
      final decoded = jsonDecode(prefs.getString(_transactionsKey) ?? '[]');
      if (decoded is List) {
        transactions = decoded
            .whereType<Map>()
            .map(
              (item) =>
                  AmitiaStyleTransaction.fromMap(item.cast<String, dynamic>()),
            )
            .toList(growable: false);
      }
    } catch (_) {}
    state = UIStylePatchState(
      revision: (prefs.getInt(_revisionKey) ?? 0).clamp(0, 1 << 31).toInt(),
      patch: patch,
      rules: rules,
      transactions: transactions,
    );
    _initialized = true;
  }

  Future<void> replace(
    Map<String, dynamic> patch,
    int revision, {
    List<AmitiaStyleRule>? rules,
    List<AmitiaStyleTransaction>? transactions,
  }) async {
    final normalized = patch.map((key, value) => MapEntry(key, value));
    final nextRules = rules ?? state.rules;
    final nextTransactions = transactions ?? state.transactions;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_patchKey, jsonEncode(normalized));
    await prefs.setInt(_revisionKey, revision);
    if (rules != null) {
      await prefs.setString(
        _rulesKey,
        jsonEncode(
          nextRules.map((rule) => rule.toMap()).toList(growable: false),
        ),
      );
    }
    if (transactions != null) {
      await prefs.setString(
        _transactionsKey,
        jsonEncode(
          nextTransactions
              .map((transaction) => transaction.toMap())
              .toList(growable: false),
        ),
      );
    }
    if (mounted) {
      state = UIStylePatchState(
        revision: revision,
        patch: normalized,
        rules: nextRules,
        transactions: nextTransactions,
      );
    }
  }

  Future<void> replaceDocument(
    AmitiaStyleDocument document,
    Map<String, dynamic> patch,
  ) => replace(
    patch,
    document.revision,
    rules: document.rules,
    transactions: document.transactions,
  );

  Future<void> clear(int revision) => replace(
    const <String, dynamic>{},
    revision,
    rules: const <AmitiaStyleRule>[],
    transactions: const <AmitiaStyleTransaction>[],
  );

  Future<List<Map<String, dynamic>>> loadHistory() async {
    await init();
    final prefs = await SharedPreferences.getInstance();
    try {
      final decoded = jsonDecode(prefs.getString(_historyKey) ?? '[]');
      if (decoded is! List) return <Map<String, dynamic>>[];
      return decoded
          .whereType<Map>()
          .map((item) => item.cast<String, dynamic>())
          .toList(growable: true);
    } catch (_) {
      return <Map<String, dynamic>>[];
    }
  }

  Future<void> pushHistory(Map<String, dynamic> snapshot) async {
    final history = await loadHistory();
    history.add(snapshot);
    while (history.length > maxHistory) {
      history.removeAt(0);
    }
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_historyKey, jsonEncode(history));
  }

  Future<Map<String, dynamic>?> popHistory() async {
    final history = await loadHistory();
    if (history.isEmpty) return null;
    final snapshot = history.removeLast();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_historyKey, jsonEncode(history));
    return snapshot;
  }
}
