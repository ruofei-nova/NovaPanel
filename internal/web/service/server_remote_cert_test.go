package service

import (
	"strings"
	"testing"
)

func TestGetPublicRemoteCertHashBlocksPrivateAddress(t *testing.T) {
	_, err := (&ServerService{}).GetPublicRemoteCertHash("127.0.0.1:443")
	if err == nil {
		t.Fatal("delegated certificate probe accepted a loopback target")
	}
	if !strings.Contains(err.Error(), "blocked private/internal address") {
		t.Fatalf("unexpected private-target error: %v", err)
	}
}

func TestGetCertHashRejectsOversizedInlineContent(t *testing.T) {
	_, err := (&ServerService{}).GetCertHash("", strings.Repeat("x", (1<<20)+1))
	if err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized certificate content was not rejected: %v", err)
	}
}
