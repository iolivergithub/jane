//go:build !linux && !windows

package uefi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestEfivarsNotSupported(t *testing.T) {
	for _, h := range []echo.HandlerFunc{Efivars, BootConfig} {
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")), rec)
		if err := h(c); err != nil || rec.Code != http.StatusNotImplemented || !strings.Contains(rec.Body.String(), "not supported") {
			t.Errorf("got %d %q, %v", rec.Code, rec.Body.String(), err)
		}
	}
}
