// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package readyz

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type HTTPResponseBodyChecker func(body []byte) error

type HTTPReadyZ struct {
	uri            string
	expectHTTPCode int
	bodyChecker    HTTPResponseBodyChecker
}

func NewHTTPReadyZ(uri string, code int, body HTTPResponseBodyChecker) ReadyZ {
	return &HTTPReadyZ{
		uri:            uri,
		expectHTTPCode: code,
		bodyChecker:    body,
	}
}

func (hr *HTTPReadyZ) IsReady(ctx context.Context) error {
	cl := http.Client{}
	req, err := http.NewRequest(http.MethodGet, hr.uri, nil)
	if err != nil {
		return fmt.Errorf("NewRequest error:%s", err.Error())
	}

	resp, err := cl.Do(req)
	if err != nil {
		nlog.Warnf("Get ready err:%s", err.Error())
		return err
	}
	if resp.StatusCode != hr.expectHTTPCode {
		return fmt.Errorf("uri(%s) expect http code is %d, but got %d", hr.uri, hr.expectHTTPCode, resp.StatusCode)
	}

	if hr.bodyChecker != nil {
		if resp.Body == nil {
			return fmt.Errorf("resp must has body, but got nil")
		}
		defer resp.Body.Close()
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("readAll fail with error: %s", err.Error())
		}

		return hr.bodyChecker(respBytes)
	}

	return nil
}
