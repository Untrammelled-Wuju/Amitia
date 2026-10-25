import 'dart:math' as math;

import 'package:flutter/material.dart';

enum AmitiaIconGlyph {
  messages,
  tasks,
  memory,
  plugins,
  profile,
  add,
  search,
  back,
  chevronRight,
  more,
  newChat,
  newGroup,
  reminder,
  continuity,
  memoryOverview,
  episodicMemory,
  memoryGraph,
  memoryTimeline,
  userProfile,
  worldBook,
  personalSpace,
  characters,
  workshop,
  game,
  channels,
  settings,
  package,
  mcp,
  skill,
  workflow,
  edit,
  archive,
  pin,
  delete,
  update,
  warning,
  member,
  workspace,
  phone,
  close,
  check,
  refresh,
  folder,
  newFolder,
  image,
  video,
  attachment,
  code,
  lock,
  unlock,
  microphone,
  keyboard,
  send,
  copy,
  download,
  screenShare,
  cleanup,
  page,
}

class AmitiaIcon extends StatelessWidget {
  const AmitiaIcon(
    this.glyph, {
    super.key,
    this.size = 20,
    this.color,
    this.strokeWidth = 1.8,
    this.semanticLabel,
  });

  final AmitiaIconGlyph glyph;
  final double size;
  final Color? color;
  final double strokeWidth;
  final String? semanticLabel;

  static AmitiaIconGlyph? tryFromMaterial(IconData icon) {
    return switch (icon) {
      Icons.add || Icons.add_rounded => AmitiaIconGlyph.add,
      Icons.search || Icons.search_rounded => AmitiaIconGlyph.search,
      Icons.arrow_back_ios_new ||
      Icons.arrow_back_ios_new_rounded ||
      Icons.arrow_back_rounded => AmitiaIconGlyph.back,
      Icons.chevron_right ||
      Icons.chevron_right_rounded => AmitiaIconGlyph.chevronRight,
      Icons.more_horiz ||
      Icons.more_horiz_rounded ||
      Icons.more_vert ||
      Icons.more_vert_rounded => AmitiaIconGlyph.more,
      Icons.close || Icons.close_rounded => AmitiaIconGlyph.close,
      Icons.check || Icons.check_rounded => AmitiaIconGlyph.check,
      Icons.refresh || Icons.refresh_rounded => AmitiaIconGlyph.refresh,
      Icons.folder_outlined ||
      Icons.folder_open_outlined => AmitiaIconGlyph.folder,
      Icons.create_new_folder_outlined => AmitiaIconGlyph.newFolder,
      Icons.photo_library_outlined ||
      Icons.photo_camera_outlined ||
      Icons.image_outlined => AmitiaIconGlyph.image,
      Icons.video_library_outlined ||
      Icons.videocam_outlined ||
      Icons.videocam_off_outlined => AmitiaIconGlyph.video,
      Icons.attach_file_outlined => AmitiaIconGlyph.attachment,
      Icons.code || Icons.code_rounded => AmitiaIconGlyph.code,
      Icons.lock_outline_rounded || Icons.lock_outlined => AmitiaIconGlyph.lock,
      Icons.lock_open_rounded ||
      Icons.lock_open_outlined => AmitiaIconGlyph.unlock,
      Icons.mic_none_outlined ||
      Icons.mic_outlined => AmitiaIconGlyph.microphone,
      Icons.keyboard_outlined ||
      Icons.keyboard_alt_outlined => AmitiaIconGlyph.keyboard,
      Icons.arrow_upward_rounded || Icons.send_outlined => AmitiaIconGlyph.send,
      Icons.copy_outlined || Icons.copy_rounded => AmitiaIconGlyph.copy,
      Icons.file_download_outlined ||
      Icons.download_outlined ||
      Icons.download_rounded => AmitiaIconGlyph.download,
      Icons.screen_share_outlined ||
      Icons.screen_share_rounded => AmitiaIconGlyph.screenShare,
      Icons.cleaning_services_outlined ||
      Icons.delete_sweep_outlined => AmitiaIconGlyph.cleanup,
      Icons.chat_bubble_outline ||
      Icons.chat_bubble_outline_rounded => AmitiaIconGlyph.messages,
      Icons.task_alt_outlined || Icons.task_outlined => AmitiaIconGlyph.tasks,
      Icons.timeline_outlined ||
      Icons.timeline_rounded => AmitiaIconGlyph.memoryTimeline,
      Icons.auto_stories_outlined => AmitiaIconGlyph.episodicMemory,
      Icons.menu_book_outlined => AmitiaIconGlyph.worldBook,
      Icons.person_search_outlined => AmitiaIconGlyph.userProfile,
      Icons.account_circle_outlined => AmitiaIconGlyph.personalSpace,
      Icons.brush_outlined => AmitiaIconGlyph.workshop,
      Icons.sports_esports_outlined ||
      Icons.sports_esports_rounded => AmitiaIconGlyph.game,
      Icons.space_dashboard_outlined => AmitiaIconGlyph.workspace,
      Icons.phone_in_talk_outlined ||
      Icons.call ||
      Icons.call_outlined => AmitiaIconGlyph.phone,
      Icons.person_add_alt_1_outlined ||
      Icons.person_add_alt_1_rounded ||
      Icons.group_add_outlined => AmitiaIconGlyph.newGroup,
      Icons.add_comment_outlined => AmitiaIconGlyph.newChat,
      Icons.inventory_2_outlined ||
      Icons.archive_outlined => AmitiaIconGlyph.package,
      Icons.hub_outlined => AmitiaIconGlyph.mcp,
      Icons.auto_awesome_outlined ||
      Icons.auto_awesome => AmitiaIconGlyph.skill,
      Icons.account_tree_outlined ||
      Icons.schema_outlined => AmitiaIconGlyph.workflow,
      Icons.extension_outlined || Icons.extension => AmitiaIconGlyph.plugins,
      Icons.system_update_alt => AmitiaIconGlyph.update,
      Icons.warning_amber_outlined ||
      Icons.warning_amber => AmitiaIconGlyph.warning,
      Icons.person_outline || Icons.person_outlined => AmitiaIconGlyph.profile,
      Icons.people_outline ||
      Icons.people_outlined => AmitiaIconGlyph.characters,
      Icons.notifications_none ||
      Icons.notifications_outlined => AmitiaIconGlyph.reminder,
      Icons.checklist_rtl || Icons.checklist => AmitiaIconGlyph.continuity,
      Icons.psychology_outlined => AmitiaIconGlyph.memory,
      Icons.forum_outlined => AmitiaIconGlyph.channels,
      Icons.settings_outlined => AmitiaIconGlyph.settings,
      _ => null,
    };
  }

