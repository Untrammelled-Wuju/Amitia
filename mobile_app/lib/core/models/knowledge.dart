class KnowledgeSourceDto {
  final String id;
  final String logicalName;
  final String filename;
  final String mimeType;
  final String sourceType;
  final int sizeBytes;
  final String sha256;
  final int revision;
  final String parserVersion;
  final String chunkSchemaVersion;
  final String embeddingVersion;
  final String indexVersion;
  final String status;
  final int expectedUnits;
  final int processedUnits;
  final int failedUnits;
  final int indexedChunks;
  final String lastError;
  final String createdAt;
  final String updatedAt;

  const KnowledgeSourceDto({
    required this.id,
    required this.logicalName,
    required this.filename,
    required this.mimeType,
    required this.sourceType,
    required this.sizeBytes,
    required this.sha256,
    required this.revision,
    required this.parserVersion,
    required this.chunkSchemaVersion,
    required this.embeddingVersion,
    required this.indexVersion,
    required this.status,
    required this.expectedUnits,
    required this.processedUnits,
    required this.failedUnits,
    required this.indexedChunks,
    required this.lastError,
    required this.createdAt,
    required this.updatedAt,
  });

  factory KnowledgeSourceDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeSourceDto(
        id: (json['id'] ?? '').toString(),
        logicalName: (json['logicalName'] ?? '').toString(),
        filename: (json['filename'] ?? '').toString(),
        mimeType: (json['mimeType'] ?? '').toString(),
        sourceType: (json['sourceType'] ?? '').toString(),
        sizeBytes: (json['sizeBytes'] as num?)?.toInt() ?? 0,
        sha256: (json['sha256'] ?? '').toString(),
        revision: (json['revision'] as num?)?.toInt() ?? 0,
        parserVersion: (json['parserVersion'] ?? '').toString(),
        chunkSchemaVersion: (json['chunkSchemaVersion'] ?? '').toString(),
        embeddingVersion: (json['embeddingVersion'] ?? '').toString(),
        indexVersion: (json['indexVersion'] ?? '').toString(),
        status: (json['status'] ?? '').toString(),
        expectedUnits: (json['expectedUnits'] as num?)?.toInt() ?? 0,
        processedUnits: (json['processedUnits'] as num?)?.toInt() ?? 0,
        failedUnits: (json['failedUnits'] as num?)?.toInt() ?? 0,
        indexedChunks: (json['indexedChunks'] as num?)?.toInt() ?? 0,
        lastError: (json['lastError'] ?? '').toString(),
        createdAt: (json['createdAt'] ?? '').toString(),
        updatedAt: (json['updatedAt'] ?? '').toString(),
      );

  bool get isReady => status == 'ready' || status == 'ready_degraded';
  bool get isBusy => status == 'pending' || status == 'processing';
  double get progress {
    if (expectedUnits <= 0) return isReady ? 1 : 0;
    return (processedUnits / expectedUnits).clamp(0, 1).toDouble();
  }
}

class KnowledgeSourceRevisionDto {
  final String id;
  final String sourceId;
  final int revision;
  final String filename;
  final String mimeType;
  final String sourceType;
  final int sizeBytes;
  final String sha256;
  final String parserVersion;
  final String chunkSchemaVersion;
  final String embeddingVersion;
  final String indexVersion;
  final String status;
  final int expectedUnits;
  final int processedUnits;
  final int failedUnits;
  final int indexedChunks;
  final String lastError;
  final String createdAt;
  final String updatedAt;
  final String activatedAt;
  final String retainUntil;
  final String purgedAt;
  final bool isActive;
  final bool canRollback;
  final bool canPurge;

  const KnowledgeSourceRevisionDto({
    required this.id,
    required this.sourceId,
    required this.revision,
    required this.filename,
    required this.mimeType,
    required this.sourceType,
    required this.sizeBytes,
    required this.sha256,
    required this.parserVersion,
    required this.chunkSchemaVersion,
    required this.embeddingVersion,
    required this.indexVersion,
    required this.status,
    required this.expectedUnits,
    required this.processedUnits,
    required this.failedUnits,
    required this.indexedChunks,
    required this.lastError,
    required this.createdAt,
    required this.updatedAt,
    required this.activatedAt,
    required this.retainUntil,
    required this.purgedAt,
    required this.isActive,
    required this.canRollback,
    required this.canPurge,
  });

  factory KnowledgeSourceRevisionDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeSourceRevisionDto(
        id: (json['id'] ?? '').toString(),
        sourceId: (json['sourceId'] ?? '').toString(),
        revision: (json['revision'] as num?)?.toInt() ?? 0,
        filename: (json['filename'] ?? '').toString(),
        mimeType: (json['mimeType'] ?? '').toString(),
        sourceType: (json['sourceType'] ?? '').toString(),
        sizeBytes: (json['sizeBytes'] as num?)?.toInt() ?? 0,
        sha256: (json['sha256'] ?? '').toString(),
        parserVersion: (json['parserVersion'] ?? '').toString(),
        chunkSchemaVersion: (json['chunkSchemaVersion'] ?? '').toString(),
        embeddingVersion: (json['embeddingVersion'] ?? '').toString(),
        indexVersion: (json['indexVersion'] ?? '').toString(),
        status: (json['status'] ?? '').toString(),
        expectedUnits: (json['expectedUnits'] as num?)?.toInt() ?? 0,
        processedUnits: (json['processedUnits'] as num?)?.toInt() ?? 0,
        failedUnits: (json['failedUnits'] as num?)?.toInt() ?? 0,
        indexedChunks: (json['indexedChunks'] as num?)?.toInt() ?? 0,
        lastError: (json['lastError'] ?? '').toString(),
        createdAt: (json['createdAt'] ?? '').toString(),
        updatedAt: (json['updatedAt'] ?? '').toString(),
        activatedAt: (json['activatedAt'] ?? '').toString(),
        retainUntil: (json['retainUntil'] ?? '').toString(),
        purgedAt: (json['purgedAt'] ?? '').toString(),
        isActive: json['isActive'] == true,
        canRollback: json['canRollback'] == true,
        canPurge: json['canPurge'] == true,
      );
}

class KnowledgeSourceDetailDto {
  final KnowledgeSourceDto source;
  final List<KnowledgeSourceRevisionDto> revisions;
  final KnowledgeJobDto? latestJob;
  final int unitCount;
  final int chunkCount;
  final String indexHealth;
  final bool hasActiveJob;

  const KnowledgeSourceDetailDto({
    required this.source,
    required this.revisions,
    required this.latestJob,
    required this.unitCount,
    required this.chunkCount,
    required this.indexHealth,
    required this.hasActiveJob,
  });

  factory KnowledgeSourceDetailDto.fromJson(Map<String, dynamic> json) {
    final source = Map<String, dynamic>.from(
      json['source'] as Map? ?? const {},
    );
    final revisions = (json['revisions'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) => KnowledgeSourceRevisionDto.fromJson(
            Map<String, dynamic>.from(item),
          ),
        )
        .toList(growable: false);
    final latest = json['latestJob'];
    return KnowledgeSourceDetailDto(
      source: KnowledgeSourceDto.fromJson(source),
      revisions: revisions,
      latestJob: latest is Map
          ? KnowledgeJobDto.fromJson(Map<String, dynamic>.from(latest))
          : null,
      unitCount: (json['unitCount'] as num?)?.toInt() ?? 0,
      chunkCount: (json['chunkCount'] as num?)?.toInt() ?? 0,
      indexHealth: (json['indexHealth'] ?? '').toString(),
      hasActiveJob: json['hasActiveJob'] == true,
    );
  }
}

class KnowledgeJobDto {
  final String id;
  final String sourceId;
  final int sourceRevision;
  final String kind;
  final String stage;
  final String status;
  final int lastCommittedOrdinal;
  final int expectedUnits;
  final int processedUnits;
  final int failedUnits;
  final int indexedChunks;
  final int retryCount;
  final String lastError;
  final String createdAt;
  final String updatedAt;
  final String finishedAt;

  const KnowledgeJobDto({
    required this.id,
    required this.sourceId,
    required this.sourceRevision,
    required this.kind,
    required this.stage,
    required this.status,
    required this.lastCommittedOrdinal,
    required this.expectedUnits,
    required this.processedUnits,
    required this.failedUnits,
    required this.indexedChunks,
    required this.retryCount,
    required this.lastError,
    required this.createdAt,
    required this.updatedAt,
    required this.finishedAt,
  });

