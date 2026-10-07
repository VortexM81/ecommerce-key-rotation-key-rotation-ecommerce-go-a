package rotation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var orderStages = []string{"checkout", "fulfillment", "receipts", "customer-order-updates"}

type KeyAPI interface {
	CreateKey(context.Context, string, string, string) (json.RawMessage, error)
	RotateKey(context.Context, string, int, string) (json.RawMessage, error)
	SearchLogs(context.Context) (json.RawMessage, error)
	RevokeKey(context.Context, string) (json.RawMessage, error)
}

type Request struct {
	ProjectID     string `json:"project_id"`
	GraceHours    int    `json:"grace_hours"`
	SecretDir     string `json:"secret_dir"`
	ExecuteRevoke bool   `json:"execute_revoke"`
}

type Result struct {
	KeyID            string   `json:"key_id"`
	CoveredStages    []string `json:"covered_stages"`
	StaleDeployments []string `json:"stale_deployments"`
	Decision         string   `json:"decision"`
	SecretFiles      []string `json:"secret_files"`
}

type keyRecord struct {
	ID string `json:"id"`
}

type logRecord struct {
	Deployment string `json:"deployment"`
	KeyID      string `json:"key_id"`
}

type logData struct {
	Logs []logRecord `json:"logs"`
}

func Run(ctx context.Context, api KeyAPI, req Request) (Result, error) {
	if req.GraceHours <= 0 || req.SecretDir == "" {
		return Result{}, fmt.Errorf("grace_hours must be positive and secret_dir is required")
	}
	createToken, err := token()
	if err != nil {
		return Result{}, err
	}
	created, err := api.CreateKey(ctx, req.ProjectID, "ecommerce-rotation-candidate", createToken)
	if err != nil {
		return Result{}, err
	}
	var record keyRecord
	if err := json.Unmarshal(created, &record); err != nil || record.ID == "" {
		return Result{}, fmt.Errorf("decode created key record")
	}
	createdFile, err := storeOnce(req.SecretDir, "created-key.json", created)
	if err != nil {
		return Result{}, err
	}

	rotateToken, err := token()
	if err != nil {
		return Result{}, err
	}
	rotated, err := api.RotateKey(ctx, record.ID, req.GraceHours, rotateToken)
	if err != nil {
		return Result{}, err
	}
	rotatedFile, err := storeOnce(req.SecretDir, "rotated-key.json", rotated)
	if err != nil {
		return Result{}, err
	}

	logs, err := api.SearchLogs(ctx)
	if err != nil {
		return Result{}, err
	}
	stale, err := deploymentsUsing(logs, record.ID)
	if err != nil {
		return Result{}, err
	}
	result := Result{KeyID: record.ID, CoveredStages: append([]string(nil), orderStages...), StaleDeployments: stale, Decision: "hold", SecretFiles: []string{createdFile, rotatedFile}}
	if len(stale) == 0 {
		result.Decision = "ready-to-revoke"
		if req.ExecuteRevoke {
			if _, err := api.RevokeKey(ctx, record.ID); err != nil {
				return Result{}, err
			}
			result.Decision = "revoked"
		}
	}
	return result, nil
}

func deploymentsUsing(raw json.RawMessage, keyID string) ([]string, error) {
	var data logData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode log records: %w", err)
	}
	seen := map[string]bool{}
	for _, entry := range data.Logs {
		if entry.KeyID == keyID && entry.Deployment != "" {
			seen[entry.Deployment] = true
		}
	}
	deployments := make([]string, 0, len(seen))
	for deployment := range seen {
		deployments = append(deployments, deployment)
	}
	sort.Strings(deployments)
	return deployments, nil
}

func storeOnce(dir, name string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create secret directory: %w", err)
	}
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("store one-time key response: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write one-time key response: %w", err)
	}
	return path, nil
}

func token() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("create idempotency key: %w", err)
	}
	return strings.ToLower(hex.EncodeToString(b)), nil
}