  static AmitiaIconGlyph fromMaterial(IconData icon) {
    return tryFromMaterial(icon) ?? AmitiaIconGlyph.page;
  }

  static Widget adaptive(
    IconData icon, {
    double size = 20,
    Color? color,
    double strokeWidth = 1.8,
    String? semanticLabel,
  }) {
    final glyph = tryFromMaterial(icon);
    if (glyph == null) {
      return Icon(icon, size: size, color: color, semanticLabel: semanticLabel);
    }
    return AmitiaIcon(
      glyph,
      size: size,
      color: color,
      strokeWidth: strokeWidth,
      semanticLabel: semanticLabel,
    );
  }

  @override
  Widget build(BuildContext context) {
    final resolvedColor = color ?? IconTheme.of(context).color ?? Colors.black;
    final icon = SizedBox.square(
      dimension: size,
      child: CustomPaint(
        painter: _AmitiaIconPainter(
          glyph: glyph,
          color: resolvedColor,
          strokeWidth: strokeWidth,
        ),
      ),
    );
    if (semanticLabel == null) return ExcludeSemantics(child: icon);
    return Semantics(label: semanticLabel, image: true, child: icon);
  }
}

class _AmitiaIconPainter extends CustomPainter {
  _AmitiaIconPainter({
    required this.glyph,
    required this.color,
    required this.strokeWidth,
  });

  final AmitiaIconGlyph glyph;
  final Color color;
  final double strokeWidth;

  @override
  void paint(Canvas canvas, Size size) {
    final scale = size.shortestSide / 24;
    canvas.save();
    canvas.translate(
      (size.width - 24 * scale) / 2,
      (size.height - 24 * scale) / 2,
    );
    canvas.scale(scale);
    final stroke = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = strokeWidth
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round;
    final fill = Paint()
      ..color = color
      ..style = PaintingStyle.fill;

    switch (glyph) {
      case AmitiaIconGlyph.messages:
        _messages(canvas, stroke);
      case AmitiaIconGlyph.tasks:
        _tasks(canvas, stroke);
      case AmitiaIconGlyph.memory:
        _memory(canvas, stroke, fill);
      case AmitiaIconGlyph.plugins:
        _plugins(canvas, stroke, fill);
      case AmitiaIconGlyph.profile:
        _profile(canvas, stroke);
      case AmitiaIconGlyph.add:
        _lines(canvas, stroke, [(12, 5, 12, 19), (5, 12, 19, 12)]);
      case AmitiaIconGlyph.search:
        _search(canvas, stroke);
      case AmitiaIconGlyph.back:
        _polyline(canvas, stroke, [
          const Offset(14.5, 5),
          const Offset(7.5, 12),
          const Offset(14.5, 19),
        ]);
      case AmitiaIconGlyph.chevronRight:
        _polyline(canvas, stroke, [
          const Offset(9.5, 5),
          const Offset(16.5, 12),
          const Offset(9.5, 19),
        ]);
      case AmitiaIconGlyph.more:
        for (final x in [6.0, 12.0, 18.0]) {
          canvas.drawCircle(Offset(x, 12), 1.25, fill);
        }
      case AmitiaIconGlyph.newChat:
        _newChat(canvas, stroke);
      case AmitiaIconGlyph.newGroup:
        _newGroup(canvas, stroke, fill);
      case AmitiaIconGlyph.reminder:
        _reminder(canvas, stroke, fill);
      case AmitiaIconGlyph.continuity:
        _continuity(canvas, stroke, fill);
      case AmitiaIconGlyph.memoryOverview:
        _memoryOverview(canvas, stroke, fill);
      case AmitiaIconGlyph.episodicMemory:
        _episodicMemory(canvas, stroke);
      case AmitiaIconGlyph.memoryGraph:
        _memoryGraph(canvas, stroke, fill);
      case AmitiaIconGlyph.memoryTimeline:
        _memoryTimeline(canvas, stroke, fill);
      case AmitiaIconGlyph.userProfile:
        _userProfile(canvas, stroke, fill);
      case AmitiaIconGlyph.worldBook:
        _worldBook(canvas, stroke);
      case AmitiaIconGlyph.personalSpace:
        _personalSpace(canvas, stroke);
      case AmitiaIconGlyph.characters:
        _characters(canvas, stroke);
      case AmitiaIconGlyph.workshop:
        _workshop(canvas, stroke, fill);
      case AmitiaIconGlyph.game:
        _game(canvas, stroke, fill);
      case AmitiaIconGlyph.channels:
        _channels(canvas, stroke);
      case AmitiaIconGlyph.settings:
        _settings(canvas, stroke, fill);
      case AmitiaIconGlyph.package:
        _package(canvas, stroke);
      case AmitiaIconGlyph.mcp:
        _mcp(canvas, stroke, fill);
      case AmitiaIconGlyph.skill:
        _skill(canvas, stroke);
      case AmitiaIconGlyph.workflow:
        _workflow(canvas, stroke, fill);
      case AmitiaIconGlyph.edit:
        _edit(canvas, stroke);
      case AmitiaIconGlyph.archive:
        _archive(canvas, stroke);
      case AmitiaIconGlyph.pin:
        _pin(canvas, stroke);
      case AmitiaIconGlyph.delete:
        _delete(canvas, stroke);
      case AmitiaIconGlyph.update:
        _update(canvas, stroke);
      case AmitiaIconGlyph.warning:
        _warning(canvas, stroke);
      case AmitiaIconGlyph.member:
        _member(canvas, stroke);
      case AmitiaIconGlyph.workspace:
        _workspace(canvas, stroke);
      case AmitiaIconGlyph.phone:
        _phone(canvas, stroke);
      case AmitiaIconGlyph.close:
        _lines(canvas, stroke, [
          (6.2, 6.2, 17.8, 17.8),
          (17.8, 6.2, 6.2, 17.8),
        ]);
      case AmitiaIconGlyph.check:
        _polyline(canvas, stroke, [
          const Offset(4.8, 12.4),
          const Offset(9.7, 17.2),
          const Offset(19.2, 7),
        ]);
      case AmitiaIconGlyph.refresh:
        _refresh(canvas, stroke);
      case AmitiaIconGlyph.folder:
        _folder(canvas, stroke);
      case AmitiaIconGlyph.newFolder:
        _folder(canvas, stroke);
        _lines(canvas, stroke, [(12, 9.5, 12, 15.5), (9, 12.5, 15, 12.5)]);
      case AmitiaIconGlyph.image:
        _image(canvas, stroke, fill);
      case AmitiaIconGlyph.video:
        _video(canvas, stroke, fill);
      case AmitiaIconGlyph.attachment:
        _attachment(canvas, stroke);
      case AmitiaIconGlyph.code:
        _code(canvas, stroke);
      case AmitiaIconGlyph.lock:
        _lock(canvas, stroke, locked: true);
      case AmitiaIconGlyph.unlock:
        _lock(canvas, stroke, locked: false);
      case AmitiaIconGlyph.microphone:
        _microphone(canvas, stroke);
      case AmitiaIconGlyph.keyboard:
        _keyboard(canvas, stroke, fill);
      case AmitiaIconGlyph.send:
        _send(canvas, stroke);
      case AmitiaIconGlyph.copy:
        _copy(canvas, stroke);
      case AmitiaIconGlyph.download:
        _download(canvas, stroke);
      case AmitiaIconGlyph.screenShare:
        _screenShare(canvas, stroke, fill);
      case AmitiaIconGlyph.cleanup:
        _cleanup(canvas, stroke, fill);
      case AmitiaIconGlyph.page:
        _page(canvas, stroke);
    }
    canvas.restore();
  }