  factory KnowledgeJobDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeJobDto(
        id: (json['id'] ?? '').toString(),
        sourceId: (json['sourceId'] ?? '').toString(),
        sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
        kind: (json['kind'] ?? 'ingest').toString(),
        stage: (json['stage'] ?? '').toString(),
        status: (json['status'] ?? '').toString(),
        lastCommittedOrdinal:
            (json['lastCommittedOrdinal'] as num?)?.toInt() ?? 0,
        expectedUnits: (json['expectedUnits'] as num?)?.toInt() ?? 0,
        processedUnits: (json['processedUnits'] as num?)?.toInt() ?? 0,
        failedUnits: (json['failedUnits'] as num?)?.toInt() ?? 0,
        indexedChunks: (json['indexedChunks'] as num?)?.toInt() ?? 0,
        retryCount: (json['retryCount'] as num?)?.toInt() ?? 0,
        lastError: (json['lastError'] ?? '').toString(),
        createdAt: (json['createdAt'] ?? '').toString(),
        updatedAt: (json['updatedAt'] ?? '').toString(),
        finishedAt: (json['finishedAt'] ?? '').toString(),
      );
}

class KnowledgeEvidenceDto {
  final String id;
  final String unitId;
  final String evidenceKind;
  final List<String> coverageKeys;
  final String checksum;
  final String sourceType;
  final String sourceId;
  final int sourceRevision;
  final String title;
  final String content;
  final Map<String, dynamic> locator;
  final double denseScore;
  final double sparseScore;
  final double fusionScore;
  final double rerankScore;
  final double authority;
  final double confidence;
  final String trust;
  final String validFrom;
  final String validTo;
  final List<String> supersedes;
  final String conflictGroup;
  final Map<String, dynamic> metadata;

  const KnowledgeEvidenceDto({
    required this.id,
    required this.unitId,
    required this.evidenceKind,
    required this.coverageKeys,
    required this.checksum,
    required this.sourceType,
    required this.sourceId,
    required this.sourceRevision,
    required this.title,
    required this.content,
    required this.locator,
    required this.denseScore,
    required this.sparseScore,
    required this.fusionScore,
    required this.rerankScore,
    required this.authority,
    required this.confidence,
    required this.trust,
    required this.validFrom,
    required this.validTo,
    required this.supersedes,
    required this.conflictGroup,
    required this.metadata,
  });

  factory KnowledgeEvidenceDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeEvidenceDto(
        id: (json['id'] ?? '').toString(),
        unitId: (json['unitId'] ?? '').toString(),
        evidenceKind: (json['evidenceKind'] ?? '').toString(),
        coverageKeys: (json['coverageKeys'] as List? ?? const [])
            .map((item) => item.toString())
            .toList(growable: false),
        checksum: (json['checksum'] ?? '').toString(),
        sourceType: (json['sourceType'] ?? '').toString(),
        sourceId: (json['sourceId'] ?? '').toString(),
        sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
        title: (json['title'] ?? '').toString(),
        content: (json['content'] ?? '').toString(),
        locator: Map<String, dynamic>.from(json['locator'] as Map? ?? const {}),
        denseScore: (json['denseScore'] as num?)?.toDouble() ?? 0,
        sparseScore: (json['sparseScore'] as num?)?.toDouble() ?? 0,
        fusionScore: (json['fusionScore'] as num?)?.toDouble() ?? 0,
        rerankScore: (json['rerankScore'] as num?)?.toDouble() ?? 0,
        authority: (json['authority'] as num?)?.toDouble() ?? 0,
        confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
        trust: (json['trust'] ?? '').toString(),
        validFrom: (json['validFrom'] ?? '').toString(),
        validTo: (json['validTo'] ?? '').toString(),
        supersedes: (json['supersedes'] as List? ?? const [])
            .map((item) => item.toString())
            .toList(growable: false),
        conflictGroup: (json['conflictGroup'] ?? '').toString(),
        metadata: Map<String, dynamic>.from(
          json['metadata'] as Map? ?? const {},
        ),
      );
}

class KnowledgeReadRequestDto {
  final String evidenceId;
  final String unitId;
  final String sourceId;
  final int sourceRevision;
  final Map<String, dynamic> locator;
  final int beforeUnits;
  final int afterUnits;
  final int maxUnits;
  final int maxRunes;

  const KnowledgeReadRequestDto({
    this.evidenceId = '',
    this.unitId = '',
    this.sourceId = '',
    this.sourceRevision = 0,
    this.locator = const {},
    this.beforeUnits = 0,
    this.afterUnits = 0,
    this.maxUnits = 0,
    this.maxRunes = 0,
  });

