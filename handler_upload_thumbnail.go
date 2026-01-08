package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const maxMemory = 10 << 20

func determineFileExtension(contentType string) string {
	parts := strings.Split(contentType, "/")
	if len(parts) < 2 {
		return "bin"
	}
	return parts[1]
}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
		return
	}
	thumbnailFile, h, err := r.FormFile("thumbnail")
	defer thumbnailFile.Close()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
		return
	}
	mediaType := h.Header.Get("Content-Type")
	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error(), err)
		return
	}
	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	dstFilePath := fmt.Sprintf("%s.%s", filepath.Join(cfg.assetsRoot, videoIDString), determineFileExtension(mediaType))
	dstFile, err := os.Create(dstFilePath)
	defer func(f *os.File) {
		closeErr := f.Close()
		if closeErr != nil {
			respondWithError(w, http.StatusInternalServerError, closeErr.Error(), closeErr)
			return
		}
	}(dstFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
		return
	}
	_, err = io.Copy(dstFile, thumbnailFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
		return
	}
	thumbnailUrl := fmt.Sprintf("http://localhost:%s/%s", cfg.port, dstFilePath)
	dbVideo.ThumbnailURL = &thumbnailUrl
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
	}
	respondWithJSON(w, http.StatusOK, dbVideo)
}
