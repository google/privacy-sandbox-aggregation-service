module github.com/google/privacy-sandbox-aggregation-service

go 1.16

require (
	cloud.google.com/go/firestore v1.9.0
	cloud.google.com/go/profiler v0.3.0
	cloud.google.com/go/pubsub v1.30.0
	cloud.google.com/go/secretmanager v1.10.0
	cloud.google.com/go/storage v1.29.0
	github.com/apache/beam v2.32.0-RC1+incompatible
	github.com/bazelbuild/rules_go v0.42.0
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/glog v1.1.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.9
	github.com/google/tink/go v1.7.0
	github.com/grd/stat v0.0.0-20130623202159-138af3fd5012
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/pborman/uuid v1.2.1
	github.com/ugorji/go/codec v1.2.6
	golang.org/x/sync v0.1.0
	gonum.org/v1/gonum v0.11.0
	google.golang.org/api v0.114.0
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0
	lukechampine.com/uint128 v1.2.0
)
