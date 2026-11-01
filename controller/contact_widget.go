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
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// ContactWidgetContact is one entry of the contact widget shown on the home
// page. Either ContactId (copied on click) or Link (opened on click) must be
// set so every entry is actionable.
type ContactWidgetContact struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	ContactId string `json:"contact_id"`
	Link      string `json:"link"`
}

// ContactWidgetConfig is the admin-configured content of the home page
// contact widget, stored as a JSON string under the ContactWidget option.
type ContactWidgetConfig struct {
	Enabled     bool                   `json:"enabled"`
	DefaultOpen bool                   `json:"default_open"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Contacts    []ContactWidgetContact `json:"contacts"`
}

const (
	contactWidgetTitleMaxLen       = 50
	contactWidgetDescriptionMaxLen = 200
	contactWidgetContactMaxCount   = 20
	contactWidgetNameMaxLen        = 30
	contactWidgetIconMaxLen        = 200
	contactWidgetContactIdMaxLen   = 128
	contactWidgetLinkMaxLen        = 500
)

// validateContactLink accepts only absolute http(s) URLs so a configured
// entry can never become a javascript: or data: navigation for visitors.
func validateContactLink(link string) error {
	parsed, err := url.Parse(link)
	if err != nil {
		return errors.New("invalid contact link")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("contact link must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("contact link must include a host")
	}
	return nil
}

// validateContactWidgetConfig checks the raw option value against the
// contact widget contract. An empty value is valid and means the widget is
// not configured.
func validateContactWidgetConfig(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var config ContactWidgetConfig
	if err := common.UnmarshalJsonStr(trimmed, &config); err != nil {
		return errors.New("contact widget config must be valid JSON")
	}
	if len([]rune(config.Title)) > contactWidgetTitleMaxLen {
		return errors.New("contact widget title is too long")
	}
	if len([]rune(config.Description)) > contactWidgetDescriptionMaxLen {
		return errors.New("contact widget description is too long")
	}
	if len(config.Contacts) > contactWidgetContactMaxCount {
		return errors.New("too many contacts in the contact widget")
	}
	for _, contact := range config.Contacts {
		if strings.TrimSpace(contact.Name) == "" {
			return errors.New("contact name is required")
		}
		if len([]rune(contact.Name)) > contactWidgetNameMaxLen {
			return errors.New("contact name is too long")
		}
		if len([]rune(contact.Icon)) > contactWidgetIconMaxLen {
			return errors.New("contact icon is too long")
		}
		if len([]rune(contact.ContactId)) > contactWidgetContactIdMaxLen {
			return errors.New("contact ID is too long")
		}
		if len([]rune(contact.Link)) > contactWidgetLinkMaxLen {
			return errors.New("contact link is too long")
		}
		if contact.Link != "" {
			if err := validateContactLink(contact.Link); err != nil {
				return err
			}
		}
		if strings.TrimSpace(contact.ContactId) == "" && strings.TrimSpace(contact.Link) == "" {
			return errors.New("each contact needs a contact ID or a link")
		}
	}
	if config.Enabled && strings.TrimSpace(config.Title) == "" {
		return errors.New("contact widget title is required when the widget is enabled")
	}
	return nil
}

func GetContactWidget(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	contactWidget := common.OptionMap["ContactWidget"]
	common.OptionMapRWMutex.RUnlock()
	serveRevalidatedJSON(c, contactWidget)
}
