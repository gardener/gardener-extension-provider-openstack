############# builder
FROM --platform=$BUILDPLATFORM golang:1.26.0 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-openstack

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG EFFECTIVE_VERSION
ARG TARGETOS
ARG TARGETARCH

RUN make build GOOS=$TARGETOS GOARCH=$TARGETARCH EFFECTIVE_VERSION=$EFFECTIVE_VERSION BUILD_OUTPUT_FILE="/output/bin/"

############# base
FROM gcr.io/distroless/static-debian12:nonroot AS base

############# gardener-extension-provider-openstack
FROM base AS gardener-extension-provider-openstack
WORKDIR /

COPY --from=builder /output/bin/gardener-extension-provider-openstack /gardener-extension-provider-openstack
ENTRYPOINT ["/gardener-extension-provider-openstack"]

############# gardener-extension-admission-openstack
FROM base AS gardener-extension-admission-openstack
WORKDIR /

COPY --from=builder /output/bin/gardener-extension-admission-openstack /gardener-extension-admission-openstack
ENTRYPOINT ["/gardener-extension-admission-openstack"]
