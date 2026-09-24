module github.com/eliasmeireles/gomailer/client/kafka

go 1.26.0

require (
	github.com/eliasmeireles/gomailer/client v0.2.0
	github.com/stretchr/testify v1.12.1
	github.com/twmb/franz-go v1.22.0
	github.com/twmb/franz-go/pkg/kfake v0.0.0-20260923172636-a5c3af0cdb3d
)

require (
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.30 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.14.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace github.com/eliasmeireles/gomailer/client => ../
