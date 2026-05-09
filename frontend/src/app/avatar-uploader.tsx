import { Camera, ImagePlus, Loader2, Move } from "lucide-react";
import { useEffect, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { avatarOutputSize, cropViewportSize, type CropOffset, type ImageSize } from "./types";

export function AvatarUploader({ busy, onUpload }: { busy: boolean; onUpload: (avatar: Blob) => Promise<void> }) {
  const [imageSrc, setImageSrc] = useState("");
  const [imageSize, setImageSize] = useState<ImageSize | null>(null);
  const [zoom, setZoom] = useState(1);
  const [offset, setOffset] = useState<CropOffset>({ x: 0, y: 0 });
  const [dragStart, setDragStart] = useState<{ pointerX: number; pointerY: number; offset: CropOffset } | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    return () => {
      if (imageSrc) URL.revokeObjectURL(imageSrc);
    };
  }, [imageSrc]);

  const reset = () => {
    if (imageSrc) URL.revokeObjectURL(imageSrc);
    setImageSrc("");
    setImageSize(null);
    setZoom(1);
    setOffset({ x: 0, y: 0 });
    setDragStart(null);
    if (inputRef.current) inputRef.current.value = "";
  };

  const handleFile = (file?: File) => {
    if (!file || !file.type.startsWith("image/")) return;
    if (imageSrc) URL.revokeObjectURL(imageSrc);
    setImageSrc(URL.createObjectURL(file));
    setImageSize(null);
    setZoom(1);
    setOffset({ x: 0, y: 0 });
  };

  const updateZoom = (value: number) => {
    setZoom(value);
    setOffset((current) => (imageSize ? clampCropOffset(current, imageSize, value) : current));
  };

  const beginDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!imageSize) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragStart({
      pointerX: event.clientX,
      pointerY: event.clientY,
      offset
    });
  };

  const moveDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!dragStart || !imageSize) return;
    setOffset(
      clampCropOffset(
        {
          x: dragStart.offset.x + event.clientX - dragStart.pointerX,
          y: dragStart.offset.y + event.clientY - dragStart.pointerY
        },
        imageSize,
        zoom
      )
    );
  };

  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (dragStart) event.currentTarget.releasePointerCapture(event.pointerId);
    setDragStart(null);
  };

  const upload = async () => {
    if (!imageSrc || !imageSize) return;
    const avatar = await createCircularAvatarBlob(imageSrc, imageSize, zoom, offset);
    await onUpload(avatar);
    reset();
  };

  const imageStyle = imageSize ? cropImageStyle(imageSize, zoom, offset) : undefined;

  return (
    <div className="avatar-uploader">
      <input
        ref={inputRef}
        accept="image/png,image/jpeg,image/webp,image/gif"
        className="visually-hidden"
        type="file"
        onChange={(event) => handleFile(event.target.files?.[0])}
      />
      <button className="avatar-upload-button" disabled={busy} type="button" onClick={() => inputRef.current?.click()}>
        <Camera size={16} />
        选择头像
      </button>

      {imageSrc && (
        <div className="avatar-cropper">
          <div
            className="crop-stage"
            onPointerDown={beginDrag}
            onPointerMove={moveDrag}
            onPointerUp={endDrag}
            onPointerCancel={endDrag}
          >
            <img
              alt=""
              className="crop-image"
              draggable={false}
              src={imageSrc}
              style={imageStyle}
              onLoad={(event) => {
                const nextSize = {
                  width: event.currentTarget.naturalWidth,
                  height: event.currentTarget.naturalHeight
                };
                setImageSize(nextSize);
                setOffset(clampCropOffset({ x: 0, y: 0 }, nextSize, zoom));
              }}
            />
            <div className="crop-mask" />
            <span className="crop-move-icon">
              <Move size={17} />
            </span>
          </div>
          <label className="crop-zoom">
            <span>缩放</span>
            <input min="1" max="3" step="0.05" type="range" value={zoom} onChange={(event) => updateZoom(Number(event.target.value))} />
          </label>
          <div className="crop-actions">
            <button disabled={busy} type="button" onClick={reset}>
              取消
            </button>
            <button disabled={busy || !imageSize} type="button" onClick={() => void upload()}>
              {busy ? <Loader2 className="spin" size={16} /> : <ImagePlus size={16} />}
              上传
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function cropImageStyle(imageSize: ImageSize, zoom: number, offset: CropOffset): CSSProperties {
  const metrics = cropMetrics(imageSize, zoom);
  return {
    width: imageSize.width * metrics.scale,
    height: imageSize.height * metrics.scale,
    transform: `translate(-50%, -50%) translate(${offset.x}px, ${offset.y}px)`
  };
}

function cropMetrics(imageSize: ImageSize, zoom: number) {
  const baseScale = Math.max(cropViewportSize / imageSize.width, cropViewportSize / imageSize.height);
  const scale = baseScale * zoom;
  return {
    baseScale,
    scale,
    maxX: Math.max(0, (imageSize.width * scale - cropViewportSize) / 2),
    maxY: Math.max(0, (imageSize.height * scale - cropViewportSize) / 2)
  };
}

function clampCropOffset(offset: CropOffset, imageSize: ImageSize, zoom: number): CropOffset {
  const metrics = cropMetrics(imageSize, zoom);
  return {
    x: clamp(offset.x, -metrics.maxX, metrics.maxX),
    y: clamp(offset.y, -metrics.maxY, metrics.maxY)
  };
}

async function createCircularAvatarBlob(src: string, imageSize: ImageSize, zoom: number, offset: CropOffset) {
  const image = await loadImage(src);
  const metrics = cropMetrics(imageSize, zoom);
  const sourceSize = cropViewportSize / metrics.scale;
  const sourceX = (0 - cropViewportSize / 2 - offset.x) / metrics.scale + imageSize.width / 2;
  const sourceY = (0 - cropViewportSize / 2 - offset.y) / metrics.scale + imageSize.height / 2;
  const canvas = document.createElement("canvas");
  canvas.width = avatarOutputSize;
  canvas.height = avatarOutputSize;
  const context = canvas.getContext("2d");
  if (!context) throw new Error("canvas is not supported");
  context.clearRect(0, 0, avatarOutputSize, avatarOutputSize);
  context.save();
  context.beginPath();
  context.arc(avatarOutputSize / 2, avatarOutputSize / 2, avatarOutputSize / 2, 0, Math.PI * 2);
  context.clip();
  context.drawImage(image, sourceX, sourceY, sourceSize, sourceSize, 0, 0, avatarOutputSize, avatarOutputSize);
  context.restore();
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error("failed to crop avatar"));
    }, "image/png");
  });
}

function loadImage(src: string) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("failed to load image"));
    image.src = src;
  });
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}
