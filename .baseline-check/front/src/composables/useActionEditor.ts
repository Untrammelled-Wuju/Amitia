// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "./useApi";

const BASE = "/api/desktop-pets";

export interface RevisionSummary {
  id: string;
  revisionNumber: number;
  revisionType: string;
  status: string;
  frameCount: number;
  durationMs: number;
  defaultFps: number;
  loopType: string;
  qualityVerdict: string;
  changeSummary: string;
  parentRevisionId: string;
  isActive: boolean;
  createdAt: string;
}

export interface FrameAsset {
  id: string;
  contentHash: string;
  mimeType: string;
  width: number;
  height: number;
  sourceType: string;
  sourceRefId: string;
}

export interface ActionRevisionFrame {
  id: string;
  revisionId: string;
  frameId: string;
  assetId: string;
  logicalIndex: number;
  durationMs: number;
  anchorX: number;
  anchorY: number;
  anchorSpace: string;
  lineageType: string;
  sourceFrameId: string;
  sourceRevisionId: string;
}

export interface RevisionDetail {
  revision: any;
  frames: ActionRevisionFrame[];
  assets: FrameAsset[];
  manifest?: any;
}

export interface FrameTimelineItem {
  frameId: string;
  logicalIndex: number;
  assetId: string;
  contentHash: string;
  durationMs: number;
  sourceType: string;
  width: number;
  height: number;
  anchorX: number;
  anchorY: number;
  hasQualityIssue: boolean;
}

export interface ActionEditSummary {
  actionKey: string;
  activeRevisionId: string;
  activeRevisionNum: number;
  bindingVersion: number;
  frameCount: number;
  durationMs: number;
  qualityVerdict: string;
  hasOpenSession: boolean;
  revisionCount: number;
  timeline: FrameTimelineItem[];
}

export interface EditSession {
  id: string;
  userId: string;
  characterId: string;
  actionStreamId: string;
  processingTaskId: string;
  actionKey: string;
  baseRevisionId: string;
  baseActionContentHash: string;
  baseBindingRevision: number;
  sessionVersion: number;
  status: string;
  cursor: number;
  lastOperationSeq: number;
  draftSnapshotId: string;
  draftSnapshotHash: string;
  expiresAt: string;
  createdAt: string;
}

export interface ApplyOperationRequest {
  baseSessionVersion: number;
  idempotencyKey: string;
  operation: {
    type: string;
    schemaVersion: number;
    payload: unknown;
  };
}

export interface ApplyOperationResponse {
  sessionVersion: number;
  sequence: number;
  status: string;
}


export interface CommitSessionRequest {
  expectedSessionVersion: number;
  changeSummary?: string;
  activationPolicy: "immediate" | "manual" | "keep_current";
  idempotencyKey: string;
}

export interface CommitSessionResponse {
  revisionId: string;
  qualityJobId?: string;
  status: string;
}

export async function listRevisions(
  processingTaskId: string | number,
  actionKey: string,
): Promise<RevisionSummary[]> {
  const res = await apiClient.get(
    `${BASE}/processing-tasks/${processingTaskId}/actions/${actionKey}/revisions`,
  );
  return res.data;
}

export async function getRevision(
  revisionId: string,
): Promise<RevisionDetail> {
  const res = await apiClient.get(`${BASE}/revisions/${revisionId}`);
  return res.data;
}

export async function getActiveRevision(
  processingTaskId: string | number,
  actionKey: string,
): Promise<RevisionDetail> {
  const res = await apiClient.get(
    `${BASE}/processing-tasks/${processingTaskId}/actions/${actionKey}/active-revision`,
  );
  return res.data;
}

export async function activateRevision(
  processingTaskId: string | number,
  actionKey: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/processing-tasks/${processingTaskId}/actions/${actionKey}/active-revision`,
    data,
  );
  return res.data;
}

export async function getEditSummary(
  processingTaskId: string | number,
  actionKey: string,
): Promise<ActionEditSummary> {
  const res = await apiClient.get(
    `${BASE}/processing-tasks/${processingTaskId}/actions/${actionKey}/edit-summary`,
  );
  return res.data;
}

export async function getPreviewManifest(
  revisionId: string,
): Promise<any> {
  const res = await apiClient.get(
    `${BASE}/revisions/${revisionId}/preview-manifest`,
  );
  return res.data;
}

export function getFrameImageUrl(
  revisionId: string,
  frameId: string,
): string {
  return `${BASE}/revisions/${revisionId}/frames/${frameId}/image`;
}

export function getFrameThumbnailUrl(
  revisionId: string,
  frameId: string,
): string {
  return `${BASE}/revisions/${revisionId}/frames/${frameId}/thumbnail`;
}

export async function triggerQualityEvaluation(
  revisionId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/revisions/${revisionId}/quality-evaluations`,
  );
  return res.data;
}

