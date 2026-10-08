import ActivityKit
import Foundation
import SwiftUI
import WidgetKit

@main
struct AmitiaLiveActivityBundle: WidgetBundle {
  var body: some Widget {
    AmitiaRunLiveActivity()
  }
}

struct AmitiaRunLiveActivity: Widget {
  var body: some WidgetConfiguration {
    ActivityConfiguration(for: AmitiaRunAttributes.self) { context in
      LockScreenRunView(context: context)
        .activityBackgroundTint(activityBackground(context.state))
        .activitySystemActionForegroundColor(activityForeground(context.state))
    } dynamicIsland: { context in
      DynamicIsland {
        DynamicIslandExpandedRegion(.leading) {
          Image(systemName: iconName(
            context.state.phase,
            agentName: effectiveAgentName(context),
            isStale: context.isStale
          ))
            .font(.title3)
        }
        DynamicIslandExpandedRegion(.trailing) {
          progressText(context.state, isStale: context.isStale)
            .font(.caption.monospacedDigit())
        }
        DynamicIslandExpandedRegion(.center) {
          Text(context.state.title.isEmpty ? context.attributes.agentName : context.state.title)
            .font(.headline)
            .lineLimit(1)
        }
        DynamicIslandExpandedRegion(.bottom) {
          VStack(alignment: .leading, spacing: 6) {
            if context.isStale {
              Label(
                liveActivityStaleLabel(context.state.locale),
                systemImage: "exclamationmark.triangle.fill"
              )
              .font(.caption)
              .lineLimit(1)
            } else if !context.state.summary.isEmpty {
              Text(context.state.summary)
                .font(.caption)
                .lineLimit(2)
            }
            ProgressView(value: normalizedProgress(context.state))
          }
        }
      } compactLeading: {
        Image(systemName: iconName(
          context.state.phase,
          agentName: effectiveAgentName(context),
          isStale: context.isStale
        ))
      } compactTrailing: {
        progressText(context.state, isStale: context.isStale)
          .font(.caption2.monospacedDigit())
      } minimal: {
        Image(systemName: iconName(
          context.state.phase,
          agentName: effectiveAgentName(context),
          isStale: context.isStale
        ))
      }
      .widgetURL(deepLink(context.attributes))
      .keylineTint(tint(context.state.phase))
    }
  }

  private func normalizedProgress(
    _ state: AmitiaRunAttributes.ContentState
  ) -> Double {
    if state.phase == "completed" {
      return 1
    }
    if state.totalSteps > 0 {
      return min(
        1,
        max(0, Double(state.currentStep) / Double(state.totalSteps))
      )
    }
    return min(1, max(0, state.progress))
  }

  private func progressText(
    _ state: AmitiaRunAttributes.ContentState,
    isStale: Bool
  ) -> Text {
    if isStale {
      return Text(liveActivityStaleLabel(state.locale))
    }
    if state.totalSteps > 0 {
      return Text("\(state.currentStep)/\(state.totalSteps)")
    }
    let value = Int((normalizedProgress(state) * 100).rounded())
    if value > 0 {
      return Text("\(value)%")
    }
    return Text(statusLabel(state.phase, locale: state.locale))
  }

  private func statusLabel(_ phase: String, locale: String?) -> String {
    liveActivityStatus(phase, locale: locale, long: false)
  }

  private func effectiveAgentName(
    _ context: ActivityViewContext<AmitiaRunAttributes>
  ) -> String {
    let value = context.state.agentName?.trimmingCharacters(in: .whitespacesAndNewlines)
    return value?.isEmpty == false ? value! : context.attributes.agentName
  }

  private func iconName(
    _ phase: String,
    agentName: String,
    isStale: Bool = false
  ) -> String {
    if isStale {
      return "exclamationmark.triangle.fill"
    }
    switch phase {
    case "completed":
      return "checkmark.circle.fill"
    case "failed":
      return "xmark.circle.fill"
    case "interrupted", "cancelled":
      return "stop.circle.fill"
    case "waiting_approval":
      return "hand.raised.fill"
    default:
      return liveActivityAgentIcon(agentName)
    }
  }

  private func tint(_ phase: String) -> Color {
    switch phase {
    case "completed":
      return .green
    case "failed":
      return .red
    case "waiting_approval":
      return .orange
    default:
      return .accentColor
    }
  }

  private func deepLink(_ attributes: AmitiaRunAttributes) -> URL? {
    var components = URLComponents()
    components.scheme = "amitia"
    components.host = "chat"
    components.path = "/" + attributes.conversationId
    components.queryItems = [
      URLQueryItem(name: "run", value: attributes.runId),
    ]
    return components.url
  }
}

