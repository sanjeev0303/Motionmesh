package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// To run this test, you need the api and worker services running locally.
// export TEST_API_URL="http://localhost:8080"
// export TEST_API_KEY="mot_live_5745bc3a6016ce7c.f94d892157d2ad2201d8865bbddc01e6244b3e0d46fbfb8a188ae27d8423005f"
// go test -v -tags=integration ./tests/integration

type UploadInitResponse struct {
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
	Video     struct {
		ID string `json:"id"`
	} `json:"video"`
}

type VideoResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ErrorMsg  string `json:"error_msg"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	SpriteKey string `json:"sprite_key"`
}

func TestE2EVideoUploadProcess(t *testing.T) {
	apiUrl := os.Getenv("TEST_API_URL")
	if apiUrl == "" {
		apiUrl = "http://localhost:8080"
	}

	apiKey := os.Getenv("TEST_API_KEY")
	if apiKey == "" {
		t.Skip("TEST_API_KEY environment variable is required for integration tests")
	}

	// 1. Download or create a mock video file
	testFileName := "e2e_test_video.mp4"
	err := createMockVideo(testFileName)
	if err != nil {
		t.Fatalf("failed to prepare mock video: %v", err)
	}
	defer os.Remove(testFileName)

	fileInfo, err := os.Stat(testFileName)
	if err != nil {
		t.Fatalf("failed to stat mock video: %v", err)
	}

	// 2. Initiate Upload
	initReqBody := []byte(fmt.Sprintf(`{"filename": "%s", "size_bytes": %d}`, testFileName, fileInfo.Size()))
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/v1/videos", apiUrl), bytes.NewBuffer(initReqBody))
	if err != nil {
		t.Fatalf("failed to create init request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to execute init request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201, got %d. Body: %s", resp.StatusCode, body)
	}

	var initResp UploadInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		t.Fatalf("failed to decode init response: %v", err)
	}

	t.Logf("Upload initialized. Video ID: %s", initResp.Video.ID)

	// 3. Upload Video
	file, err := os.Open(testFileName)
	if err != nil {
		t.Fatalf("failed to open video file: %v", err)
	}
	defer file.Close()

	uploadReq, err := http.NewRequest(http.MethodPut, initResp.UploadURL, file)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}
	uploadReq.Header.Set("Content-Type", "video/mp4")
	uploadReq.ContentLength = fileInfo.Size()

	// S3/B2 uploads might take slightly longer depending on connection
	uploadClient := &http.Client{Timeout: 60 * time.Second}
	uploadResp, err := uploadClient.Do(uploadReq)
	if err != nil {
		t.Fatalf("failed to execute upload request: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("expected status 200 for PUT upload, got %d. Body: %s", uploadResp.StatusCode, string(body))
	}

	t.Log("Video uploaded successfully to storage.")

	// 4. Poll for Processing Completion
	t.Log("Waiting for transcode worker to process the video...")
	
	maxRetries := 60
	delay := 3 * time.Second

	var finalStatus string
	for i := 0; i < maxRetries; i++ {
		time.Sleep(delay)

		statusReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/videos", apiUrl), nil)
		statusReq.Header.Set("Authorization", "Bearer "+apiKey)
		
		statusResp, err := client.Do(statusReq)
		if err != nil {
			t.Logf("warning: failed to poll status: %v", err)
			continue
		}
		
		var videos []VideoResponse
		if err := json.NewDecoder(statusResp.Body).Decode(&videos); err != nil {
			statusResp.Body.Close()
			t.Logf("warning: failed to decode status response: %v", err)
			continue
		}
		statusResp.Body.Close()

		var targetVideo *VideoResponse
		for _, v := range videos {
			if v.ID == initResp.Video.ID {
				targetVideo = &v
				break
			}
		}

		if targetVideo == nil {
			t.Log("video not found in list yet, waiting...")
			continue
		}

		t.Logf("Current Status: %s", targetVideo.Status)
		
		if targetVideo.Status == "ready" {
			finalStatus = targetVideo.Status
			if targetVideo.SpriteKey == "" {
				t.Errorf("expected sprite_key to be populated, got empty")
			}
			break
		} else if targetVideo.Status == "failed" {
			t.Fatalf("video processing failed: %s", targetVideo.ErrorMsg)
		}
	}

	if finalStatus != "ready" {
		t.Fatalf("video did not reach 'ready' state within timeout. Final status was %s", finalStatus)
	}

	t.Log("E2E Test Passed Successfully!")
}

// createMockVideo creates a minimal valid file or downloads a small one for testing
func createMockVideo(path string) error {
	// For simplicity, download a tiny sample MP4 like test_e2e.sh did
	resp, err := http.Get("https://www.w3schools.com/html/mov_bbb.mp4")
	if err != nil {
		return fmt.Errorf("failed to download sample mp4: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