export async function getLatestQualityEvaluation(
  revisionId: string,
): Promise<any> {
  const res = await apiClient.get(
    `${BASE}/revisions/${revisionId}/quality-evaluations/latest`,
  );
  return res.data;
}

export async function createSession(
  processingTaskId: string | number,
  actionKey: string,
  data: any,
): Promise<EditSession> {
  const res = await apiClient.post(
    `${BASE}/processing-tasks/${processingTaskId}/actions/${actionKey}/edit-sessions`,
    data,
  );
  return res.data;
}

export async function getSession(
  sessionId: string,
): Promise<EditSession> {
  const res = await apiClient.get(`${BASE}/edit-sessions/${sessionId}`);
  return res.data;
}

export async function applyOperation(
  sessionId: string,
  data: ApplyOperationRequest,
): Promise<ApplyOperationResponse> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/operations`,
    data,
  );
  return res.data;
}

export async function undo(
  sessionId: string,
  baseVersion: number,
): Promise<ApplyOperationResponse> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/undo`,
    null,
    { params: { baseSessionVersion: baseVersion } },
  );
  return res.data;
}

export async function redo(
  sessionId: string,
  baseVersion: number,
): Promise<ApplyOperationResponse> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/redo`,
    null,
    { params: { baseSessionVersion: baseVersion } },
  );
  return res.data;
}

export async function createCheckpoint(
  sessionId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/checkpoints`,
  );
  return res.data;
}

export async function commitSession(
  sessionId: string,
  data: CommitSessionRequest,
): Promise<CommitSessionResponse> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/commit`,
    data,
  );
  return res.data;
}

export async function abandonSession(
  sessionId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/abandon`,
  );
  return res.data;
}

export async function getSessionEvents(
  sessionId: string,
): Promise<any> {
  const res = await apiClient.get(
    `${BASE}/edit-sessions/${sessionId}/events`,
  );
  return res.data;
}

export async function createRegenerationJob(
  sessionId: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/regeneration-jobs`,
    data,
  );
  return res.data;
}

export async function getRegenerationJob(
  sessionId: string,
  jobId: string,
): Promise<any> {
  const res = await apiClient.get(
    `${BASE}/edit-sessions/${sessionId}/regeneration-jobs/${jobId}`,
  );
  return res.data;
}

export async function cancelRegenerationJob(
  sessionId: string,
  jobId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/regeneration-jobs/${jobId}/cancel`,
  );
  return res.data;
}

export async function acceptCandidate(
  sessionId: string,
  candidateId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/candidates/${candidateId}/accept`,
    { idempotencyKey: `candidate:accept:${candidateId}` },
  );
  return res.data;
}

export async function rejectCandidate(
  sessionId: string,
  candidateId: string,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/candidates/${candidateId}/reject`,
    { idempotencyKey: `candidate:reject:${candidateId}` },
  );
  return res.data;
}

export async function uploadCandidate(
  sessionId: string,
  file: File,
  targetFrameId: string,
): Promise<any> {
  const fd = new FormData();
  fd.append("file", file);
  fd.append("targetFrameId", targetFrameId);
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/upload-candidates`,
    fd,
  );
  return res.data;
}

export async function applyBackgroundPatch(
  sessionId: string,
  frameId: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/frames/${frameId}/background-patches`,
    data,
  );
  return res.data;
}

export async function resetBackgroundPatch(
  sessionId: string,
  frameId: string,
): Promise<any> {
  const res = await apiClient.delete(
    `${BASE}/edit-sessions/${sessionId}/frames/${frameId}/background-patches`,
  );
  return res.data;
}

export async function setFrameAnchor(
  sessionId: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/frames/${data.frameId}/anchor`,
    data,
  );
  return res.data;
}

export async function batchOffsetAnchors(
  sessionId: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/anchors/batch-offset`,
    data,
  );
  return res.data;
}

export async function resetAnchors(
  sessionId: string,
  data: any,
): Promise<any> {
  const res = await apiClient.post(
    `${BASE}/edit-sessions/${sessionId}/anchors/reset`,
    data,
  );
  return res.data;
}