private func liveActivityLocale(_ raw: String?) -> String {
  let value = (raw ?? "").replacingOccurrences(of: "_", with: "-").lowercased()
  if value.hasPrefix("zh-hant") || value.hasPrefix("zh-tw") ||
      value.hasPrefix("zh-hk") {
    return "zh-TW"
  }
  if value.hasPrefix("zh") { return "zh" }
  for code in ["ja", "ko", "fr", "es", "de", "pt", "ru", "ar"] {
    if value.hasPrefix(code) { return code }
  }
  return "en"
}

private func liveActivityStatus(
  _ phase: String,
  locale: String?,
  long: Bool
) -> String {
  let key: String
  switch phase {
  case "completed": key = "completed"
  case "failed": key = "failed"
  case "interrupted", "cancelled": key = "interrupted"
  case "waiting_approval": key = "waiting"
  case "cancelling": key = "cancelling"
  default: key = "running"
  }
  let values: [String: [String: String]] = [
    "zh": [
      "completed": long ? "已完成" : "完成",
      "failed": long ? "执行失败" : "失败",
      "interrupted": long ? "已中断" : "中断",
      "waiting": long ? "等待确认" : "待确认",
      "cancelling": "取消中",
      "running": long ? "正在执行" : "执行中",
    ],
    "zh-TW": [
      "completed": long ? "已完成" : "完成",
      "failed": long ? "執行失敗" : "失敗",
      "interrupted": long ? "已中斷" : "中斷",
      "waiting": long ? "等待確認" : "待確認",
      "cancelling": "取消中",
      "running": long ? "正在執行" : "執行中",
    ],
    "ja": [
      "completed": "完了", "failed": "失敗", "interrupted": "中断",
      "waiting": "確認待ち", "cancelling": "キャンセル中", "running": "実行中",
    ],
    "ko": [
      "completed": "완료", "failed": "실패", "interrupted": "중단",
      "waiting": "확인 대기", "cancelling": "취소 중", "running": "실행 중",
    ],
    "fr": [
      "completed": "Terminé", "failed": "Échec", "interrupted": "Interrompu",
      "waiting": "Confirmation", "cancelling": "Annulation", "running": "En cours",
    ],
    "es": [
      "completed": "Completado", "failed": "Error", "interrupted": "Interrumpido",
      "waiting": "Confirmación", "cancelling": "Cancelando", "running": "En curso",
    ],
    "de": [
      "completed": "Fertig", "failed": "Fehlgeschlagen", "interrupted": "Unterbrochen",
      "waiting": "Bestätigung", "cancelling": "Wird abgebrochen", "running": "Läuft",
    ],
    "pt": [
      "completed": "Concluído", "failed": "Falhou", "interrupted": "Interrompido",
      "waiting": "Confirmação", "cancelling": "Cancelando", "running": "Em execução",
    ],
    "ru": [
      "completed": "Готово", "failed": "Ошибка", "interrupted": "Прервано",
      "waiting": "Подтверждение", "cancelling": "Отмена", "running": "Выполняется",
    ],
    "ar": [
      "completed": "مكتمل", "failed": "فشل", "interrupted": "متوقف",
      "waiting": "بانتظار التأكيد", "cancelling": "جارٍ الإلغاء", "running": "قيد التنفيذ",
    ],
    "en": [
      "completed": "Completed", "failed": "Failed", "interrupted": "Interrupted",
      "waiting": "Waiting", "cancelling": "Cancelling", "running": "Running",
    ],
  ]
  let language = liveActivityLocale(locale)
  return values[language]?[key] ?? values["en"]![key]!
}

private func liveActivityStaleLabel(_ locale: String?) -> String {
  switch liveActivityLocale(locale) {
  case "zh": return "连接已过期"
  case "zh-TW": return "連線已過期"
  case "ja": return "接続が期限切れ"
  case "ko": return "연결 만료"
  case "fr": return "Connexion expirée"
  case "es": return "Conexión caducada"
  case "de": return "Verbindung veraltet"
  case "pt": return "Conexão expirada"
  case "ru": return "Соединение устарело"
  case "ar": return "انتهت صلاحية الاتصال"
  default: return "Connection stale"
  }
}

private func liveActivityTokenText(_ total: Int) -> String {
  guard total > 0 else { return "" }
  if total >= 1_000_000 {
    let value = Double(total) / 1_000_000
    return String(format: value >= 10 ? "%.0fM" : "%.1fM", value) + " tokens"
  }
  if total >= 1_000 {
    let value = Double(total) / 1_000
    return String(format: value >= 10 ? "%.0fK" : "%.1fK", value) + " tokens"
  }
  return "\(total) tokens"
}

