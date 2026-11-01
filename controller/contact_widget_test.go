/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateContactWidgetConfig(t *testing.T) {
	validContact := `{"id":1,"name":"WeChat Support","icon":"wechat","contact_id":"newapi_service","link":""}`
	validConfig := `{"enabled":true,"title":"Contact Us","description":"Pick a channel","contacts":[` + validContact + `]}`

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty value is not configured", raw: "", wantErr: false},
		{name: "disabled config without title", raw: `{"enabled":false,"title":"","contacts":[]}`, wantErr: false},
		{name: "valid enabled config", raw: validConfig, wantErr: false},
		{name: "contact without id or link", raw: `{"enabled":true,"title":"Contact Us","contacts":[{"id":1,"name":"WeChat Support","icon":"wechat"}]}`, wantErr: true},
		{name: "contact with only a link", raw: `{"enabled":true,"title":"Contact Us","contacts":[{"id":1,"name":"Telegram","link":"https://example.com/chat"}]}`, wantErr: false},
		{name: "enabled without title", raw: `{"enabled":true,"title":"","contacts":[` + validContact + `]}`, wantErr: true},
		{name: "javascript link", raw: `{"enabled":true,"title":"Contact Us","contacts":[{"id":1,"name":"Bad","link":"javascript:alert(1)"}]}`, wantErr: true},
		{name: "link without host", raw: `{"enabled":true,"title":"Contact Us","contacts":[{"id":1,"name":"Bad","link":"https://"}]}`, wantErr: true},
		{name: "contact without name", raw: `{"enabled":true,"title":"Contact Us","contacts":[{"id":1,"name":"  ","contact_id":"x"}]}`, wantErr: true},
		{name: "invalid json", raw: `{"enabled":true,`, wantErr: true},
		{name: "title too long", raw: `{"enabled":true,"title":"` + strings.Repeat("x", 51) + `","contacts":[]}`, wantErr: true},
		{name: "too many contacts", raw: `{"enabled":true,"title":"Contact Us","contacts":[` + strings.Repeat(validContact+",", 20) + validContact + `]}`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContactWidgetConfig(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetContactWidgetServesConfiguredContent(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest("GET", "/api/contact_widget", nil)

	GetContactWidget(context)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"success":true`)
	assert.NotEmpty(t, response.Header().Get("ETag"))
	assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
}
