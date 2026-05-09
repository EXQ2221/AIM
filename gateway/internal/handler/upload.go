package handler

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model"
	"example.com/aim/gateway/internal/rpc"
	"example.com/aim/gateway/internal/upload"
	userpb "example.com/aim/gateway/kitex_gen/user"
	"github.com/gin-gonic/gin"
)

const multipartOverheadLimit = 1 << 20

var avatarExtByContentType = map[string]string{
	"image/gif":  ".gif",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func UploadAvatar(ctx *gin.Context) {
	authCtx, ok := middleware.GetAuthContext(ctx)
	if !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	maxBytes := upload.MaxBytes()
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes+multipartOverheadLimit)

	fileHeader, err := formFile(ctx, "file", "avatar")
	if err != nil {
		if isBodyTooLarge(err) {
			writeError(ctx, http.StatusRequestEntityTooLarge, "file is too large")
			return
		}
		writeError(ctx, 400, "file is required")
		return
	}
	if fileHeader.Size <= 0 {
		writeError(ctx, 400, "file is empty")
		return
	}
	if fileHeader.Size > maxBytes {
		writeError(ctx, http.StatusRequestEntityTooLarge, "file is too large")
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}
	defer src.Close()

	saved, err := saveAvatarFile(src, authCtx.UserID, maxBytes)
	if err != nil {
		status := 400
		if errors.Is(err, errFileTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(ctx, status, err.Error())
		return
	}

	client, err := rpc.UserClient()
	if err != nil {
		removeSavedFile(saved.diskPath)
		writeError(ctx, 500, err.Error())
		return
	}

	userResp, err := client.UpdateAvatar(ctx.Request.Context(), &userpb.UpdateAvatarRequest{
		UserId: authCtx.UserID,
		Avatar: saved.publicPath,
	})
	if err != nil {
		removeSavedFile(saved.diskPath)
		writeError(ctx, statusFromMessage(err.Error()), presentableMessage(err.Error()))
		return
	}
	if userResp == nil || userResp.User == nil {
		removeSavedFile(saved.diskPath)
		writeError(ctx, 500, "empty user response")
		return
	}

	writeJSON(ctx, 200, model.APIResponse{
		Code:    0,
		Message: "success",
		Data: model.UploadAvatarResponse{
			Avatar: saved.publicPath,
			File: model.UploadedFileInfo{
				URL:         saved.publicPath,
				Filename:    path.Base(saved.publicPath),
				ContentType: saved.contentType,
				Size:        saved.size,
			},
			User: model.UserInfo{
				UserID:       userResp.User.UserId,
				AimID:        userResp.User.AimId,
				Email:        userResp.User.Email,
				Nickname:     userResp.User.Nickname,
				Avatar:       userResp.User.Avatar,
				Status:       userResp.User.Status,
				Role:         userResp.User.Role,
				TokenVersion: userResp.User.TokenVersion,
				CreatedAt:    userResp.User.CreatedAt,
				UpdatedAt:    userResp.User.UpdatedAt,
			},
		},
	})
}

type savedAvatar struct {
	diskPath    string
	publicPath  string
	contentType string
	size        int64
}

var errFileTooLarge = errors.New("file is too large")

func formFile(ctx *gin.Context, names ...string) (*multipart.FileHeader, error) {
	var firstErr error
	for _, name := range names {
		fileHeader, err := ctx.FormFile(name)
		if err == nil {
			return fileHeader, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func saveAvatarFile(src io.Reader, userID int64, maxBytes int64) (savedAvatar, error) {
	header := make([]byte, 512)
	n, err := io.ReadFull(src, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return savedAvatar{}, err
	}
	if n == 0 {
		return savedAvatar{}, errors.New("file is empty")
	}

	contentType := http.DetectContentType(header[:n])
	ext, ok := avatarExtByContentType[contentType]
	if !ok {
		return savedAvatar{}, errors.New("unsupported avatar file type")
	}

	userIDText := strconv.FormatInt(userID, 10)
	relativeDir := filepath.Join("avatars", userIDText)
	targetDir := filepath.Join(upload.Dir(), relativeDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return savedAvatar{}, err
	}

	filename := newID() + ext
	diskPath := filepath.Join(targetDir, filename)
	dst, err := os.OpenFile(diskPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return savedAvatar{}, err
	}
	closed := false
	closeDst := func() {
		if !closed {
			_ = dst.Close()
			closed = true
		}
	}
	defer closeDst()

	written, err := dst.Write(header[:n])
	if err != nil {
		closeDst()
		removeSavedFile(diskPath)
		return savedAvatar{}, err
	}

	remaining := maxBytes - int64(written)
	if remaining < 0 {
		closeDst()
		removeSavedFile(diskPath)
		return savedAvatar{}, errFileTooLarge
	}

	copied, err := io.Copy(dst, io.LimitReader(src, remaining+1))
	if err != nil {
		closeDst()
		removeSavedFile(diskPath)
		return savedAvatar{}, err
	}
	total := int64(written) + copied
	if total > maxBytes {
		closeDst()
		removeSavedFile(diskPath)
		return savedAvatar{}, errFileTooLarge
	}

	publicPath := path.Join(upload.PublicPrefix(), "avatars", userIDText, filename)
	if len(publicPath) > 512 {
		closeDst()
		removeSavedFile(diskPath)
		return savedAvatar{}, errors.New("avatar path is too long")
	}

	return savedAvatar{
		diskPath:    diskPath,
		publicPath:  publicPath,
		contentType: contentType,
		size:        total,
	}, nil
}

func isBodyTooLarge(err error) bool {
	return err != nil && strings.Contains(err.Error(), "request body too large")
}

func removeSavedFile(filename string) {
	if filename != "" {
		_ = os.Remove(filename)
	}
}
