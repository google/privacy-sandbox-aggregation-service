// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package utils contains utility functions for services
package utils

import (
	"context"
	"fmt"
	"strings"

	log "github.com/golang/glog"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/idtoken"
)

// GetAuthorizationToken gets GCP service auth token based env service account or impersonated service account through default credentials
func GetAuthorizationToken(ctx context.Context, audience, impersonatedSvcAccount string) (string, error) {
	// TODO Switch to this implementation once google api upgraded to v0.52.0+
	// tokenSource, err = impersonate.IDTokenSource(ctx, impersonate.IDTokenConfig{
	// 	Audience:        audience,
	// 	TargetPrincipal: impersonatedSvcAccount,
	// 	IncludeEmail:    true,
	// })
	// if err != nil {
	// 	return nil, err
	// }
	token := ""
	// First we try the idtoken package, which only works for service accounts
	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		if !strings.Contains(err.Error(), `idtoken: credential must be service_account, found`) {
			return token, err
		}
		if impersonatedSvcAccount == "" {
			return token, fmt.Errorf("Couldn't obtain Auth Token, no svc account for impersonation set (flag 'impersonated_svc_account'): %v", err)
		}

		log.Info("no service account found, using application default credentials to impersonate service account")
		svc, err := iamcredentials.NewService(ctx)
		if err != nil {
			return token, err
		}
		resp, err := svc.Projects.ServiceAccounts.GenerateIdToken("projects/-/serviceAccounts/"+impersonatedSvcAccount, &iamcredentials.GenerateIdTokenRequest{
			Audience: audience,
		}).Do()
		if err != nil {
			return token, err
		}
		token = resp.Token

	} else {
		t, err := tokenSource.Token()
		if err != nil {
			return token, fmt.Errorf("TokenSource.Token: %v", err)
		}
		token = t.AccessToken
	}
	return token, nil
}