private func liveActivityStepText(
  current: Int,
  total: Int,
  locale: String?
) -> String {
  switch liveActivityLocale(locale) {
  case "zh": return "\(current) / \(total) 步"
  case "zh-TW": return "\(current) / \(total) 步"
  case "ja": return "\(current) / \(total) ステップ"
  case "ko": return "\(current) / \(total) 단계"
  case "fr": return "\(current) / \(total) étapes"
  case "es": return "\(current) / \(total) pasos"
  case "de": return "\(current) / \(total) Schritte"
  case "pt": return "\(current) / \(total) etapas"
  case "ru": return "\(current) / \(total) шаг."
  case "ar": return "\(current) / \(total) خطوة"
  default: return "\(current) / \(total) steps"
  }
}

private func liveActivityAgentIcon(_ raw: String) -> String {
  let value = raw.lowercased()
  if value.contains("codex") { return "chevron.left.forwardslash.chevron.right" }
  if value.contains("claude") { return "c.circle.fill" }
  if value.contains("hermes") { return "shippingbox.fill" }
  if value.contains("workflow") { return "point.3.connected.trianglepath.dotted" }
  if value.contains("search") { return "magnifyingglass" }
  return "sparkles"
}

private func activityBackground(
  _ state: AmitiaRunAttributes.ContentState
) -> Color {
  state.appearance == "light"
    ? Color.white.opacity(0.94)
    : Color.black.opacity(0.82)
}

private func activityForeground(
  _ state: AmitiaRunAttributes.ContentState
) -> Color {
  state.appearance == "light" ? .black : .white
}

private struct LockScreenRunView: View {
  let context: ActivityViewContext<AmitiaRunAttributes>

  var body: some View {
    VStack(alignment: .leading, spacing: 10) {
      HStack(spacing: 8) {
        Image(systemName: symbol)
        Text(effectiveAgentName)
          .font(.headline)
          .lineLimit(1)
        Spacer()
        Text(status)
          .font(.caption)
          .foregroundStyle(.secondary)
      }
      Text(context.state.title)
        .font(.subheadline.weight(.semibold))
        .lineLimit(1)
      if !context.state.summary.isEmpty {
        Text(context.state.summary)
          .font(.caption)
          .foregroundStyle(.secondary)
          .lineLimit(2)
      }
      ProgressView(value: progress)
      HStack {
        if context.state.totalSteps > 0 {
          Text(liveActivityStepText(
            current: context.state.currentStep,
            total: context.state.totalSteps,
            locale: context.state.locale
          ))
            .font(.caption2.monospacedDigit())
        } else if progress > 0 {
          Text("\(Int((progress * 100).rounded()))%")
            .font(.caption2.monospacedDigit())
        }
        if let tokens = context.state.totalTokens, tokens > 0 {
          Text(liveActivityTokenText(tokens))
            .font(.caption2.monospacedDigit())
            .foregroundStyle(.tertiary)
        }
        Spacer()
        Text(updated)
          .font(.caption2)
          .foregroundStyle(.tertiary)
      }
    }
    .padding()
    .widgetURL(deepLink)
  }

  private var progress: Double {
    if context.state.phase == "completed" {
      return 1
    }
    if context.state.totalSteps > 0 {
      return min(
        1,
        max(
          0,
          Double(context.state.currentStep) /
            Double(context.state.totalSteps)
        )
      )
    }
    return min(1, max(0, context.state.progress))
  }

  private var effectiveAgentName: String {
    let current = context.state.agentName?
      .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    return current.isEmpty
      ? (context.attributes.agentName.isEmpty ? "Amitia" : context.attributes.agentName)
      : current
  }

  private var symbol: String {
    if context.isStale {
      return "exclamationmark.triangle.fill"
    }
    switch context.state.phase {
    case "completed":
      return "checkmark.circle.fill"
    case "failed":
      return "xmark.circle.fill"
    case "waiting_approval":
      return "hand.raised.fill"
    case "interrupted", "cancelled":
      return "stop.circle.fill"
    default:
      return liveActivityAgentIcon(effectiveAgentName)
    }
  }

  private var status: String {
    if context.isStale {
      return liveActivityStaleLabel(context.state.locale)
    }
    return liveActivityStatus(
      context.state.phase,
      locale: context.state.locale,
      long: true
    )
  }

  private var updated: String {
    let date = Date(
      timeIntervalSince1970: TimeInterval(context.state.updatedAt)
    )
    return date.formatted(date: .omitted, time: .shortened)
  }

  private var deepLink: URL? {
    var components = URLComponents()
    components.scheme = "amitia"
    components.host = "chat"
    components.path = "/" + context.attributes.conversationId
    components.queryItems = [
      URLQueryItem(name: "run", value: context.attributes.runId),
    ]
    return components.url
  }
}
