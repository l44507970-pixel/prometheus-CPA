package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/donation-station/donation-station/internal/cpa"
)

func TestAuthFileUnavailableMessage(t *testing.T) {
	tests := []struct {
		name      string
		file      cpa.AuthFile
		wantBlock bool
	}{
		{
			name:      "disabled credential is blocked",
			file:      cpa.AuthFile{Disabled: true},
			wantBlock: true,
		},
		{
			name:      "unavailable credential is blocked",
			file:      cpa.AuthFile{Unavailable: true, StatusMessage: "刷新失败"},
			wantBlock: true,
		},
		{
			name:      "missing project id message is blocked",
			file:      cpa.AuthFile{Status: "ok", StatusMessage: "Antigravity 凭证缺少 project_id。请重新登录或刷新凭证以发现项目。"},
			wantBlock: true,
		},
		{
			name:      "normal credential is allowed",
			file:      cpa.AuthFile{Status: "ok"},
			wantBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authFileUnavailableMessage(tt.file)
			if tt.wantBlock && strings.TrimSpace(got) == "" {
				t.Fatalf("expected credential to be blocked")
			}
			if !tt.wantBlock && got != "" {
				t.Fatalf("expected credential to be allowed, got %q", got)
			}
		})
	}
}

func TestFindAuthFileByEmail(t *testing.T) {
	files := []cpa.AuthFile{
		{Email: "user@example.com", Provider: "codex", ID: "codex-file"},
		{Email: "User@Example.com", Provider: "gemini-cli", ID: "gemini-file"},
	}

	got := findAuthFileByEmail(files, "user@example.com", "gemini_cli")
	if got == nil || got.ID != "gemini-file" {
		t.Fatalf("expected normalized provider match, got %#v", got)
	}

	got = findAuthFileByEmail(files, "user@example.com", "")
	if got == nil {
		t.Fatalf("expected email fallback match")
	}
}

func TestAuthFileValidationMessageBlocksAntigravityWithoutProjectID(t *testing.T) {
	got := (&Server{}).authFileValidationMessage(context.Background(), cpa.AuthFile{
		Provider: "antigravity",
	})
	if !strings.Contains(got, "project_id") {
		t.Fatalf("expected missing project_id to be blocked, got %q", got)
	}
}

func TestAntigravityProbeRequest(t *testing.T) {
	got := antigravityProbeRequest(" auth-index ")
	if got.AuthIndex != "auth-index" {
		t.Fatalf("AuthIndex = %q, want auth-index", got.AuthIndex)
	}
	if got.Method != http.MethodPost {
		t.Fatalf("Method = %q, want POST", got.Method)
	}
	if got.URL != antigravityProbeURL {
		t.Fatalf("URL = %q, want %q", got.URL, antigravityProbeURL)
	}
	if got.Header["Authorization"] != "Bearer $TOKEN$" {
		t.Fatalf("Authorization header = %q", got.Header["Authorization"])
	}
	if got.Data != antigravityProbeBody {
		t.Fatalf("Data = %q, want %q", got.Data, antigravityProbeBody)
	}
}

func TestAntigravityAPICallUnavailableMessage(t *testing.T) {
	tests := []struct {
		name      string
		file      cpa.AuthFile
		resp      *cpa.APICallResponse
		wantBlock bool
		wantText  string
	}{
		{
			name:      "missing project id is blocked",
			resp:      &cpa.APICallResponse{StatusCode: http.StatusOK, Body: `{"allowedTiers":[{"id":"free-tier"}]}`},
			wantBlock: true,
			wantText:  "project_id",
		},
		{
			name:      "project id from probe is allowed",
			resp:      &cpa.APICallResponse{StatusCode: http.StatusOK, Body: `{"cloudaicompanionProject":"project-1"}`},
			wantBlock: false,
		},
		{
			name:      "project id from auth file is allowed",
			file:      cpa.AuthFile{ProjectID: "project-1"},
			resp:      &cpa.APICallResponse{StatusCode: http.StatusOK, Body: `{"paidTier":{"availableCredits":[{"creditType":"GOOGLE_ONE_AI","creditAmount":"100","minimumCreditAmountForUsage":"50"}]}}`},
			wantBlock: false,
		},
		{
			name:      "upstream error is blocked",
			resp:      &cpa.APICallResponse{StatusCode: http.StatusUnauthorized, Body: `{"error":{"message":"invalid token"}}`},
			wantBlock: true,
			wantText:  "invalid token",
		},
		{
			name:      "insufficient credits are blocked",
			file:      cpa.AuthFile{ProjectID: "project-1"},
			resp:      &cpa.APICallResponse{StatusCode: http.StatusOK, Body: `{"paidTier":{"availableCredits":[{"creditType":"GOOGLE_ONE_AI","creditAmount":"1","minimumCreditAmountForUsage":"50"}]}}`},
			wantBlock: true,
			wantText:  "额度不足",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := antigravityAPICallUnavailableMessage(tt.file, tt.resp)
			if tt.wantBlock && strings.TrimSpace(got) == "" {
				t.Fatalf("expected credential to be blocked")
			}
			if !tt.wantBlock && got != "" {
				t.Fatalf("expected credential to be allowed, got %q", got)
			}
			if tt.wantText != "" && !strings.Contains(got, tt.wantText) {
				t.Fatalf("message = %q, want text %q", got, tt.wantText)
			}
		})
	}
}
