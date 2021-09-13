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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/golang/glog"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/idtoken"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
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

// IsGCSObjectExist checks if a GCS object exists.
func IsGCSObjectExist(ctx context.Context, client *storage.Client, filename string) (bool, error) {
	bucket, object, err := ioutils.ParseGCSPath(filename)
	if err != nil {
		return false, err
	}
	_, err = client.Bucket(bucket).Object(object).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return false, nil
	}
	return false, err
}

// PublishRequest publishes on a topic with the aggregation request as the content.
func PublishRequest(ctx context.Context, client *pubsub.Client, pubsubTopic string, content interface{}) error {
	topic := client.Topic(pubsubTopic)

	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	log.Infof("topic: %s; request: %s", pubsubTopic, string(b))

	_, err = topic.Publish(ctx, &pubsub.Message{Data: b}).Get(ctx)
	return err
}

// ParsePubSubResourceName parses the PubSub resource name and get the project ID and topic or subscription.
//
// Details about the resource names: https://cloud.google.com/pubsub/docs/admin#resource_names
func ParsePubSubResourceName(name string) (projectID, relativeName string, err error) {
	strs := strings.Split(name, "/")
	if len(strs) != 4 || strs[0] != "projects" || (strs[2] != "subscriptions" && strs[2] != "topics") {
		err = fmt.Errorf("expect format %s, got %s", "projects/project-identifier/collection/relative-name", name)
		return
	}
	projectID, relativeName = strs[1], strs[3]
	return
}
