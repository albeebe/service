// Copyright (c) 2024 Alan Beebe [www.alanbeebe.com]
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// Created: October 2, 2024

package auth

import (
	"net/http"
	"time"
)

type AuthProvider interface {
	// AuthorizeRequest checks if the request meets the specified authorization requirement.
	// It returns true if the request is authorized, otherwise false, and an error if something goes wrong.
	AuthorizeRequest(r *http.Request, permission string) (isAuthorized bool, err error)

	// IsServiceRequest checks whether the given HTTP request originates from a service.
	// It returns true if the request is identified as a service request, otherwise false.
	IsServiceRequest(r *http.Request) (isService bool)

	// RefreshAccessToken refreshes the current access token.
	// It returns the new access token, the time for the next refresh, and an error if the operation fails.
	RefreshAccessToken() (accessToken *AccessToken, nextRefresh time.Time, err error)

	// RefreshKeys retrieves updated authentication keys and the scheduled time for the next key refresh.
	// It returns a slice of keys, the time for the next refresh, and an error if the operation fails.
	RefreshKeys() (keys []*Key, nextRefresh time.Time, err error)
}
