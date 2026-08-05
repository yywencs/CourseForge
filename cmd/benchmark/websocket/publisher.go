package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const maxPublishResponseBytes = 1 << 20

type publishRequest struct {
	ClientMessageID string `json:"client_msg_id"`
	VideoTimeMS     uint64 `json:"video_time_ms"`
	Content         string `json:"content"`
}

type publishResponse struct {
	Code int `json:"code"`
}

func runPublisher(
	ctx context.Context,
	workerIndex int,
	cfg benchmarkConfig,
	token string,
	state *runState,
	metrics *benchmarkMetrics,
	sequence *atomic.Uint64,
	client *http.Client,
) {
	ticker := time.NewTicker(cfg.PublishEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case sentAt := <-ticker.C:
			seq := sequence.Add(1)
			content := fmt.Sprintf("cfbench:%d:%d", sentAt.UnixNano(), seq)
			body, _ := json.Marshal(publishRequest{
				ClientMessageID: uuid.NewString(), VideoTimeMS: seq % 600_000, Content: content,
			})
			request, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.publishURL(workerIndex), bytes.NewReader(body))
			if err == nil {
				request.Header.Set("Authorization", "Bearer "+token)
				request.Header.Set("Content-Type", "application/json")
				err = publish(client, request)
			}
			if !state.shouldRecord(sentAt) {
				continue
			}
			if err != nil {
				metrics.publishFailed.Add(1)
				continue
			}
			metrics.publishSucceeded.Add(1)
			// 发布期间连接数稳定时，该值可用于估算消息遗漏；连接抖动需结合 connected 一起判断。
			metrics.expected.Add(metrics.connected.Load())
		}
	}
}

func publish(client *http.Client, request *http.Request) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPublishResponseBytes+1))
	if err != nil || len(body) > maxPublishResponseBytes {
		return fmt.Errorf("读取发布响应失败")
	}
	var envelope publishResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || envelope.Code != 0 {
		return fmt.Errorf("发布失败: HTTP %d, code %d", response.StatusCode, envelope.Code)
	}
	return nil
}

func studentToken(cfg benchmarkConfig, studentID uint64, issuedAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"student_id": strconv.FormatUint(studentID, 10), "sub": strconv.FormatUint(studentID, 10),
		"actor_type": "student", "iss": cfg.JWTIssuer, "aud": cfg.JWTAudience,
		"iat": issuedAt.Unix(), "exp": issuedAt.Add(cfg.JWTTokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSigningKey))
}
