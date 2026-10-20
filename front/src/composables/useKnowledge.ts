import { ref } from "vue";
import { useApi } from "./useApi";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";
import { coreConfigurationRequestConfig } from "./useCoreConfigurationAccess";

export interface KnowledgeSource {
  id: string;
  logicalName: string;
  filename: string;
  mimeType: string;
  sourceType: string;
  sizeBytes: number;
  sha256: string;
  revision: number;
  parserVersion: string;
  chunkSchemaVersion: string;
  embeddingVersion: string;
  indexVersion: string;
  status: "pending" | "processing" | "ready" | "ready_degraded" | "failed" | "deleted" | string;
  expectedUnits: number;
  processedUnits: number;
  failedUnits: number;
  indexedChunks: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface KnowledgeJob {
  id: string;
  sourceId: string;
  sourceRevision: number;
  kind: "ingest" | "reindex" | "rollback" | string;
  stage: string;
  status: "queued" | "running" | "completed" | "failed" | string;
  lastCommittedOrdinal: number;
  expectedUnits: number;
  processedUnits: number;
  failedUnits: number;
  indexedChunks: number;
  retryCount: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
  finishedAt?: string;
}


export interface KnowledgeSourceRevision {
  id: string;
  sourceId: string;
  revision: number;
  filename: string;
  mimeType: string;
  sourceType: string;
  sizeBytes: number;
  sha256: string;
  parserVersion: string;
  chunkSchemaVersion: string;
  embeddingVersion: string;
  indexVersion: string;
  status: string;
  expectedUnits: number;
  processedUnits: number;
  failedUnits: number;
  indexedChunks: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
  activatedAt?: string;
  retainUntil?: string;
  purgedAt?: string;
  isActive: boolean;
  canRollback: boolean;
  canPurge: boolean;
}

export interface KnowledgeSourceDetail {
  source: KnowledgeSource;
  revisions: KnowledgeSourceRevision[];
  latestJob?: KnowledgeJob;
  unitCount: number;
  chunkCount: number;
  indexHealth: "healthy" | "degraded" | "unavailable" | string;
  hasActiveJob: boolean;
}

export interface KnowledgeEvidence {
  id: string;
  unitId?: string;
  evidenceKind?: string;
  coverageKeys?: string[];
  checksum?: string;
  sourceType: string;
  sourceId: string;
  sourceRevision: number;
  title: string;
  content: string;
  locator: Record<string, unknown>;
  denseScore: number;
  sparseScore: number;
  fusionScore: number;
  rerankScore: number;
  authority: number;
  confidence: number;
  trust: string;
  validFrom?: string;
  validTo?: string;
  supersedes?: string[];
  conflictGroup?: string;
  metadata: Record<string, unknown>;
}

export interface KnowledgeUnitOutline {
  id: string;
  sourceId: string;
  sourceRevision: number;
  ordinal: number;
  unitType: string;
  title: string;
  locator: Record<string, unknown>;
  checksum?: string;
}

export interface KnowledgeUnitOutlineResponse {
  sourceId: string;
  sourceRevision: number;
  items: KnowledgeUnitOutline[];
  nextAfterOrdinal?: number;
}

export interface KnowledgeReadRequest {
  evidenceId?: string;
  unitId?: string;
  sourceId?: string;
  sourceRevision?: number;
  locator?: Record<string, unknown>;
  beforeUnits?: number;
  afterUnits?: number;
  maxUnits?: number;
  maxRunes?: number;
}

export interface KnowledgeReadUnit {
  id: string;
  sourceId: string;
  sourceRevision: number;
  ordinal: number;
  unitType: string;
  title: string;
  content: string;
  locator: Record<string, unknown>;
  metadata: Record<string, unknown>;
  checksum: string;
  truncated: boolean;
}

export interface KnowledgeReadResponse {
  sourceId: string;
  sourceRevision: number;
  anchorUnitId: string;
  units: KnowledgeReadUnit[];
  totalRunes: number;
  truncated: boolean;
}

export interface KnowledgeRetrievalSLOLevel {
  level: string;
  samples: number;
  errorCount: number;
  exceededCount: number;
  p50Ms: number;
  p95Ms: number;
  targetMs: number;
  enoughSamples: boolean;
  targetMet: boolean;
}

export interface KnowledgeRetrievalSLOSummary {
  configHash: string;
  windowMinutes: number;
  minSamples: number;
  from: string;
  to: string;
  levels: KnowledgeRetrievalSLOLevel[];
}

export interface KnowledgeRetrievalTraceCandidate {
  rank: number;
  matchedChunkId: string;
  evidenceId: string;
  sourceId: string;
  sourceRevision: number;
  denseScore: number;
  sparseScore: number;
  exactScore: number;
  fusionScore: number;
  relevance: number;
  rerankScore: number;
  mmrScore: number;
  conflictGroup?: string;
  locatorHash?: string;
}

export interface KnowledgeRetrievalTraceEvidence {
  rank: number;
  evidenceId: string;
  sourceId: string;
  sourceRevision: number;
  denseScore: number;
  sparseScore: number;
  fusionScore: number;
  rerankScore: number;
  confidence: number;
  trust: string;
  conflictGroup?: string;
  locatorHash?: string;
}

export interface KnowledgeRetrievalQueryPlanTrace {
  level: string;
  complexity: string;
  originalHash: string;
  effectiveHash: string;
  queryHistoryHashes: string[];
  queryCount: number;
  timeoutMs: number;
  candidateLimit: number;
  parentExpansionTokens: number;
  rerankCandidateLimit: number;
  maxAgenticRounds: number;
}

export interface KnowledgeBenchmarkRunSummary {
  id: string;
  configHash: string;
  k: number;
  cases: number;
  successfulCases: number;
  errorCases: number;
  failedCases: number;
  positiveCases: number;
  noAnswerCases: number;
  citationObservedCases: number;
  scenarioCounts: Record<string, number>;
  recallAtK: number;
  mrr: number;
  ndcgAtK: number;
  contextPrecision: number;
  contextRecall: number;
  noAnswerAccuracy: number;
  citationPrecision: number;
  citationRecall: number;
  groundedness: number;
  errorRate: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  passed: boolean;
  violations?: string[];
  createdAt: string;
}

export interface KnowledgeRuntimeStatus {
  configHash: string;
  retrievalPolicyEnabled: boolean;
  retrievalPolicyShadow: boolean;
  canonicalReaderEnabled: boolean;
  agenticInvestigationEnabled: boolean;
  readerWorkers: number;
  retrievalTraceRetentionHours: number;
  benchmarkRetentionHours: number;
  diagnosticCleanupBatchSize: number;
}

export interface KnowledgeIndexOutboxHealth {
  status: "healthy" | "degraded" | "blocked";
  pendingUpserts: number;
  pendingDeletes: number;
  retryingUpserts: number;
  retryingDeletes: number;
  exhaustedUpserts: number;
  exhaustedDeletes: number;
  activeLeases: number;
  overdueOperations: number;
  oldestOutstandingSeconds: number;
  observedAt: string;
}

export interface KnowledgeOrphanVectorAuditResult {
  status: "empty" | "sampled" | "suspected" | "unavailable";
  sampled: number;
  suspectedOrphans: number;
  metadataMismatches: number;
  inFlight: number;
  collectionsChecked: number;
  collectionsKnown: number;
  sampleLimit: number;
  exhaustive: false;
  observedAt: string;
}

export interface KnowledgeOrphanVectorAuditPageResult extends KnowledgeOrphanVectorAuditResult {
  auditId: string;
  page: number;
  finished: boolean;
  nextCursor?: string;
  observationDigest?: string;
  truncated?: boolean;
  totals?: KnowledgeOrphanPageTotals;
}

export interface KnowledgeRecoveryReview {
  id: string;
  auditId: string;
  recheckId: string;
  receiptHash: string;
  status: "pending" | "approved" | "revoked";
  historyStatus: "verified" | "inconsistent" | "unavailable";
  displayStatus: "history_inconsistent" | "revoked" | "identity_unverified" | "expired" | "needs_recheck" | "approved_unverified";
  action: "manual_investigation";
  createdAt: string;
  updatedAt: string;
  expiresAt: string;
}

export interface KnowledgeRecoveryReviewEvent {
  id: string;
  reviewId: string;
  action: "requested" | "approved" | "revoked";
  receiptHash: string;
  createdAt: string;
}

export interface KnowledgeRecoveryReviewEventHistory {
  reviewId: string;
  historyStatus: "verified" | "inconsistent" | "unavailable";
  items: KnowledgeRecoveryReviewEvent[];
  truncated: boolean;
  executable: false;
}

export interface KnowledgeRecoveryReviewPlan {
  reviewId: string;
  auditId: string;
  status: "pending" | "approved" | "revoked";
  evidenceStatus: "valid" | "expired" | "revoked" | "stale" | "unavailable" | "identity_unverified" | "history_inconsistent";
  historyStatus: "verified" | "inconsistent" | "unavailable";
  receiptHash: string;
  action: "manual_investigation";
  manualReviewReady: boolean;
  canApprove: boolean;
  executable: false;
  checkedAt: string;
}

export interface KnowledgeRecoveryReviewResponse {
  request: KnowledgeRecoveryReview;
  idempotent: boolean;
  executable: false;
}

export interface KnowledgeOrphanRecheckRecord {
  id: string;
  auditId: string;
  page: number;
  status: string;
  resultHash: string;
  receiptHash: string;
  observedAt: string;
  createdAt: string;
}

export interface KnowledgeOrphanRecheckResult {
  auditId: string;
  page: number;
  recheckId?: string;
  manualRecoveryAllowed: boolean;
  status: "matched" | "changed" | "legacy_no_digest" | "unanchored" | "checkpoint_mismatch" | "expired" | "collections_changed" | "unavailable";
  observedAt: string;
  chainAnchored: boolean;
  sampleMatched: boolean;
  countsMatched: boolean;
  previousSampled: number;
  currentSampled: number;
  previousSuspected: number;
  currentSuspected: number;
}

export interface KnowledgeOrphanPageTotals {
  auditId: string;
  pages: number;
  sampled: number;
  suspectedOrphans: number;
  metadataMismatches: number;
  inFlight: number;
}

export interface KnowledgeOrphanAuditSessionSummary extends KnowledgeOrphanPageTotals {
  status: "running" | "finished" | "truncated" | "expired";
  createdAt: string;
  updatedAt: string;
}

export interface KnowledgeOrphanAuditReceipt {
  auditId: string;
  page: number;
  requestHash: string;
  resultHash: string;
  previousHash: string;
  receiptHash: string;
  createdAt: string;
  verified: boolean;
}

export interface KnowledgeOrphanAuditReceiptsResult {
  auditId: string;
  items: KnowledgeOrphanAuditReceipt[];
  nextPage: number;
  hasMore: boolean;
  chainAnchored: boolean;
}

export interface KnowledgeVectorAuditResult {
  status: "healthy" | "degraded" | "unavailable" | "empty";
  sampled: number;
  missing: number;
  mismatched: number;
  collectionsChecked: number;
  sampleLimit: number;
  exhaustive: boolean;
  observedAt: string;
}

export interface KnowledgeRetrievalTraceSummary {
  id: string;
  requestId: string;
  queryHash: string;
  candidateCount: number;
  selectedCount: number;
  sufficient: boolean;
  durationMs: number;
  errorCode?: string;
  temporalMode: string;
  asOf?: string;
  degraded: boolean;
  ambiguous: boolean;
  warnings?: string[];
  candidates?: KnowledgeRetrievalTraceCandidate[];
  evidence?: KnowledgeRetrievalTraceEvidence[];
  queryPlan: KnowledgeRetrievalQueryPlanTrace;
  configHash: string;
  sloTargetMs: number;
  sloExceeded: boolean;
  policyMode?: string;
  routeReason?: string;
  shadowLevel?: string;
  shadowReason?: string;
  createdAt: string;
}

export interface KnowledgeSearchResponse {
  evidence: KnowledgeEvidence[];
  sufficient: boolean;
  answerState: "sufficient" | "insufficient" | "ambiguous" | string;
  sufficiencyReason: string;
  query: string;
  originalQuery: string;
  effectiveQuery: string;
  queryComplexity: "simple" | "complex" | "comparison" | string;
  retrievalLevel: "none" | "fast" | "deep" | "agentic" | string;
  queryHistory?: string[];
  degraded: boolean;
  temporalMode: "current" | "historical" | "all" | string;
  asOf?: string;
  ambiguous: boolean;
  conflictGroups?: string[];
  warnings?: string[];
}

export function useKnowledge() {
  const { get, post, del } = useApi();
  const sources = ref<KnowledgeSource[]>([]);
  const jobs = ref<KnowledgeJob[]>([]);
  const searchResult = ref<KnowledgeSearchResponse | null>(null);
  const loadingSources = ref(false);
  const searching = ref(false);
  const uploading = ref(false);
  const reconciling = ref(false);
  const collecting = ref(false);
  const sourceDetail = ref<KnowledgeSourceDetail | null>(null);
  const loadingSourceDetail = ref(false);
  const runtimeStatus = ref<KnowledgeRuntimeStatus | null>(null);
  const indexOutboxHealth = ref<KnowledgeIndexOutboxHealth | null>(null);
  const vectorAudit = ref<KnowledgeVectorAuditResult | null>(null);
  const orphanVectorAudit = ref<KnowledgeOrphanVectorAuditResult | null>(null);
  const orphanVectorPage = ref<KnowledgeOrphanVectorAuditPageResult | null>(null);
  const orphanPageTotals = ref<KnowledgeOrphanPageTotals | null>(null);
  const orphanAuditSessions = ref<KnowledgeOrphanAuditSessionSummary[]>([]);
  const orphanAuditReceipts = ref<KnowledgeOrphanAuditReceiptsResult | null>(null);
  const fetchingOrphanReceipts = ref(false);
  const orphanRecheck = ref<KnowledgeOrphanRecheckResult | null>(null);
  const orphanRecheckHistory = ref<KnowledgeOrphanRecheckRecord[]>([]);
  const recoveryReviews = ref<KnowledgeRecoveryReview[]>([]);
  const recoveryReviewEventHistoryStatus = ref<"verified" | "inconsistent" | "unavailable">("unavailable");
  const recoveryReviewEvents = ref<KnowledgeRecoveryReviewEvent[]>([]);
  const recoveryReviewPlan = ref<KnowledgeRecoveryReviewPlan | null>(null);
  const reviewingRecovery = ref(false);
  const recoveryReviewRequestKeys = new Map<string, string>();
  let recoveryAuthorityKey = "";
  let recoveryAuthorityQuery = 0;
  let recoveryReviewListSequence = 0;
  let recoveryReviewPlanSequence = 0;
  let recoveryReviewEventsSequence = 0;

  function resetRecoveryReviewState(clearRequestKeys = false) {
    recoveryReviewListSequence++;
    recoveryReviewPlanSequence++;
    recoveryReviewEventsSequence++;
    recoveryReviews.value = [];
    recoveryReviewPlan.value = null;
    recoveryReviewEvents.value = [];
    recoveryReviewEventHistoryStatus.value = "unavailable";
    if (clearRequestKeys) recoveryReviewRequestKeys.clear();
  }

  async function recoveryReviewAuthority() {
    const request = ++recoveryAuthorityQuery;
    try {
      const deployment = await getDeploymentConfig();
      if (request !== recoveryAuthorityQuery) throw new Error("Core 连接或审批授权查询已变化，请重试");
      if (deployment.mode !== "cloud") {
        if (recoveryAuthorityKey !== "local") {
          resetRecoveryReviewState(true);
          recoveryAuthorityKey = "local";
        }
        return { key: "local", headers: {} };
      }
      const result = await get<any>("/api/device-mesh/v1/coordination/me");
      if (request !== recoveryAuthorityQuery) throw new Error("Core 连接或审批授权查询已变化，请重试");
      const policy = result?.policy;
      const versions = [policy?.providerEpoch, policy?.modeRevision, policy?.permissionRevision];
      if (result?.canAdminister !== true || policy?.coordinated !== true || policy?.administrator !== true ||
        typeof result?.coreId !== "string" || !result.coreId ||
        versions.some((part) => !Number.isSafeInteger(part) || part < 1)) {
        throw new Error("当前设备的 Core 管理员授权不可用，请重新授权后刷新");
      }
      const key = JSON.stringify([deployment.serverURL, result.coreId, ...versions]);
      const config = coreConfigurationRequestConfig(key);
      if (recoveryAuthorityKey !== key) {
        resetRecoveryReviewState(true);
        recoveryAuthorityKey = key;
      }
      return { key, headers: config.headers };
    } catch (error) {
      if (request === recoveryAuthorityQuery) {
        resetRecoveryReviewState(true);
        recoveryAuthorityKey = "";
      }
      throw error;
    }
  }

  function reviewReadIsCurrent(key: string, sequence: number, kind: "list" | "events" | "plan") {
    const latest = kind === "list" ? recoveryReviewListSequence : kind === "events" ? recoveryReviewEventsSequence : recoveryReviewPlanSequence;
    return key === recoveryAuthorityKey && sequence === latest;
  }

  const fetchingOrphanRechecks = ref(false);
  const recheckingOrphans = ref(false);
  const pagingOrphans = ref(false);
  const auditingOrphans = ref(false);
  const auditingVectors = ref(false);
  const sloSummary = ref<KnowledgeRetrievalSLOSummary | null>(null);
  const retrievalTraces = ref<KnowledgeRetrievalTraceSummary[]>([]);
  const benchmarkRuns = ref<KnowledgeBenchmarkRunSummary[]>([]);
  const loadingDiagnostics = ref(false);

  async function fetchSources() {
    loadingSources.value = true;
    try {
      const result = await get<{ items?: KnowledgeSource[] }>("/api/knowledge/sources");
      sources.value = Array.isArray(result?.items) ? result.items : [];
      return sources.value;
    } finally {
      loadingSources.value = false;
    }
  }

  async function uploadSource(file: File, logicalName = "") {
    uploading.value = true;
    try {
      const form = new FormData();
      form.append("file", file, file.name);
      if (logicalName.trim()) form.append("logicalName", logicalName.trim());
      const result = await post<{ source?: KnowledgeSource; job?: KnowledgeJob }>(
        "/api/knowledge/sources",
        form,
      );
      await fetchSources();
      return result;
    } finally {
      uploading.value = false;
    }
  }


  async function fetchSourceDetail(id: string) {
    loadingSourceDetail.value = true;
    try {
      const result = await get<KnowledgeSourceDetail>(`/api/knowledge/sources/${encodeURIComponent(id)}`);
      sourceDetail.value = result;
      return result;
    } finally {
      loadingSourceDetail.value = false;
    }
  }

  async function uploadRevision(id: string, file: File) {
    const form = new FormData();
    form.append("file", file, file.name);
    const result = await post<{ revision?: KnowledgeSourceRevision; job?: KnowledgeJob }>(
      `/api/knowledge/sources/${encodeURIComponent(id)}/revisions`,
      form,
    );
    await Promise.all([fetchSources(), fetchJobs(id), fetchSourceDetail(id)]);
    return result;
  }

  async function reindexSource(id: string) {
    const result = await post<KnowledgeJob>(`/api/knowledge/sources/${encodeURIComponent(id)}/reindex`);
    await Promise.all([fetchSources(), fetchJobs(id), fetchSourceDetail(id)]);
    return result;
  }

  async function rollbackRevision(id: string, revision: number) {
    const result = await post<KnowledgeJob>(
      `/api/knowledge/sources/${encodeURIComponent(id)}/revisions/${revision}/rollback`,
    );
    await Promise.all([fetchSources(), fetchJobs(id), fetchSourceDetail(id)]);
    return result;
  }

  async function purgeRevision(id: string, revision: number) {
    const result = await del<KnowledgeSourceRevision>(
      `/api/knowledge/sources/${encodeURIComponent(id)}/revisions/${revision}`,
    );
    await Promise.all([fetchSources(), fetchJobs(id), fetchSourceDetail(id)]);
    return result;
  }

  async function deleteSource(id: string) {
    await del(`/api/knowledge/sources/${encodeURIComponent(id)}`);
    sources.value = sources.value.filter((item) => item.id !== id);
  }

  async function fetchJobs(sourceId = "") {
    const result = await get<{ items?: KnowledgeJob[] }>("/api/knowledge/jobs", {
      ...(sourceId ? { sourceId } : {}),
    });
    jobs.value = Array.isArray(result?.items) ? result.items : [];
    return jobs.value;
  }

  async function retryJob(id: string) {
    const result = await post<KnowledgeJob>(`/api/knowledge/jobs/${encodeURIComponent(id)}/retry`);
    await Promise.all([fetchJobs(), fetchSources()]);
    return result;
  }

  async function cancelJob(id: string) {
    const result = await post<KnowledgeJob>(`/api/knowledge/jobs/${encodeURIComponent(id)}/cancel`);
    await Promise.all([fetchJobs(), fetchSources()]);
    return result;
  }

  async function search(query: string, options?: {
    limit?: number;
    sourceIds?: string[];
    minScore?: number;
    retrievalLevel?: "auto" | "none" | "fast" | "deep" | "agentic";
    contextualQuery?: string;
    temporalMode?: "current" | "historical" | "all";
    asOf?: string;
  }) {
    searching.value = true;
    try {
      const temporalMode = options?.temporalMode ?? "current";
      const result = await post<KnowledgeSearchResponse>("/api/knowledge/search", {
        query: query.trim(),
        limit: options?.limit ?? 8,
        sourceIds: options?.sourceIds ?? [],
        minScore: options?.minScore ?? 0,
        retrievalLevel: options?.retrievalLevel ?? "auto",
        ...(options?.contextualQuery?.trim() ? { contextualQuery: options.contextualQuery.trim() } : {}),
        temporalMode,
        ...(temporalMode === "historical" && options?.asOf ? { asOf: options.asOf } : {}),
      });
      searchResult.value = result;
      return result;
    } finally {
      searching.value = false;
    }
  }

  async function fetchSourceOutline(sourceId: string, options?: { revision?: number; afterOrdinal?: number; limit?: number }) {
    return get<KnowledgeUnitOutlineResponse>(`/api/knowledge/sources/${encodeURIComponent(sourceId)}/outline`, {
      ...(options?.revision && options.revision > 0 ? { revision: options.revision } : {}),
      ...(options?.afterOrdinal !== undefined ? { afterOrdinal: options.afterOrdinal } : {}),
      limit: options?.limit ?? 100,
    });
  }

  async function read(request: KnowledgeReadRequest) {
    const payload: KnowledgeReadRequest = {
      ...request,
      evidenceId: request.evidenceId?.trim() || undefined,
      unitId: request.unitId?.trim() || undefined,
      sourceId: request.sourceId?.trim() || undefined,
    };
    return post<KnowledgeReadResponse>("/api/knowledge/read", payload);
  }

  async function fetchIndexOutboxHealth() {
    const result = await get<KnowledgeIndexOutboxHealth>("/api/knowledge/index-health");
    indexOutboxHealth.value = result;
    return result;
  }

  async function auditVectorSample(limit = 50) {
    auditingVectors.value = true;
    vectorAudit.value = null;
    try {
      const result = await get<KnowledgeVectorAuditResult>("/api/knowledge/vector-audit", { limit });
      vectorAudit.value = result;
      return result;
    } finally {
      auditingVectors.value = false;
    }
  }

  async function auditOrphanVectors(limit = 50) {
    auditingOrphans.value = true;
    orphanVectorAudit.value = null;
    try {
      const result = await get<KnowledgeOrphanVectorAuditResult>("/api/knowledge/vector-orphans", { limit });
      orphanVectorAudit.value = result;
      return result;
    } finally {
      auditingOrphans.value = false;
    }
  }

  async function fetchOrphanAuditSessions() {
    const result = await get<{ items?: KnowledgeOrphanAuditSessionSummary[] }>("/api/knowledge/vector-orphans/audits", { limit: 10 });
    orphanAuditSessions.value = Array.isArray(result?.items) ? result.items : [];
    return orphanAuditSessions.value;
  }

  async function fetchOrphanAuditReceipts(auditId: string, after = 0) {
    if (!/^[0-9a-f-]{36}$/i.test(auditId) || after < 0 || after > 4095) {
      throw new Error("无效巡检会话");
    }
    fetchingOrphanReceipts.value = true;
    try {
      const result = await get<KnowledgeOrphanAuditReceiptsResult>(`/api/knowledge/vector-orphans/audits/${auditId}/pages`, { after, limit: 25 });
      if (result.auditId !== auditId || !Array.isArray(result.items)) {
        throw new Error("巡检回执格式异常");
      }
      orphanAuditReceipts.value = result;
      return result;
    } finally {
      fetchingOrphanReceipts.value = false;
    }
  }

  async function fetchOrphanRecheckHistory(auditId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(auditId)) throw new Error("无效巡检会话");
    fetchingOrphanRechecks.value = true;
    try {
      const result = await get<{ items?: KnowledgeOrphanRecheckRecord[] }>(`/api/knowledge/vector-orphans/audits/${auditId}/rechecks`, { limit: 10 });
      orphanRecheckHistory.value = Array.isArray(result?.items) ? result.items.filter((item) => item.auditId === auditId) : [];
      return orphanRecheckHistory.value;
    } finally {
      fetchingOrphanRechecks.value = false;
    }
  }