  Map<String, dynamic> toJson() => {
    if (evidenceId.trim().isNotEmpty) 'evidenceId': evidenceId.trim(),
    if (unitId.trim().isNotEmpty) 'unitId': unitId.trim(),
    if (sourceId.trim().isNotEmpty) 'sourceId': sourceId.trim(),
    if (sourceRevision > 0) 'sourceRevision': sourceRevision,
    if (locator.isNotEmpty) 'locator': locator,
    if (beforeUnits > 0) 'beforeUnits': beforeUnits,
    if (afterUnits > 0) 'afterUnits': afterUnits,
    if (maxUnits > 0) 'maxUnits': maxUnits,
    if (maxRunes > 0) 'maxRunes': maxRunes,
  };
}

class KnowledgeReadUnitDto {
  final String id;
  final String sourceId;
  final int sourceRevision;
  final int ordinal;
  final String unitType;
  final String title;
  final String content;
  final Map<String, dynamic> locator;
  final Map<String, dynamic> metadata;
  final String checksum;
  final bool truncated;

  const KnowledgeReadUnitDto({
    required this.id,
    required this.sourceId,
    required this.sourceRevision,
    required this.ordinal,
    required this.unitType,
    required this.title,
    required this.content,
    required this.locator,
    required this.metadata,
    required this.checksum,
    required this.truncated,
  });

  factory KnowledgeReadUnitDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeReadUnitDto(
        id: (json['id'] ?? '').toString(),
        sourceId: (json['sourceId'] ?? '').toString(),
        sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
        ordinal: (json['ordinal'] as num?)?.toInt() ?? 0,
        unitType: (json['unitType'] ?? '').toString(),
        title: (json['title'] ?? '').toString(),
        content: (json['content'] ?? '').toString(),
        locator: Map<String, dynamic>.from(json['locator'] as Map? ?? const {}),
        metadata: Map<String, dynamic>.from(
          json['metadata'] as Map? ?? const {},
        ),
        checksum: (json['checksum'] ?? '').toString(),
        truncated: json['truncated'] == true,
      );
}

class KnowledgeReadResponseDto {
  final String sourceId;
  final int sourceRevision;
  final String anchorUnitId;
  final List<KnowledgeReadUnitDto> units;
  final int totalRunes;
  final bool truncated;

  const KnowledgeReadResponseDto({
    required this.sourceId,
    required this.sourceRevision,
    required this.anchorUnitId,
    required this.units,
    required this.totalRunes,
    required this.truncated,
  });

  factory KnowledgeReadResponseDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeReadResponseDto(
        sourceId: (json['sourceId'] ?? '').toString(),
        sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
        anchorUnitId: (json['anchorUnitId'] ?? '').toString(),
        units: (json['units'] as List? ?? const [])
            .whereType<Map>()
            .map(
              (item) => KnowledgeReadUnitDto.fromJson(
                Map<String, dynamic>.from(item),
              ),
            )
            .toList(growable: false),
        totalRunes: (json['totalRunes'] as num?)?.toInt() ?? 0,
        truncated: json['truncated'] == true,
      );
}

class KnowledgeSearchResultDto {
  final List<KnowledgeEvidenceDto> evidence;
  final bool sufficient;
  final String answerState;
  final String sufficiencyReason;
  final String query;
  final String originalQuery;
  final String effectiveQuery;
  final String queryComplexity;
  final String requestedRetrievalLevel;
  final String retrievalLevel;
  final String routingMode;
  final String policyMode;
  final String routeReason;
  final String shadowLevel;
  final String shadowReason;
  final bool routeDegraded;
  final String routeDegradationReason;
  final List<String> queryHistory;
  final bool degraded;
  final String temporalMode;
  final String asOf;
  final bool ambiguous;
  final List<String> conflictGroups;
  final List<String> warnings;

  const KnowledgeSearchResultDto({
    required this.evidence,
    required this.sufficient,
    required this.answerState,
    required this.sufficiencyReason,
    required this.query,
    required this.originalQuery,
    required this.effectiveQuery,
    required this.queryComplexity,
    required this.requestedRetrievalLevel,
    required this.retrievalLevel,
    required this.routingMode,
    required this.policyMode,
    required this.routeReason,
    required this.shadowLevel,
    required this.shadowReason,
    required this.routeDegraded,
    required this.routeDegradationReason,
    required this.queryHistory,
    required this.degraded,
    required this.temporalMode,
    required this.asOf,
    required this.ambiguous,
    required this.conflictGroups,
    required this.warnings,
  });

