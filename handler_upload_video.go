package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const maxAllowedVideoSize = 1 << 30

func newVideoKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	keyStr := fmt.Sprintf("%s.mp4", base64.URLEncoding.EncodeToString(key))
	return keyStr, nil
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAllowedVideoSize)

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

	fmt.Println("uploading video", videoID, "by user", userID)

	dbVideo, err := cfg.db.GetVideo(videoID)

	if userID != dbVideo.UserID {
		respondWithError(w, http.StatusUnauthorized, "not allowed", nil)
		return
	}

	file, header, err := r.FormFile("video")
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "bad format, only mp4 files are allowed", nil)
	}
	defer file.Close()
	// videoFileExtension := determineFileExtension(mediaType)

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)

	tempFile.Seek(0, io.SeekStart)
	objName, err := newVideoKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
		return
	}
	putObjInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &objName,
		Body:        tempFile,
		ContentType: &mediaType,
	}
	cfg.s3Client.PutObject(r.Context(), &putObjInput)
	videoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, objName)
	dbVideo.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error(), err)
	}

}
