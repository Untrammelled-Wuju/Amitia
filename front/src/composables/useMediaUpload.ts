// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, onUnmounted } from "vue";

const VISION_IMAGE_TYPES = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "image/bmp",
]);

export function isVisionImageTypeSupported(type: string) {
  return VISION_IMAGE_TYPES.has(type.toLowerCase());
}

function pngFileName(name: string) {
  const baseName = name.replace(/\.[^.]+$/, "") || "image";
  return `${baseName}.png`;
}

export function convertImageToPng(file: File): Promise<File> {
  return new Promise((resolve, reject) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();
    const cleanup = () => URL.revokeObjectURL(objectUrl);
    const fail = (error: Error) => {
      cleanup();
      reject(error);
    };

    image.onload = () => {
      const width = image.naturalWidth || image.width;
      const height = image.naturalHeight || image.height;
      if (!width || !height) {
        fail(new Error("图片尺寸无效，无法转换为 PNG"));
        return;
      }

      try {
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d");
        if (!context) {
          fail(new Error("当前环境无法转换图片格式"));
          return;
        }

        context.drawImage(image, 0, 0, width, height);
        canvas.toBlob((blob) => {
          cleanup();
          if (!blob) {
            reject(new Error("图片转换为 PNG 失败"));
            return;
          }
          resolve(
            new File([blob], pngFileName(file.name), {
              type: "image/png",
              lastModified: file.lastModified,
            }),
          );
        }, "image/png");
      } catch (error) {
  console.error("[original-error]", error);
fail(new Error("图片转换为 PNG 失败"));
      }
    };

    image.onerror = () => {
      fail(new Error("无法解析该图片格式，转换为 PNG 失败"));
    };
    image.src = objectUrl;
  });
}

export function useMediaUpload(
  onImage: (file: File, base64: string) => void,
  onVideo: (file: File, videoUrl: string) => void,
  onRemoveImage: () => void,
  onRemoveVideo: () => void,
) {
  const attachedImage = ref<File | null>(null);
  const attachedImagePreview = ref<string | null>(null);
  const fileInputRef = ref<HTMLInputElement>();
  const videoInputRef = ref<HTMLInputElement>();
  const attachedVideo = ref<File | null>(null);
  const attachedVideoUrl = ref<string | null>(null);
  const uploadingVideo = ref(false);
  let videoGeneration = 0;
  window.addEventListener("amitia:execution-scope-changed", clearVideo);
  onUnmounted(() => { window.removeEventListener("amitia:execution-scope-changed", clearVideo); videoGeneration++; });
  const videoUploadError = ref<string | null>(null);
  const processingImage = ref(false);
  let imageSelectionVersion = 0;

  async function handleImageSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      input.value = "";
      return;
    }

    const selectionVersion = ++imageSelectionVersion;
    const previousPreview = attachedImagePreview.value;
    if (previousPreview?.startsWith("blob:")) URL.revokeObjectURL(previousPreview);
    attachedImage.value = file;
    attachedImagePreview.value = null;
    processingImage.value = true;
    onRemoveImage();
    input.value = "";

    try {
      const normalizedFile = isVisionImageTypeSupported(file.type)
        ? file
        : await convertImageToPng(file);
      if (selectionVersion !== imageSelectionVersion) return;
      const previewUrl = URL.createObjectURL(normalizedFile);
      attachedImage.value = normalizedFile;
      attachedImagePreview.value = previewUrl;
      onImage(normalizedFile, previewUrl);
    } catch (error) {
      if (selectionVersion !== imageSelectionVersion) return;
      attachedImage.value = null;
      attachedImagePreview.value = null;
      onRemoveImage();
      throw error;
    } finally {
      if (selectionVersion === imageSelectionVersion) {
        processingImage.value = false;
      }
    }
  }

  function clearImage() {
    imageSelectionVersion += 1;
    if (attachedImagePreview.value?.startsWith("blob:")) URL.revokeObjectURL(attachedImagePreview.value);
    attachedImage.value = null;
    attachedImagePreview.value = null;
    processingImage.value = false;
    onRemoveImage();
  }

  function handleVideoSelect(e: Event) {
    ++videoGeneration;
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("video/")) return;
    attachedVideo.value = file;
    attachedVideoUrl.value = null;
    videoUploadError.value = null;
    uploadingVideo.value = true;
    onRemoveVideo();
    attachedVideoUrl.value = URL.createObjectURL(file);
    uploadingVideo.value = false;
    videoUploadError.value = null;
    onVideo(file, attachedVideoUrl.value);
    input.value = "";
  }

  function clearVideo() {
    videoGeneration++;
    if (attachedVideoUrl.value?.startsWith("blob:")) URL.revokeObjectURL(attachedVideoUrl.value);
    attachedVideo.value = null;
    attachedVideoUrl.value = null;
    uploadingVideo.value = false;
    videoUploadError.value = null;
    onRemoveVideo();
  }

  return {
    attachedImage,
    attachedImagePreview,
    fileInputRef,
    videoInputRef,
    attachedVideo,
    attachedVideoUrl,
    uploadingVideo,
    videoUploadError,
    processingImage,
    handleImageSelect,
    clearImage,
    handleVideoSelect,
    clearVideo,
  };
}