  factory KnowledgeSearchResultDto.fromJson(Map<String, dynamic> json) {
    final evidence = (json['evidence'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) =>
              KnowledgeEvidenceDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false);
    return KnowledgeSearchResultDto(
      evidence: evidence,
      sufficient: json['sufficient'] == true,
      answerState:
          (json['answerState'] ??
                  (json['sufficient'] == true ? 'sufficient' : 'insufficient'))
              .toString(),
      sufficiencyReason: (json['sufficiencyReason'] ?? '').toString(),
      query: (json['query'] ?? '').toString(),
      originalQuery: (json['originalQuery'] ?? json['query'] ?? '').toString(),
      effectiveQuery: (json['effectiveQuery'] ?? json['query'] ?? '')
          .toString(),
      queryComplexity: (json['queryComplexity'] ?? 'simple').toString(),
      requestedRetrievalLevel:
          (json['requestedRetrievalLevel'] ?? json['retrievalLevel'] ?? 'auto')
              .toString(),
      retrievalLevel: (json['retrievalLevel'] ?? 'fast').toString(),
      routingMode: (json['routingMode'] ?? 'diagnostic').toString(),
      policyMode: (json['policyMode'] ?? '').toString(),
      routeReason: (json['routeReason'] ?? '').toString(),
      shadowLevel: (json['shadowLevel'] ?? '').toString(),
      shadowReason: (json['shadowReason'] ?? '').toString(),
      routeDegraded: json['routeDegraded'] == true,
      routeDegradationReason: (json['routeDegradationReason'] ?? '').toString(),
      queryHistory: (json['queryHistory'] as List? ?? const [])
          .map((item) => item.toString())
          .toList(growable: false),
      degraded: json['degraded'] == true,
      temporalMode: (json['temporalMode'] ?? 'current').toString(),
      asOf: (json['asOf'] ?? '').toString(),
      ambiguous: json['ambiguous'] == true,
      conflictGroups: (json['conflictGroups'] as List? ?? const [])
          .map((item) => item.toString())
          .toList(growable: false),
      warnings: (json['warnings'] as List? ?? const [])
          .map((item) => item.toString())
          .toList(growable: false),
    );
  }
}

class KnowledgeRuntimeStatusDto {
  final String configHash;
  final bool retrievalPolicyEnabled;
  final bool retrievalPolicyShadow;
  final bool canonicalReaderEnabled;
  final bool agenticInvestigationEnabled;
  final int readerWorkers;
  final int retrievalTraceRetentionHours;
  final int benchmarkRetentionHours;
  final int diagnosticCleanupBatchSize;

  const KnowledgeRuntimeStatusDto({
    required this.configHash,
    required this.retrievalPolicyEnabled,
    required this.retrievalPolicyShadow,
    required this.canonicalReaderEnabled,
    required this.agenticInvestigationEnabled,
    required this.readerWorkers,
    required this.retrievalTraceRetentionHours,
    required this.benchmarkRetentionHours,
    required this.diagnosticCleanupBatchSize,
  });

  factory KnowledgeRuntimeStatusDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeRuntimeStatusDto(
        configHash: (json['configHash'] ?? '').toString(),
        retrievalPolicyEnabled: json['retrievalPolicyEnabled'] == true,
        retrievalPolicyShadow: json['retrievalPolicyShadow'] == true,
        canonicalReaderEnabled: json['canonicalReaderEnabled'] == true,
        agenticInvestigationEnabled:
            json['agenticInvestigationEnabled'] == true,
        readerWorkers: (json['readerWorkers'] as num?)?.toInt() ?? 0,
        retrievalTraceRetentionHours:
            (json['retrievalTraceRetentionHours'] as num?)?.toInt() ?? 0,
        benchmarkRetentionHours:
            (json['benchmarkRetentionHours'] as num?)?.toInt() ?? 0,
        diagnosticCleanupBatchSize:
            (json['diagnosticCleanupBatchSize'] as num?)?.toInt() ?? 0,
      );
}

class KnowledgeIndexHealthDto {
  final String status;
  final int pendingUpserts;
  final int pendingDeletes;
  final int retryingUpserts;
  final int retryingDeletes;
  final int exhaustedUpserts;
  final int exhaustedDeletes;
  final int activeLeases;
  final int overdueOperations;
  final int oldestOutstandingSeconds;
  final String observedAt;

  const KnowledgeIndexHealthDto({
    required this.status,
    required this.pendingUpserts,
    required this.pendingDeletes,
    required this.retryingUpserts,
    required this.retryingDeletes,
    required this.exhaustedUpserts,
    required this.exhaustedDeletes,
    required this.activeLeases,
    required this.overdueOperations,
    required this.oldestOutstandingSeconds,
    required this.observedAt,
  });

  factory KnowledgeIndexHealthDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeIndexHealthDto(
        status: (json['status'] ?? '').toString(),
        pendingUpserts: (json['pendingUpserts'] as num?)?.toInt() ?? 0,
        pendingDeletes: (json['pendingDeletes'] as num?)?.toInt() ?? 0,
        retryingUpserts: (json['retryingUpserts'] as num?)?.toInt() ?? 0,
        retryingDeletes: (json['retryingDeletes'] as num?)?.toInt() ?? 0,
        exhaustedUpserts: (json['exhaustedUpserts'] as num?)?.toInt() ?? 0,
        exhaustedDeletes: (json['exhaustedDeletes'] as num?)?.toInt() ?? 0,
        activeLeases: (json['activeLeases'] as num?)?.toInt() ?? 0,
        overdueOperations: (json['overdueOperations'] as num?)?.toInt() ?? 0,
        oldestOutstandingSeconds:
            (json['oldestOutstandingSeconds'] as num?)?.toInt() ?? 0,
        observedAt: (json['observedAt'] ?? '').toString(),
      );
}

class KnowledgeVectorAuditDto {
  final String status;
  final int sampled;
  final int missing;
  final int mismatched;
  final int collectionsChecked;
  final int sampleLimit;
  final bool exhaustive;
  final String observedAt;

  const KnowledgeVectorAuditDto({
    required this.status,
    required this.sampled,
    required this.missing,
    required this.mismatched,
    required this.collectionsChecked,
    required this.sampleLimit,
    required this.exhaustive,
    required this.observedAt,
  });

  factory KnowledgeVectorAuditDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeVectorAuditDto(
        status: (json['status'] ?? '').toString(),
        sampled: (json['sampled'] as num?)?.toInt() ?? 0,
        missing: (json['missing'] as num?)?.toInt() ?? 0,
        mismatched: (json['mismatched'] as num?)?.toInt() ?? 0,
        collectionsChecked: (json['collectionsChecked'] as num?)?.toInt() ?? 0,
        sampleLimit: (json['sampleLimit'] as num?)?.toInt() ?? 0,
        exhaustive: json['exhaustive'] == true,
        observedAt: (json['observedAt'] ?? '').toString(),
      );
}

class KnowledgeOrphanAuditDto {
  final String status;
  final String auditId;
  final int page;
  final int sampled;
  final int suspectedOrphans;
  final int metadataMismatches;
  final int inFlight;
  final int collectionsChecked;
  final int collectionsKnown;
  final int sampleLimit;
  final bool exhaustive;
  final bool finished;
  final String nextCursor;
  final String recheckId;
  final bool manualRecoveryAllowed;
  final String observedAt;

  const KnowledgeOrphanAuditDto({
    required this.status,
    required this.auditId,
    required this.page,
    required this.sampled,
    required this.suspectedOrphans,
    required this.metadataMismatches,
    required this.inFlight,
    required this.collectionsChecked,
    required this.collectionsKnown,
    required this.sampleLimit,
    required this.exhaustive,
    required this.finished,
    required this.nextCursor,
    required this.recheckId,
    required this.manualRecoveryAllowed,
    required this.observedAt,
  });

  factory KnowledgeOrphanAuditDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeOrphanAuditDto(
        status: (json['status'] ?? '').toString(),
        auditId: (json['auditId'] ?? '').toString(),
        page: ((json['page'] ?? json['pages']) as num?)?.toInt() ?? 0,
        sampled: (json['sampled'] as num?)?.toInt() ?? 0,
        suspectedOrphans: (json['suspectedOrphans'] as num?)?.toInt() ?? 0,
        metadataMismatches: (json['metadataMismatches'] as num?)?.toInt() ?? 0,
        inFlight: (json['inFlight'] as num?)?.toInt() ?? 0,
        collectionsChecked: (json['collectionsChecked'] as num?)?.toInt() ?? 0,
        collectionsKnown: (json['collectionsKnown'] as num?)?.toInt() ?? 0,
        sampleLimit: (json['sampleLimit'] as num?)?.toInt() ?? 0,
        exhaustive: json['exhaustive'] == true,
        finished: json['finished'] == true,
        nextCursor: (json['nextCursor'] ?? '').toString(),
        recheckId: (json['recheckId'] ?? '').toString(),
        manualRecoveryAllowed: json['manualRecoveryAllowed'] == true,
        observedAt: (json['observedAt'] ?? '').toString(),
      );
}

