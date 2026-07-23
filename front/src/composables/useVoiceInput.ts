// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { onUnmounted, ref } from "vue";

export function useVoiceInput(
  onVoiceAudio: (blob: Blob, duration?: number) => void,
  isDisabled: () => boolean,
  isSending: () => boolean,
  onError: () => void,
) {
  const holding = ref(false);
  const recording = ref(false);

  let mediaRecorder: MediaRecorder | null = null;
  let mediaStream: MediaStream | null = null;
  let audioChunks: Blob[] = [];
  let recordingStartTime = 0;
  let requestVersion = 0;
  let cancelled = false;

  function removeReleaseListener() {
    document.removeEventListener("pointerup", endHold);
  }

  function stopStream() {
    mediaStream?.getTracks().forEach((track) => track.stop());
    mediaStream = null;
  }

  function resetRecorder() {
    mediaRecorder = null;
    recording.value = false;
    stopStream();
  }

  async function startHold() {
    if (holding.value || recording.value || isDisabled() || isSending()) return;
    if (
      !navigator.mediaDevices?.getUserMedia ||
      typeof MediaRecorder === "undefined"
    ) {
      onError();
      return;
    }

    holding.value = true;
    cancelled = false;
    audioChunks = [];
    const version = ++requestVersion;
    document.addEventListener("pointerup", endHold);

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (version !== requestVersion || !holding.value) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }

      mediaStream = stream;
      const mimeType = MediaRecorder.isTypeSupported("audio/webm;codecs=opus")
        ? "audio/webm;codecs=opus"
        : "audio/webm";
      const recorder = new MediaRecorder(stream, { mimeType });
      mediaRecorder = recorder;
      recorder.ondataavailable = (event: BlobEvent) => {
        if (event.data.size > 0) audioChunks.push(event.data);
      };
      recorder.onstop = () => {
        const chunks = audioChunks;
        const shouldSend = !cancelled && chunks.length > 0;
        const duration = Math.max(
          1,
          Math.round((Date.now() - recordingStartTime) / 1000),
        );
        audioChunks = [];
        resetRecorder();
        if (shouldSend)
          onVoiceAudio(new Blob(chunks, { type: mimeType }), duration);
      };
      recorder.onerror = () => {
        cancelled = true;
        audioChunks = [];
        resetRecorder();
        holding.value = false;
        removeReleaseListener();
        onError();
      };
      recordingStartTime = Date.now();
      recorder.start();
      recording.value = true;
    } catch {
      if (version !== requestVersion) return;
      holding.value = false;
      cancelled = true;
      audioChunks = [];
      resetRecorder();
      removeReleaseListener();
      onError();
    }
  }

  function endHold() {
    if (!holding.value) return;
    holding.value = false;
    removeReleaseListener();
    if (!mediaRecorder) {
      requestVersion++;
      stopStream();
      return;
    }
    if (mediaRecorder.state !== "inactive") mediaRecorder.stop();
  }

  function cancelHold() {
    cancelled = true;
    endHold();
  }

  onUnmounted(() => {
    cancelled = true;
    holding.value = false;
    requestVersion++;
    removeReleaseListener();
    if (mediaRecorder && mediaRecorder.state !== "inactive")
      mediaRecorder.stop();
    resetRecorder();
  });

  return {
    holding,
    recording,
    startHold,
    endHold,
    cancelHold,
  };
}
