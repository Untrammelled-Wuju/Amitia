import ActivityKit

@available(iOS 16.1, *)
struct AmitiaRunAttributes: ActivityAttributes {
  struct ContentState: Codable, Hashable {
    var revision: Int64
    var phase: String
    var title: String
    var summary: String
    var currentStep: Int
    var totalSteps: Int
    var progress: Double
    var updatedAt: Int64
    var locale: String? = nil
    var appearance: String? = nil
    var agentName: String? = nil
    var totalTokens: Int? = nil
  }

  var runId: String
  var conversationId: String
  var characterId: String
  var agentName: String
  var startedAt: Int64
}