class KnowledgeSloLevelDto {
  final String level;
  final int samples;
  final int errorCount;
  final int exceededCount;
  final int p50Ms;
  final int p95Ms;
  final int targetMs;
  final bool enoughSamples;
  final bool targetMet;

  const KnowledgeSloLevelDto({
    required this.level,
    required this.samples,
    required this.errorCount,
    required this.exceededCount,
    required this.p50Ms,
    required this.p95Ms,
    required this.targetMs,
    required this.enoughSamples,
    required this.targetMet,
  });

  factory KnowledgeSloLevelDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeSloLevelDto(
        level: (json['level'] ?? '').toString(),
        samples: (json['samples'] as num?)?.toInt() ?? 0,
        errorCount: (json['errorCount'] as num?)?.toInt() ?? 0,
        exceededCount: (json['exceededCount'] as num?)?.toInt() ?? 0,
        p50Ms: (json['p50Ms'] as num?)?.toInt() ?? 0,
        p95Ms: (json['p95Ms'] as num?)?.toInt() ?? 0,
        targetMs: (json['targetMs'] as num?)?.toInt() ?? 0,
        enoughSamples: json['enoughSamples'] == true,
        targetMet: json['targetMet'] == true,
      );
}

class KnowledgeSloSummaryDto {
  final String configHash;
  final int windowMinutes;
  final int minSamples;
  final String from;
  final String to;
  final List<KnowledgeSloLevelDto> levels;

  const KnowledgeSloSummaryDto({
    required this.configHash,
    required this.windowMinutes,
    required this.minSamples,
    required this.from,
    required this.to,
    required this.levels,
  });

  factory KnowledgeSloSummaryDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeSloSummaryDto(
        configHash: (json['configHash'] ?? '').toString(),
        windowMinutes: (json['windowMinutes'] as num?)?.toInt() ?? 0,
        minSamples: (json['minSamples'] as num?)?.toInt() ?? 0,
        from: (json['from'] ?? '').toString(),
        to: (json['to'] ?? '').toString(),
        levels: (json['levels'] as List? ?? const [])
            .whereType<Map>()
            .map(
              (item) => KnowledgeSloLevelDto.fromJson(
                Map<String, dynamic>.from(item),
              ),
            )
            .toList(growable: false),
      );
}

class KnowledgeRetrievalTraceDto {
  final String id;
  final int candidateCount;
  final int selectedCount;
  final bool sufficient;
  final int durationMs;
  final String errorCode;
  final bool degraded;
  final bool ambiguous;
  final String level;
  final String policyMode;
  final String routeReason;
  final String shadowLevel;
  final int sloTargetMs;
  final bool sloExceeded;
  final String createdAt;
  final Map<String, dynamic> raw;

  const KnowledgeRetrievalTraceDto({
    required this.id,
    required this.candidateCount,
    required this.selectedCount,
    required this.sufficient,
    required this.durationMs,
    required this.errorCode,
    required this.degraded,
    required this.ambiguous,
    required this.level,
    required this.policyMode,
    required this.routeReason,
    required this.shadowLevel,
    required this.sloTargetMs,
    required this.sloExceeded,
    required this.createdAt,
    required this.raw,
  });

  factory KnowledgeRetrievalTraceDto.fromJson(Map<String, dynamic> json) {
    final queryPlan = Map<String, dynamic>.from(
      json['queryPlan'] as Map? ?? const {},
    );
    return KnowledgeRetrievalTraceDto(
      id: (json['id'] ?? '').toString(),
      candidateCount: (json['candidateCount'] as num?)?.toInt() ?? 0,
      selectedCount: (json['selectedCount'] as num?)?.toInt() ?? 0,
      sufficient: json['sufficient'] == true,
      durationMs: (json['durationMs'] as num?)?.toInt() ?? 0,
      errorCode: (json['errorCode'] ?? '').toString(),
      degraded: json['degraded'] == true,
      ambiguous: json['ambiguous'] == true,
      level: (queryPlan['level'] ?? '').toString(),
      policyMode: (json['policyMode'] ?? '').toString(),
      routeReason: (json['routeReason'] ?? '').toString(),
      shadowLevel: (json['shadowLevel'] ?? '').toString(),
      sloTargetMs: (json['sloTargetMs'] as num?)?.toInt() ?? 0,
      sloExceeded: json['sloExceeded'] == true,
      createdAt: (json['createdAt'] ?? '').toString(),
      raw: Map<String, dynamic>.from(json),
    );
  }
}