  void _messages(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(6.2, 5.2)
      ..lineTo(17.8, 5.2)
      ..quadraticBezierTo(20.3, 5.2, 20.3, 7.7)
      ..lineTo(20.3, 14.4)
      ..quadraticBezierTo(20.3, 16.9, 17.8, 16.9)
      ..lineTo(10.3, 16.9)
      ..lineTo(6.4, 20)
      ..lineTo(6.4, 16.9)
      ..lineTo(6.2, 16.9)
      ..quadraticBezierTo(3.8, 16.9, 3.8, 14.4)
      ..lineTo(3.8, 7.7)
      ..quadraticBezierTo(3.8, 5.2, 6.2, 5.2);
    canvas.drawPath(path, stroke);
  }

  void _tasks(Canvas canvas, Paint stroke) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(4.2, 4.2, 15.6, 15.6),
        const Radius.circular(4.2),
      ),
      stroke,
    );
    _polyline(canvas, stroke, [
      const Offset(7.8, 12.1),
      const Offset(10.5, 14.7),
      const Offset(16.4, 8.9),
    ]);
  }

  void _memory(Canvas canvas, Paint stroke, Paint fill) {
    final path = Path()
      ..moveTo(6.1, 19.2)
      ..lineTo(6.1, 12.8)
      ..cubicTo(6.1, 8.4, 8.8, 5.1, 12.4, 5.1)
      ..cubicTo(16.2, 5.1, 19.1, 8.3, 19.1, 12.6)
      ..lineTo(19.1, 16.3)
      ..quadraticBezierTo(19.1, 19.2, 16.5, 19.2)
      ..lineTo(12.7, 19.2);
    canvas.drawPath(path, stroke);
    final inner = Path()
      ..moveTo(9.3, 12.2)
      ..cubicTo(9.3, 9.9, 10.6, 8.5, 12.6, 8.5)
      ..cubicTo(14.5, 8.5, 15.8, 9.8, 15.8, 11.8)
      ..cubicTo(15.8, 14.1, 14.4, 15.5, 12.3, 15.5);
    canvas.drawPath(inner, stroke);
    canvas.drawCircle(const Offset(12.2, 12), 1.15, fill);
  }

  void _plugins(Canvas canvas, Paint stroke, Paint fill) {
    final body = Path()
      ..moveTo(8.1, 6.2)
      ..lineTo(15.9, 6.2)
      ..quadraticBezierTo(18.2, 6.2, 18.2, 8.5)
      ..lineTo(18.2, 12.5)
      ..quadraticBezierTo(18.2, 15.3, 15.5, 15.3)
      ..lineTo(8.5, 15.3)
      ..quadraticBezierTo(5.8, 15.3, 5.8, 12.5)
      ..lineTo(5.8, 8.5)
      ..quadraticBezierTo(5.8, 6.2, 8.1, 6.2);
    canvas.drawPath(body, stroke);
    _lines(canvas, stroke, [(9.3, 3.6, 9.3, 6.4), (14.7, 3.6, 14.7, 6.4)]);
    final cord = Path()
      ..moveTo(12, 15.3)
      ..cubicTo(12, 17.5, 13.6, 18.2, 15.8, 18.2)
      ..cubicTo(18.2, 18.2, 19.4, 19.4, 19.4, 21);
    canvas.drawPath(cord, stroke);
    canvas.drawCircle(const Offset(19.4, 21), 0.85, fill);
  }

  void _profile(Canvas canvas, Paint stroke) {
    canvas.drawCircle(const Offset(12, 8.1), 3.2, stroke);
    final path = Path()
      ..moveTo(5.4, 19.2)
      ..cubicTo(5.7, 15.8, 8.2, 13.7, 12, 13.7)
      ..cubicTo(15.8, 13.7, 18.3, 15.8, 18.6, 19.2);
    canvas.drawPath(path, stroke);
  }

  void _search(Canvas canvas, Paint stroke) {
    canvas.drawCircle(const Offset(10.4, 10.4), 5.6, stroke);
    _lines(canvas, stroke, [(14.5, 14.5, 20, 20)]);
  }

  void _newChat(Canvas canvas, Paint stroke) {
    final box = Path()
      ..moveTo(9.4, 5.2)
      ..lineTo(6.6, 5.2)
      ..quadraticBezierTo(4.2, 5.2, 4.2, 7.6)
      ..lineTo(4.2, 17.4)
      ..quadraticBezierTo(4.2, 19.8, 6.6, 19.8)
      ..lineTo(16.4, 19.8)
      ..quadraticBezierTo(18.8, 19.8, 18.8, 17.4)
      ..lineTo(18.8, 14.7);
    canvas.drawPath(box, stroke);
    _lines(canvas, stroke, [
      (11.1, 13.5, 20.1, 4.5),
      (17.4, 4.8, 19.8, 7.2),
      (10.2, 17.2, 10.1, 14.6),
      (10.1, 14.6, 12.7, 14.5),
    ]);
  }

  void _newGroup(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawCircle(const Offset(8.1, 8.1), 2.8, stroke);
    final left = Path()
      ..moveTo(3.8, 19.2)
      ..cubicTo(4.1, 15.9, 5.7, 14.2, 8.2, 14.2)
      ..cubicTo(10.7, 14.2, 12.3, 15.9, 12.6, 19.2);
    canvas.drawPath(left, stroke);
    final right = Path()
      ..moveTo(14.4, 10.1)
      ..cubicTo(16.7, 9.8, 18.2, 11.1, 18.5, 13.2);
    canvas.drawPath(right, stroke);
    _lines(canvas, stroke, [(18.5, 4.2, 18.5, 9.2), (16, 6.7, 21, 6.7)]);
    canvas.drawCircle(const Offset(15.6, 8.2), .9, fill);
  }

  void _reminder(Canvas canvas, Paint stroke, Paint fill) {
    final path = Path()
      ..moveTo(6.2, 16.3)
      ..cubicTo(7.4, 14.8, 7.8, 13.2, 7.8, 10.9)
      ..cubicTo(7.8, 8.2, 9.5, 6.1, 12, 6.1)
      ..cubicTo(14.5, 6.1, 16.2, 8.2, 16.2, 10.9)
      ..cubicTo(16.2, 13.2, 16.6, 14.8, 17.8, 16.3);
    canvas.drawPath(path, stroke);
    _lines(canvas, stroke, [(5.5, 16.3, 18.5, 16.3)]);
    canvas.drawArc(
      const Rect.fromLTWH(10, 16, 4, 3),
      0,
      math.pi,
      false,
      stroke,
    );
    canvas.drawCircle(const Offset(12, 4.1), 1.05, fill);
  }

  void _continuity(Canvas canvas, Paint stroke, Paint fill) {
    final path = Path()
      ..moveTo(12, 12)
      ..cubicTo(9.8, 8.3, 4.2, 7.7, 4.2, 12)
      ..cubicTo(4.2, 16.3, 9.8, 15.7, 12, 12)
      ..cubicTo(14.2, 8.3, 19.8, 7.7, 19.8, 12)
      ..cubicTo(19.8, 16.3, 14.2, 15.7, 12, 12);
    canvas.drawPath(path, stroke);
    canvas.drawCircle(const Offset(12, 12), 1.05, fill);
  }

  void _memoryOverview(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawCircle(const Offset(12, 12), 7.2, stroke);
    canvas.drawCircle(const Offset(12, 12), 4.1, stroke);
    canvas.drawCircle(const Offset(12, 12), 1.2, fill);
    _lines(canvas, stroke, [(12, 2.8, 12, 4.8), (12, 19.2, 12, 21.2)]);
  }

  void _episodicMemory(Canvas canvas, Paint stroke) {
    final page = Path()
      ..moveTo(6.2, 5.1)
      ..lineTo(16.6, 5.1)
      ..quadraticBezierTo(18.8, 5.1, 18.8, 7.3)
      ..lineTo(18.8, 18.8)
      ..lineTo(7.1, 18.8)
      ..quadraticBezierTo(5.2, 18.8, 5.2, 16.9)
      ..lineTo(5.2, 6.1)
      ..quadraticBezierTo(5.2, 5.1, 6.2, 5.1);
    canvas.drawPath(page, stroke);
    _lines(canvas, stroke, [
      (8.2, 9, 15.7, 9),
      (8.2, 12.3, 15.7, 12.3),
      (8.2, 15.6, 13.1, 15.6),
    ]);
  }

  void _memoryGraph(Canvas canvas, Paint stroke, Paint fill) {
    const nodes = [
      Offset(5.2, 7.3),
      Offset(11.4, 11.2),
      Offset(18.8, 6.2),
      Offset(8.2, 18),
      Offset(18.3, 16.1),
    ];
    _lines(canvas, stroke, [
      (5.2, 7.3, 11.4, 11.2),
      (11.4, 11.2, 18.8, 6.2),
      (5.2, 7.3, 8.2, 18),
      (11.4, 11.2, 18.3, 16.1),
    ]);
    for (final node in nodes) {
      canvas.drawCircle(node, 1.55, fill);
    }
  }

  void _memoryTimeline(Canvas canvas, Paint stroke, Paint fill) {
    _lines(canvas, stroke, [(6, 4.5, 6, 19.5)]);
    for (final entry in const [(7.2, 12.2), (12.0, 16.3), (16.8, 13.8)]) {
      canvas.drawCircle(Offset(6, entry.$1), 1.7, fill);
      canvas.drawLine(
        Offset(8.2, entry.$1),
        Offset(entry.$2, entry.$1),
        stroke,
      );
    }
  }

  void _userProfile(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(3.8, 5, 16.4, 14),
        const Radius.circular(3.5),
      ),
      stroke,
    );
    canvas.drawCircle(const Offset(9, 10.8), 2.1, fill);
    _lines(canvas, stroke, [(6.1, 16.4, 12, 16.4)]);
    _lines(canvas, stroke, [
      (14.2, 10.5, 17.4, 10.5),
      (14.2, 13.6, 16.6, 13.6),
    ]);
  }

  void _worldBook(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(4.2, 6.4)
      ..quadraticBezierTo(7.5, 5.2, 12, 7.2)
      ..lineTo(12, 19)
      ..quadraticBezierTo(7.8, 17.2, 4.2, 18.2)
      ..close();
    canvas.drawPath(path, stroke);
    final right = Path()
      ..moveTo(19.8, 6.4)
      ..quadraticBezierTo(16.5, 5.2, 12, 7.2)
      ..lineTo(12, 19)
      ..quadraticBezierTo(16.2, 17.2, 19.8, 18.2)
      ..close();
    canvas.drawPath(right, stroke);
  }

  void _personalSpace(Canvas canvas, Paint stroke) {
    canvas.drawCircle(const Offset(12, 12), 8, stroke);
    canvas.drawCircle(const Offset(12, 9.1), 2.3, stroke);
    final body = Path()
      ..moveTo(7.6, 17)
      ..cubicTo(8.1, 14.8, 9.8, 13.7, 12, 13.7)
      ..cubicTo(14.2, 13.7, 15.9, 14.8, 16.4, 17);
    canvas.drawPath(body, stroke);
  }

  void _characters(Canvas canvas, Paint stroke) {
    canvas.drawCircle(const Offset(8.3, 8.3), 2.7, stroke);
    canvas.drawCircle(const Offset(16.2, 9.2), 2.2, stroke);
    final left = Path()
      ..moveTo(3.9, 19)
      ..cubicTo(4.3, 15.8, 6, 14.2, 8.4, 14.2)
      ..cubicTo(10.8, 14.2, 12.5, 15.8, 12.9, 19);
    canvas.drawPath(left, stroke);
    final right = Path()
      ..moveTo(13.9, 15.2)
      ..cubicTo(15, 14.5, 16.3, 14.5, 17.5, 15.2)
      ..cubicTo(18.8, 16, 19.4, 17.2, 19.6, 18.7);
    canvas.drawPath(right, stroke);
  }

  void _workshop(Canvas canvas, Paint stroke, Paint fill) {
    final nib = Path()
      ..moveTo(5.2, 18.8)
      ..lineTo(7.1, 12.6)
      ..lineTo(14.8, 4.9)
      ..quadraticBezierTo(16.1, 3.6, 17.4, 4.9)
      ..lineTo(19.1, 6.6)
      ..quadraticBezierTo(20.4, 7.9, 19.1, 9.2)
      ..lineTo(11.4, 16.9)
      ..close();
    canvas.drawPath(nib, stroke);
    _lines(canvas, stroke, [(9.1, 12.8, 11.2, 14.9)]);
    canvas.drawCircle(const Offset(7.7, 16), 1.1, fill);
  }

  void _game(Canvas canvas, Paint stroke, Paint fill) {
    final path = Path()
      ..moveTo(7.2, 7.2)
      ..lineTo(16.8, 7.2)
      ..cubicTo(18.5, 7.2, 19.7, 8.4, 20.1, 10.1)
      ..lineTo(21, 14.2)
      ..cubicTo(21.4, 16.4, 19.8, 18.1, 17.8, 17.5)
      ..lineTo(15.8, 16.9)
      ..lineTo(8.2, 16.9)
      ..lineTo(6.2, 17.5)
      ..cubicTo(4.2, 18.1, 2.6, 16.4, 3, 14.2)
      ..lineTo(3.9, 10.1)
      ..cubicTo(4.3, 8.4, 5.5, 7.2, 7.2, 7.2);
    canvas.drawPath(path, stroke);
    _lines(canvas, stroke, [(7.2, 10.4, 7.2, 13.8), (5.5, 12.1, 8.9, 12.1)]);
    canvas.drawCircle(const Offset(16.4, 10.8), 1, fill);
    canvas.drawCircle(const Offset(18.2, 13), 1, fill);
  }

  void _channels(Canvas canvas, Paint stroke) {
    final back = Path()
      ..moveTo(5.2, 4.8)
      ..lineTo(15.2, 4.8)
      ..quadraticBezierTo(17.6, 4.8, 17.6, 7.2)
      ..lineTo(17.6, 11)
      ..lineTo(10.2, 11)
      ..lineTo(7.1, 13.8)
      ..lineTo(7.1, 11)
      ..lineTo(6, 11)
      ..quadraticBezierTo(4.2, 11, 4.2, 9.2)
      ..lineTo(4.2, 6.4)
      ..quadraticBezierTo(4.2, 4.8, 5.2, 4.8);
    canvas.drawPath(back, stroke);
    final front = Path()
      ..moveTo(9, 13.2)
      ..lineTo(18, 13.2)
      ..quadraticBezierTo(20, 13.2, 20, 15.2)
      ..lineTo(20, 17.4)
      ..quadraticBezierTo(20, 19.2, 18.2, 19.2)
      ..lineTo(12.2, 19.2)
      ..lineTo(9.4, 21)
      ..lineTo(9.4, 19.2)
      ..lineTo(9, 19.2);
    canvas.drawPath(front, stroke);
  }

  void _settings(Canvas canvas, Paint stroke, Paint fill) {
    _lines(canvas, stroke, [
      (4.2, 7, 19.8, 7),
      (4.2, 12, 19.8, 12),
      (4.2, 17, 19.8, 17),
    ]);
    for (final offset in const [
      Offset(8.2, 7),
      Offset(15.2, 12),
      Offset(10.5, 17),
    ]) {
      canvas.drawCircle(offset, 1.8, fill);
    }
  }

  void _package(Canvas canvas, Paint stroke) {
    final box = Path()
      ..moveTo(4.2, 7.1)
      ..lineTo(12, 3.7)
      ..lineTo(19.8, 7.1)
      ..lineTo(19.8, 16.9)
      ..lineTo(12, 20.3)
      ..lineTo(4.2, 16.9)
      ..close();
    canvas.drawPath(box, stroke);
    _lines(canvas, stroke, [
      (4.6, 7.2, 12, 10.6),
      (19.4, 7.2, 12, 10.6),
      (12, 10.6, 12, 20.1),
    ]);
  }

  void _mcp(Canvas canvas, Paint stroke, Paint fill) {
    const center = Offset(12, 12);
    _lines(canvas, stroke, [
      (12, 12, 12, 4.5),
      (12, 12, 18.3, 15.7),
      (12, 12, 5.7, 15.7),
    ]);
    canvas.drawCircle(center, 2.1, fill);
    for (final point in const [
      Offset(12, 4.4),
      Offset(18.4, 15.8),
      Offset(5.6, 15.8),
    ]) {
      canvas.drawCircle(point, 1.8, fill);
    }
  }

  void _skill(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(12, 3.8)
      ..quadraticBezierTo(12.9, 9.2, 20.2, 12)
      ..quadraticBezierTo(12.9, 14.8, 12, 20.2)
      ..quadraticBezierTo(11.1, 14.8, 3.8, 12)
      ..quadraticBezierTo(11.1, 9.2, 12, 3.8);
    canvas.drawPath(path, stroke);
    canvas.drawCircle(
      const Offset(18.8, 5.2),
      .85,
      stroke..style = PaintingStyle.fill,
    );
    stroke.style = PaintingStyle.stroke;
  }

  void _workflow(Canvas canvas, Paint stroke, Paint fill) {
    const nodes = [Offset(5, 7), Offset(12, 12), Offset(19, 7), Offset(19, 18)];
    _lines(canvas, stroke, [
      (6.4, 7.8, 10.6, 11.2),
      (13.4, 11.2, 17.6, 7.8),
      (12, 12, 18.3, 17.4),
    ]);
    for (final node in nodes) {
      canvas.drawCircle(node, 1.65, fill);
    }
  }

  void _edit(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(5.2, 18.8)
      ..lineTo(6.6, 14.3)
      ..lineTo(16.6, 4.3)
      ..quadraticBezierTo(18, 2.9, 19.4, 4.3)
      ..lineTo(19.7, 4.6)
      ..quadraticBezierTo(21.1, 6, 19.7, 7.4)
      ..lineTo(9.7, 17.4)
      ..close();
    canvas.drawPath(path, stroke);
    _lines(canvas, stroke, [(8.5, 15.5, 17.8, 6.2)]);
  }

  void _archive(Canvas canvas, Paint stroke) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(4, 5, 16, 4.5),
        const Radius.circular(1.7),
      ),
      stroke,
    );
    _polyline(canvas, stroke, [
      const Offset(5.4, 9.5),
      const Offset(6.4, 19),
      const Offset(17.6, 19),
      const Offset(18.6, 9.5),
    ]);
    _lines(canvas, stroke, [(10, 12.2, 14, 12.2)]);
  }

  void _pin(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(8.1, 4.3)
      ..lineTo(15.9, 4.3)
      ..lineTo(14.7, 9.1)
      ..lineTo(17.4, 12.2)
      ..lineTo(6.6, 12.2)
      ..lineTo(9.3, 9.1)
      ..close();
    canvas.drawPath(path, stroke);
    _lines(canvas, stroke, [(12, 12.2, 12, 20)]);
  }

  void _delete(Canvas canvas, Paint stroke) {
    _lines(canvas, stroke, [
      (5, 7, 19, 7),
      (9, 4, 15, 4),
      (7, 7, 8, 20),
      (17, 7, 16, 20),
      (8, 20, 16, 20),
      (10.5, 10, 10.7, 17),
      (13.5, 10, 13.3, 17),
    ]);
  }

  void _update(Canvas canvas, Paint stroke) {
    canvas.drawArc(
      const Rect.fromLTWH(5, 5, 14, 14),
      -math.pi / 2,
      math.pi * 1.55,
      false,
      stroke,
    );
    _polyline(canvas, stroke, [
      const Offset(16.3, 3.8),
      const Offset(19.4, 5.3),
      const Offset(17.6, 8.2),
    ]);
  }

  void _warning(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(10.7, 4.7)
      ..lineTo(3.9, 17.7)
      ..quadraticBezierTo(3.2, 19.2, 4.9, 19.2)
      ..lineTo(19.1, 19.2)
      ..quadraticBezierTo(20.8, 19.2, 20.1, 17.7)
      ..lineTo(13.3, 4.7)
      ..quadraticBezierTo(12.4, 3, 10.7, 4.7);
    canvas.drawPath(path, stroke);
    _lines(canvas, stroke, [(12, 8.5, 12, 13.4)]);
    canvas.drawCircle(
      const Offset(12, 16.5),
      1,
      stroke..style = PaintingStyle.fill,
    );
    stroke.style = PaintingStyle.stroke;
  }

  void _member(Canvas canvas, Paint stroke) {
    canvas.drawCircle(const Offset(9.1, 8.5), 2.8, stroke);
    final body = Path()
      ..moveTo(4.2, 19)
      ..cubicTo(4.5, 15.8, 6.4, 14.1, 9.2, 14.1)
      ..cubicTo(11.9, 14.1, 13.8, 15.8, 14.2, 19);
    canvas.drawPath(body, stroke);
    _lines(canvas, stroke, [(18, 5, 18, 10), (15.5, 7.5, 20.5, 7.5)]);
  }

  void _workspace(Canvas canvas, Paint stroke) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(3.8, 4.5, 16.4, 15),
        const Radius.circular(3.5),
      ),
      stroke,
    );
    _lines(canvas, stroke, [(9.3, 4.7, 9.3, 19.3), (9.4, 12, 19.8, 12)]);
  }

  void _phone(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(7.1, 3.9)
      ..cubicTo(5.8, 4.3, 5, 5.4, 5.1, 7)
      ..cubicTo(5.5, 12, 11.9, 18.5, 17, 18.9)
      ..cubicTo(18.6, 19, 19.7, 18.2, 20.1, 16.9)
      ..lineTo(20.8, 14.6)
      ..cubicTo(21.1, 13.7, 20.6, 12.8, 19.7, 12.4)
      ..lineTo(16.7, 11.2)
      ..cubicTo(16, 10.9, 15.2, 11.1, 14.7, 11.7)
      ..lineTo(13.7, 12.8)
      ..cubicTo(11.7, 11.8, 10.1, 10.2, 9.2, 8.2)
      ..lineTo(10.3, 7.2)
      ..cubicTo(10.9, 6.7, 11.1, 5.9, 10.8, 5.2)
      ..lineTo(9.5, 4.3)
      ..cubicTo(8.9, 3.7, 8, 3.5, 7.1, 3.9);
    canvas.drawPath(path, stroke);
  }

  void _refresh(Canvas canvas, Paint stroke) {
    canvas.drawArc(
      const Rect.fromLTWH(4.5, 4.5, 15, 15),
      -math.pi * 0.35,
      math.pi * 1.45,
      false,
      stroke,
    );
    _polyline(canvas, stroke, [
      const Offset(18.1, 3.6),
      const Offset(19.7, 6.3),
      const Offset(16.8, 7.1),
    ]);
  }

  void _folder(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(3.8, 7.2)
      ..quadraticBezierTo(3.8, 5.5, 5.5, 5.5)
      ..lineTo(9.2, 5.5)
      ..lineTo(10.4, 7.1)
      ..lineTo(18.5, 7.1)
      ..quadraticBezierTo(20.2, 7.1, 20.2, 8.8)
      ..lineTo(20.2, 17.2)
      ..quadraticBezierTo(20.2, 18.9, 18.5, 18.9)
      ..lineTo(5.5, 18.9)
      ..quadraticBezierTo(3.8, 18.9, 3.8, 17.2)
      ..close();
    canvas.drawPath(path, stroke);
  }

  void _image(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(3.8, 4.8, 16.4, 14.4),
        const Radius.circular(3),
      ),
      stroke,
    );
    canvas.drawCircle(const Offset(8.4, 9.3), 1.25, fill);
    _polyline(canvas, stroke, [
      const Offset(5.4, 17.1),
      const Offset(10.2, 12.5),
      const Offset(13.6, 15.8),
      const Offset(15.3, 14.2),
      const Offset(18.5, 17.2),
    ]);
  }

  void _video(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(3.8, 5.5, 12.8, 13),
        const Radius.circular(3),
      ),
      stroke,
    );
    _polyline(canvas, stroke, [
      const Offset(16.6, 10),
      const Offset(20.2, 7.8),
      const Offset(20.2, 16.2),
      const Offset(16.6, 14),
    ]);
    final play = Path()
      ..moveTo(8.7, 9.2)
      ..lineTo(12.2, 12)
      ..lineTo(8.7, 14.8)
      ..close();
    canvas.drawPath(play, fill);
  }

  void _attachment(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(9.2, 11.8)
      ..lineTo(15.8, 5.2)
      ..cubicTo(17.2, 3.8, 19.5, 3.8, 20.9, 5.2)
      ..cubicTo(22.2, 6.5, 22.2, 8.8, 20.9, 10.1)
      ..lineTo(12.3, 18.7)
      ..cubicTo(10.1, 20.9, 6.5, 20.9, 4.3, 18.7)
      ..cubicTo(2.1, 16.5, 2.1, 12.9, 4.3, 10.7)
      ..lineTo(12.7, 2.3);
    canvas.drawPath(path, stroke);
  }

  void _code(Canvas canvas, Paint stroke) {
    _polyline(canvas, stroke, [
      const Offset(8.2, 6.8),
      const Offset(3.5, 12),
      const Offset(8.2, 17.2),
    ]);
    _polyline(canvas, stroke, [
      const Offset(15.8, 6.8),
      const Offset(20.5, 12),
      const Offset(15.8, 17.2),
    ]);
    _lines(canvas, stroke, [(13.7, 4.8, 10.3, 19.2)]);
  }

  void _lock(Canvas canvas, Paint stroke, {required bool locked}) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(5, 10.2, 14, 10.2),
        const Radius.circular(2.8),
      ),
      stroke,
    );
    final shackle = Path()
      ..moveTo(locked ? 7.8 : 9.3, 10.2)
      ..lineTo(locked ? 7.8 : 9.3, 7.9)
      ..cubicTo(
        locked ? 7.8 : 9.3,
        4.4,
        locked ? 16.2 : 15.5,
        4.4,
        locked ? 16.2 : 15.5,
        7.9,
      )
      ..lineTo(locked ? 16.2 : 13.6, locked ? 10.2 : 9.2);
    canvas.drawPath(shackle, stroke);
    canvas.drawCircle(const Offset(12, 15.3), 1.15, stroke);
  }

  void _microphone(Canvas canvas, Paint stroke) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(8.3, 3.3, 7.4, 12),
        const Radius.circular(3.7),
      ),
      stroke,
    );
    final stand = Path()
      ..moveTo(5.9, 11.2)
      ..cubicTo(5.9, 14.8, 8.6, 17.2, 12, 17.2)
      ..cubicTo(15.4, 17.2, 18.1, 14.8, 18.1, 11.2);
    canvas.drawPath(stand, stroke);
    _lines(canvas, stroke, [(12, 17.2, 12, 21), (8.8, 21, 15.2, 21)]);
  }

  void _keyboard(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(2.8, 6.2, 18.4, 12.2),
        const Radius.circular(2.8),
      ),
      stroke,
    );
    for (final y in const [9.5, 12.3]) {
      for (final x in const [6.2, 9.2, 12.0, 14.8, 17.8]) {
        canvas.drawCircle(Offset(x, y), 0.75, fill);
      }
    }
    _lines(canvas, stroke, [(7.1, 15.5, 16.9, 15.5)]);
  }

  void _send(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(3.8, 4.3)
      ..lineTo(20.7, 12)
      ..lineTo(3.8, 19.7)
      ..lineTo(6.4, 12)
      ..close()
      ..moveTo(6.4, 12)
      ..lineTo(20.1, 12);
    canvas.drawPath(path, stroke);
  }

  void _copy(Canvas canvas, Paint stroke) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(8, 3.8, 12.2, 12.2),
        const Radius.circular(2.5),
      ),
      stroke,
    );
    final path = Path()
      ..moveTo(5.2, 7)
      ..quadraticBezierTo(3.8, 7, 3.8, 8.6)
      ..lineTo(3.8, 18.3)
      ..quadraticBezierTo(3.8, 20.2, 5.7, 20.2)
      ..lineTo(15.4, 20.2)
      ..quadraticBezierTo(17, 20.2, 17, 18.8);
    canvas.drawPath(path, stroke);
  }

  void _download(Canvas canvas, Paint stroke) {
    _lines(canvas, stroke, [
      (12, 3.7, 12, 15.2),
      (7.8, 11, 12, 15.2),
      (16.2, 11, 12, 15.2),
      (4.5, 19.6, 19.5, 19.6),
    ]);
  }

  void _screenShare(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(3, 4.5, 18, 12.5),
        const Radius.circular(2.8),
      ),
      stroke,
    );
    _lines(canvas, stroke, [(9, 20.2, 15, 20.2), (12, 17, 12, 20.2)]);
    _polyline(canvas, stroke, [
      const Offset(12.2, 13.5),
      const Offset(12.2, 8),
      const Offset(15.8, 11.5),
    ]);
    canvas.drawCircle(const Offset(12.2, 8), 0.9, fill);
  }

  void _cleanup(Canvas canvas, Paint stroke, Paint fill) {
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        const Rect.fromLTWH(9.6, 3.8, 4.8, 9.5),
        const Radius.circular(2.1),
      ),
      stroke,
    );
    final path = Path()
      ..moveTo(7.2, 13.2)
      ..lineTo(16.8, 13.2)
      ..lineTo(19.2, 20.2)
      ..lineTo(4.8, 20.2)
      ..close();
    canvas.drawPath(path, stroke);
    canvas.drawCircle(const Offset(18.4, 4.2), 1.05, fill);
  }

  void _page(Canvas canvas, Paint stroke) {
    final path = Path()
      ..moveTo(6, 3.8)
      ..lineTo(14.8, 3.8)
      ..lineTo(19, 8)
      ..lineTo(19, 20.2)
      ..lineTo(6, 20.2)
      ..close()
      ..moveTo(14.8, 3.8)
      ..lineTo(14.8, 8)
      ..lineTo(19, 8);
    canvas.drawPath(path, stroke);
  }

  void _lines(
    Canvas canvas,
    Paint stroke,
    List<(double, double, double, double)> lines,
  ) {
    for (final line in lines) {
      canvas.drawLine(
        Offset(line.$1, line.$2),
        Offset(line.$3, line.$4),
        stroke,
      );
    }
  }

  void _polyline(Canvas canvas, Paint stroke, List<Offset> points) {
    final path = Path()..moveTo(points.first.dx, points.first.dy);
    for (final point in points.skip(1)) {
      path.lineTo(point.dx, point.dy);
    }
    canvas.drawPath(path, stroke);
  }

  @override
  bool shouldRepaint(_AmitiaIconPainter oldDelegate) {
    return oldDelegate.glyph != glyph ||
        oldDelegate.color != color ||
        oldDelegate.strokeWidth != strokeWidth;
  }
}
