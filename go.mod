module go.bryk.io/covid-tracking

go 1.14

require (
	github.com/gogo/googleapis v1.3.2
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.5
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/grpc-gateway v1.13.0
	github.com/mwitkow/go-proto-validators v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.6.3
	go.bryk.io/x v0.0.0-20200510004120-4d614c0921e7
	go.mongodb.org/mongo-driver v1.3.2
	golang.org/x/crypto v0.0.0-20200406173513-056763e48d71
	google.golang.org/grpc v1.28.1
)

replace github.com/cloudflare/cfssl => github.com/bryk-io/cfssl v0.0.0-20191204191638-bb9c164a4cb1