class KnowledgeBenchmarkRunDto {
  final String id;
  final bool passed;
  final int samples;
  final int errorCount;
  final double recallAtK;
  final double mrr;
  final double ndcgAtK;
  final double noAnswerAccuracy;
  final int p50LatencyMs;
  final int p95LatencyMs;
  final String createdAt;

  const KnowledgeBenchmarkRunDto({
    required this.id,
    required this.passed,
    required this.samples,
    required this.errorCount,
    required this.recallAtK,
    required this.mrr,
    required this.ndcgAtK,
    required this.noAnswerAccuracy,
    required this.p50LatencyMs,
    required this.p95LatencyMs,
    required this.createdAt,
  });

  factory KnowledgeBenchmarkRunDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeBenchmarkRunDto(
        id: (json['id'] ?? '').toString(),
        passed: json['passed'] == true,
        samples: (json['cases'] as num?)?.toInt() ?? 0,
        errorCount: (json['errorCases'] as num?)?.toInt() ?? 0,
        recallAtK: (json['recallAtK'] as num?)?.toDouble() ?? 0,
        mrr: (json['mrr'] as num?)?.toDouble() ?? 0,
        ndcgAtK: (json['ndcgAtK'] as num?)?.toDouble() ?? 0,
        noAnswerAccuracy: (json['noAnswerAccuracy'] as num?)?.toDouble() ?? 0,
        p50LatencyMs: (json['p50LatencyMs'] as num?)?.toInt() ?? 0,
        p95LatencyMs: (json['p95LatencyMs'] as num?)?.toInt() ?? 0,
        createdAt: (json['createdAt'] ?? '').toString(),
      );
}

class KnowledgeOutlineItemDto {
  final String id;
  final String sourceId;
  final int sourceRevision;
  final int ordinal;
  final String unitType;
  final String title;
  final Map<String, dynamic> locator;
  final String checksum;

  const KnowledgeOutlineItemDto({
    required this.id,
    required this.sourceId,
    required this.sourceRevision,
    required this.ordinal,
    required this.unitType,
    required this.title,
    required this.locator,
    required this.checksum,
  });

  factory KnowledgeOutlineItemDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeOutlineItemDto(
        id: (json['id'] ?? '').toString(),
        sourceId: (json['sourceId'] ?? '').toString(),
        sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
        ordinal: (json['ordinal'] as num?)?.toInt() ?? 0,
        unitType: (json['unitType'] ?? '').toString(),
        title: (json['title'] ?? '').toString(),
        locator: Map<String, dynamic>.from(json['locator'] as Map? ?? const {}),
        checksum: (json['checksum'] ?? '').toString(),
      );
}

class KnowledgeOutlineResponseDto {
  final String sourceId;
  final int sourceRevision;
  final List<KnowledgeOutlineItemDto> items;
  final int? nextAfterOrdinal;

  const KnowledgeOutlineResponseDto({
    required this.sourceId,
    required this.sourceRevision,
    required this.items,
    required this.nextAfterOrdinal,
  });

  factory KnowledgeOutlineResponseDto.fromJson(
    Map<String, dynamic> json,
  ) => KnowledgeOutlineResponseDto(
    sourceId: (json['sourceId'] ?? '').toString(),
    sourceRevision: (json['sourceRevision'] as num?)?.toInt() ?? 0,
    items: (json['items'] as List? ?? const [])
        .whereType<Map>()
        .map(
          (item) =>
              KnowledgeOutlineItemDto.fromJson(Map<String, dynamic>.from(item)),
        )
        .toList(growable: false),
    nextAfterOrdinal: (json['nextAfterOrdinal'] as num?)?.toInt(),
  );
}

class KnowledgeRecoveryReviewDto {
  final String id;
  final String auditId;
  final String displayStatus;
  final String status;
  final String expiresAt;

  const KnowledgeRecoveryReviewDto({
    required this.id,
    required this.auditId,
    required this.displayStatus,
    required this.status,
    required this.expiresAt,
  });

  factory KnowledgeRecoveryReviewDto.fromJson(Map<String, dynamic> json) =>
      KnowledgeRecoveryReviewDto(
        id: (json['id'] ?? '').toString(),
        auditId: (json['auditId'] ?? '').toString(),
        displayStatus: (json['displayStatus'] ?? '').toString(),
        status: (json['status'] ?? '').toString(),
        expiresAt: (json['expiresAt'] ?? '').toString(),
      );
}
