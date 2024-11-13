module github.com/google/privacy-sandbox-aggregation-service

go 1.16

require (
	cloud.google.com/go/firestore v1.6.1
	cloud.google.com/go/profiler v0.3.0
	cloud.google.com/go/pubsub v1.22.2
	cloud.google.com/go/secretmanager v1.5.0
	cloud.google.com/go/storage v1.22.1
	github.com/apache/beam v2.32.0-RC1+incompatible
	github.com/bazelbuild/rules_go v0.48.1
	github.com/golang/glog v0.0.0-20210429001901-424d2337a529
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.8
	github.com/google/tink/go v1.7.0
	github.com/google/uuid v1.3.0 // indirect
	github.com/grd/stat v0.0.0-20130623202159-138af3fd5012
	github.com/hashicorp/go-retryablehttp v0.7.7
	github.com/pborman/uuid v1.2.1
	github.com/ugorji/go/codec v1.2.6
	golang.org/x/sync v0.5.0
	gonum.org/v1/gonum v0.8.2
	google.golang.org/api v0.85.0
	google.golang.org/genproto v0.0.0-20220617124728-180714bec0ad
	google.golang.org/protobuf v1.31.0
	lukechampine.com/uint128 v1.1.1
)
