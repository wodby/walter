/* walter: a deployment pipeline template
 * Copyright (C) 2014 Recruit Technologies Co., Ltd. and contributors
 * (see CONTRIBUTORS.md)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package messengers provides all functionality for the suported messengers
package messengers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// HipChat2 reports pipeline results using the V2 API.
type HipChat2 struct {
	BaseMessenger `config:"suppress"`
	RoomID        string `config:"room_id"`
	Token         string `config:"token"`
	From          string `config:"from"`
	BaseURL       string `config:"base_url"`
}

// Post sends a notification to the configured server with bounded HTTP handling.
func (hc *HipChat2) Post(message string, color ...string) bool {
	base := hc.BaseURL
	if base == "" {
		base = "https://api.hipchat.com/v2"
	}
	payload, err := json.Marshal(map[string]interface{}{"color": "purple", "message": message, "notify": true, "message_format": "text"})
	if err != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/room/"+url.PathEscape(hc.RoomID)+"/notification", bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+hc.Token)
	req.Header.Set("Content-Type", "application/json")
	return postNotification(req)
}
