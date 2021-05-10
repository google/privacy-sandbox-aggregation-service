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

// This binary sets up the crypto key pairs for the browser simulator and helper servers.
package main

import (
	"context"
	"flag"
	"os"
	"path"

	log "github.com/golang/glog"
	"google.golang.org/grpc"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"

	cryptopb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
	grpcpb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
	servicepb "github.com/google/privacy-sandbox-aggregation-service/service/service_go_grpc_proto"
)

var (
	helperAddr1 = flag.String("helper_addr1", "", "Address of helper 1 server.")
	helperAddr2 = flag.String("helper_addr2", "", "Address of helper 2 server.")

	publicKeyDir1 = flag.String("public_key_dir1", "", "Directory where the helper 1 public key is stored.")
	publicKeyDir2 = flag.String("public_key_dir2", "", "Directory where the helper 2 public key is stored.")
)

func createCryptoKeys(ctx context.Context, client grpcpb.AggregatorClient, publicKeyDir string) (*cryptopb.ElGamalPublicKey, error) {
	r, err := client.CreateCryptoKeys(ctx, &servicepb.CreateCryptoKeysRequest{})
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(publicKeyDir, 0755)
	if err != nil {
		return nil, err
	}

	if err := cryptoio.SaveStandardPublicKey(path.Join(publicKeyDir, cryptoio.DefaultStandardPublicKey), r.StandardPublicKey); err != nil {
		return nil, err
	}
	if err := cryptoio.SaveElGamalPublicKey(path.Join(publicKeyDir, cryptoio.DefaultElgamalPublicKey), r.ElgamalPublicKey); err != nil {
		return nil, err
	}

	return r.ElgamalPublicKey, nil
}

func main() {
	flag.Parse()

	conn1, err := grpc.Dial(*helperAddr1, nil)
	if err != nil {
		log.Exit(err)
	}
	defer conn1.Close()
	client1 := grpcpb.NewAggregatorClient(conn1)

	conn2, err := grpc.Dial(*helperAddr2, nil)
	if err != nil {
		log.Exit(err)
	}
	defer conn2.Close()
	client2 := grpcpb.NewAggregatorClient(conn2)

	ctx := context.Background()
	ePub1, err := createCryptoKeys(ctx, client1, *publicKeyDir1)
	if err != nil {
		log.Exit(err)
	}
	ePub2, err := createCryptoKeys(ctx, client2, *publicKeyDir2)
	if err != nil {
		log.Exit(err)
	}

	if _, err := client1.SaveOtherHelperInfo(ctx, &servicepb.SaveOtherHelperInfoRequest{ElgamalPublicKey: ePub2}); err != nil {
		log.Exit(err)
	}
	if _, err := client2.SaveOtherHelperInfo(ctx, &servicepb.SaveOtherHelperInfoRequest{ElgamalPublicKey: ePub1}); err != nil {
		log.Exit(err)
	}
}
