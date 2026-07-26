// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import {
  createAuthorizedRequestInit,
  resolveApiUrl,
} from "../runtime/runtime-adapter";

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
      } catch {
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
  const processingImage = ref(false);
  let imageSelectionVersion = 0;

  function fileToBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result as string);
      reader.onerror = () => reject(new Error("文件读取失败"));
      reader.readAsDataURL(file);
    });
  }

  async function handleImageSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      input.value = "";
      return;
    }

    const selectionVersion = ++imageSelectionVersion;
    attachedImage.value = file;
    attachedImagePreview.value = null;
    processingImage.value = true;
    onRemoveImage();
    input.value = "";

    try {
      const normalizedFile = isVisionImageTypeSupported(file.type)
        ? file
        : await convertImageToPng(file);
      const dataUrl = await fileToBase64(normalizedFile);
      if (selectionVersion !== imageSelectionVersion) return;
      attachedImage.value = normalizedFile;
      attachedImagePreview.value = dataUrl;
      onImage(normalizedFile, dataUrl);
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
    attachedImage.value = null;
    attachedImagePreview.value = null;
    processingImage.value = false;
    onRemoveImage();
  }

  function handleVideoSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("video/")) return;
    attachedVideo.value = file;
    uploadingVideo.value = true;
    const formData = new FormData();
    formData.append("video", file);
    Promise.all([
      resolveApiUrl("/api/video/upload"),
      createAuthorizedRequestInit({ method: "POST", body: formData }),
    ])
      .then(([url, init]) => fetch(url, init))
      .then((res) => res.json())
      .then((data) => {
        const videoUrl = data?.data?.videoUrl || data?.videoUrl || "";
        if (videoUrl) {
          attachedVideoUrl.value = videoUrl;
          onVideo(file, videoUrl);
        }
      })
      .catch(() => {})
      .finally(() => {
        uploadingVideo.value = false;
      });
    input.value = "";
  }

  function clearVideo() {
    attachedVideo.value = null;
    attachedVideoUrl.value = null;
    uploadingVideo.value = false;
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
    processingImage,
    handleImageSelect,
    clearImage,
    handleVideoSelect,
    clearVideo,
    fileToBase64,
  };
}
