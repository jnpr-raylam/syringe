
cat <<EOT >> ./cmd/syringed/buildinfo.go
package main

/*
This file is automatically generated by Syringe build scripts.
Please do not edit.
*/

const (
	BUILD_COMMIT = "$(git rev-parse HEAD)"
)
EOT