  async function fetchRecoveryReviews(auditId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(auditId)) throw new Error("无效巡检会话");
    const authority = await recoveryReviewAuthority();
    const sequence = ++recoveryReviewListSequence;
    recoveryReviewPlan.value = null;
    try {
      const result = await get<{ items?: KnowledgeRecoveryReview[]; executable: false }>(`/api/knowledge/vector-orphans/audits/${auditId}/recovery-reviews`, { limit: 10 }, { headers: authority.headers });
      await recoveryReviewAuthority();
      if (!reviewReadIsCurrent(authority.key, sequence, "list")) throw new Error("审批授权或会话状态已变化，请重新读取");
      if (result.executable !== false) throw new Error("审批列表响应无效");
      recoveryReviews.value = Array.isArray(result.items) ? result.items.filter((item) => item.auditId === auditId) : [];
      return recoveryReviews.value;
    } catch (error) {
      if (reviewReadIsCurrent(authority.key, sequence, "list")) resetRecoveryReviewState();
      throw error;
    }
  }

  async function fetchRecoveryReviewEvents(id: string) {
    if (!/^[0-9a-f-]{36}$/i.test(id)) throw new Error("无效审查记录");
    const authority = await recoveryReviewAuthority();
    const sequence = ++recoveryReviewEventsSequence;
    recoveryReviewEvents.value = [];
    recoveryReviewEventHistoryStatus.value = "unavailable";
    try {
      const result = await get<KnowledgeRecoveryReviewEventHistory>(`/api/knowledge/vector-orphans/recovery-reviews/${id}/events`, undefined, { headers: authority.headers });
      await recoveryReviewAuthority();
      if (!reviewReadIsCurrent(authority.key, sequence, "events")) throw new Error("审批授权或会话状态已变化，请重新读取");
      if (result.reviewId !== id || result.executable !== false || !["verified", "inconsistent", "unavailable"].includes(result.historyStatus)) {
        throw new Error("操作历史响应无效");
      }
      recoveryReviewEventHistoryStatus.value = result.truncated ? "inconsistent" : result.historyStatus;
      recoveryReviewEvents.value = Array.isArray(result.items) ? result.items.filter((entry) => entry.reviewId === id) : [];
      return recoveryReviewEvents.value;
    } catch (error) {
      if (reviewReadIsCurrent(authority.key, sequence, "events")) resetRecoveryReviewState();
      throw error;
    }
  }

  async function fetchRecoveryReviewPlan(id: string) {
    if (!/^[0-9a-f-]{36}$/i.test(id)) throw new Error("无效审查记录");
    const authority = await recoveryReviewAuthority();
    const sequence = ++recoveryReviewPlanSequence;
    recoveryReviewPlan.value = null;
    try {
      const result = await get<KnowledgeRecoveryReviewPlan>(`/api/knowledge/vector-orphans/recovery-reviews/${id}/plan`, undefined, { headers: authority.headers });
      await recoveryReviewAuthority();
      if (!reviewReadIsCurrent(authority.key, sequence, "plan")) throw new Error("审批授权或会话状态已变化，请重新读取");
      if (result.reviewId !== id || result.executable !== false || result.action !== "manual_investigation") {
        throw new Error("恢复计划响应无效");
      }
      recoveryReviewPlan.value = result;
      return result;
    } catch (error) {
      if (reviewReadIsCurrent(authority.key, sequence, "plan")) resetRecoveryReviewState();
      throw error;
    }
  }

  async function requestRecoveryReview(auditId: string, recheckId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(auditId) || !/^[0-9a-f-]{36}$/i.test(recheckId)) throw new Error("无效复核记录");
    const authority = await recoveryReviewAuthority();
    const requestKey = recoveryReviewRequestKeys.get(recheckId) || crypto.randomUUID();
    recoveryReviewRequestKeys.set(recheckId, requestKey);
    reviewingRecovery.value = true;
    try {
      const result = await post<KnowledgeRecoveryReviewResponse>(`/api/knowledge/vector-orphans/audits/${auditId}/recovery-reviews`, { recheckId, requestKey }, { headers: authority.headers });
      if (authority.key !== recoveryAuthorityKey || result.executable !== false) throw new Error("审批授权已变化或响应无效，请重新读取");
      await fetchRecoveryReviews(auditId);
      return result;
    } catch (error) {
      resetRecoveryReviewState();
      await fetchRecoveryReviews(auditId).catch(() => null);
      throw error;
    } finally {
      reviewingRecovery.value = false;
    }
  }

  async function transitionRecoveryReview(id: string, auditId: string, action: "approve" | "revoke") {
    if (!/^[0-9a-f-]{36}$/i.test(id) || !/^[0-9a-f-]{36}$/i.test(auditId)) throw new Error("无效审查记录");
    const authority = await recoveryReviewAuthority();
    reviewingRecovery.value = true;
    try {
      if (action === "approve") {
        const plan = await fetchRecoveryReviewPlan(id);
        if (authority.key !== recoveryAuthorityKey || plan.status !== "pending" || plan.historyStatus !== "verified" || plan.evidenceStatus !== "valid" || !plan.manualReviewReady || !plan.canApprove) {
          throw new Error("审查证据、授权或操作轨迹已失效，请重新巡检及复核");
        }
      }
      const result = await post<KnowledgeRecoveryReviewResponse>(`/api/knowledge/vector-orphans/recovery-reviews/${id}/${action}`, undefined, { headers: authority.headers });
      if (authority.key !== recoveryAuthorityKey || result.executable !== false) throw new Error("审批授权已变化或响应无效，请重新读取");
      await fetchRecoveryReviews(auditId);
      return result;
    } catch (error) {
      resetRecoveryReviewState();
      await fetchRecoveryReviews(auditId).catch(() => null);
      throw error;
    } finally {
      reviewingRecovery.value = false;
    }
  }

  async function recheckOrphanLatest(auditId: string) {
    if (!/^[0-9a-f-]{36}$/i.test(auditId)) throw new Error("无效巡检会话");
    recheckingOrphans.value = true;
    orphanRecheck.value = null;
    try {
      const result = await get<KnowledgeOrphanRecheckResult>(`/api/knowledge/vector-orphans/audits/${auditId}/recheck`);
      if (result.auditId !== auditId || !Number.isInteger(result.page) || result.page < 1) {
        throw new Error("复核结果格式异常");
      }
      orphanRecheck.value = result;
      return result;
    } finally {
      recheckingOrphans.value = false;
    }
  }

  async function auditOrphanPage(next = false, limit = 25) {
    if (pagingOrphans.value) return;
    const cursor = next ? orphanVectorPage.value?.nextCursor : undefined;
    if (next && !cursor) return;
    pagingOrphans.value = true;
    try {
      const result = await get<KnowledgeOrphanVectorAuditPageResult>("/api/knowledge/vector-orphans/page", {
        limit,
        ...(cursor ? { cursor } : {}),
      });
      if (!result.totals || result.totals.auditId !== result.auditId || result.totals.pages !== result.page) {
        throw new Error("巡检结果缺少服务端持久化统计，请刷新页面后重试");
      }
      orphanPageTotals.value = result.totals;
      orphanVectorPage.value = result;
      await fetchOrphanAuditSessions().catch(() => null);
      return result;
    } finally {
      pagingOrphans.value = false;
    }
  }

  async function fetchDiagnostics(limit = 20) {
    loadingDiagnostics.value = true;
    try {
      const [runtime, slo, traceResult, benchmarkResult, indexHealth] = await Promise.all([
        get<KnowledgeRuntimeStatus>("/api/knowledge/runtime"),
        get<KnowledgeRetrievalSLOSummary>("/api/knowledge/slo"),
        get<{ items?: KnowledgeRetrievalTraceSummary[] }>("/api/knowledge/traces", { limit }),
        get<{ items?: KnowledgeBenchmarkRunSummary[] }>("/api/knowledge/benchmark/runs", { limit }),
        fetchIndexOutboxHealth().catch(() => null),
      ]);
      runtimeStatus.value = runtime;
      sloSummary.value = slo;
      retrievalTraces.value = Array.isArray(traceResult?.items) ? traceResult.items : [];
      benchmarkRuns.value = Array.isArray(benchmarkResult?.items) ? benchmarkResult.items : [];
      return { runtime: runtimeStatus.value, slo: sloSummary.value, indexHealth, traces: retrievalTraces.value, benchmarks: benchmarkRuns.value };
    } finally {
      loadingDiagnostics.value = false;
    }
  }

  async function reconcile() {
    reconciling.value = true;
    try {
      await post("/api/knowledge/reconcile");
      await Promise.all([fetchSources(), fetchJobs()]);
    } finally {
      reconciling.value = false;
    }
  }

  async function garbageCollect() {
    collecting.value = true;
    try {
      const result = await post<{ purged?: number }>("/api/knowledge/gc");
      await Promise.all([fetchSources(), fetchJobs()]);
      return result;
    } finally {
      collecting.value = false;
    }
  }

  return {
    sources,
    jobs,
    searchResult,
    loadingSources,
    searching,
    uploading,
    reconciling,
    collecting,
    sourceDetail,
    loadingSourceDetail,
    runtimeStatus,
    indexOutboxHealth,
    vectorAudit,
    auditingVectors,
    orphanVectorAudit,
    orphanVectorPage,
    orphanPageTotals,
    orphanAuditSessions,
    orphanAuditReceipts,
    fetchingOrphanReceipts,
    orphanRecheck,
    recheckingOrphans,
    orphanRecheckHistory,
    recoveryReviews,
    recoveryReviewEvents,
    recoveryReviewEventHistoryStatus,
    recoveryReviewPlan,
    reviewingRecovery,
    fetchingOrphanRechecks,
    pagingOrphans,
    auditingOrphans,
    sloSummary,
    retrievalTraces,
    benchmarkRuns,
    loadingDiagnostics,
    fetchSources,
    fetchSourceDetail,
    uploadSource,
    uploadRevision,
    reindexSource,
    rollbackRevision,
    purgeRevision,
    deleteSource,
    fetchJobs,
    retryJob,
    cancelJob,
    search,
    fetchSourceOutline,
    read,
    fetchDiagnostics,
    fetchIndexOutboxHealth,
    auditVectorSample,
    auditOrphanVectors,
    auditOrphanPage,
    fetchOrphanAuditSessions,
    fetchOrphanAuditReceipts,
    recheckOrphanLatest,
    fetchOrphanRecheckHistory,
    fetchRecoveryReviews,
    fetchRecoveryReviewEvents,
    fetchRecoveryReviewPlan,
    requestRecoveryReview,
    transitionRecoveryReview,
    reconcile,
    garbageCollect,
  };
}
