import 'dart:async';

/// Stable action names shared by Flutter native/schema/web providers and the
/// Web Vue host. Extensions should bind to these names rather than application
/// implementation details.
abstract final class ConversationUIAction {
  static const send = 'conversation.send';
  static const retry = 'conversation.retry';
  static const regenerate = 'conversation.regenerate';
  static const stop = 'conversation.stop';
  static const delete = 'conversation.delete';
  static const newConversation = 'conversation.new';
  static const openDrawer = 'conversation.openDrawer';
  static const clear = 'conversation.clear';
  static const reply = 'conversation.reply';
  static const sendFile = 'conversation.sendFile';
  static const sendImage = 'conversation.sendImage';
  static const sendCode = 'conversation.sendCode';
  static const sendVoice = 'conversation.sendVoice';
  static const sendEmote = 'conversation.sendEmote';
}

typedef UIActionHandler = FutureOr<dynamic> Function(dynamic input);
