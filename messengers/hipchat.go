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
	"net/http"
	"net/url"
	"strings"
)

// HipChat reports pipeline results using the legacy V1 API.
type HipChat struct {
	BaseMessenger `config:"suppress"`
	RoomID        string `config:"room_id"`
	Token         string `config:"token"`
	From          string `config:"from"`
}

// Post sends a notification without following redirects or logging its contents.
func (hc *HipChat) Post(message string, color ...string) bool {
	endpoint := "https://api.hipchat.com/v1/rooms/message?" + url.Values{"auth_token": {hc.Token}, "format": {"json"}}.Encode()
	body := url.Values{"room_id": {hc.RoomID}, "from": {hc.From}, "message": {message}, "color": {"purple"}, "message_format": {"text"}, "notify": {"1"}}
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return postNotification(req)
}
