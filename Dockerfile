############# builder
FROM golang:1.21.1 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-openstack
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM gcr.io/distroless/static-debian11:nonroot AS base

############# gardener-extension-provider-openstack
FROM base AS gardener-extension-provider-openstack
WORKDIR /

COPY --from=builder /go/bin/gardener-extension-provider-openstack /gardener-extension-provider-openstack
ENTRYPOINT ["/gardener-extension-provider-openstack"]

############# gardener-extension-admission-openstack
FROM base as gardener-extension-admission-openstack
WORKDIR /

COPY --from=builder /go/bin/gardener-extension-admission-openstack /gardener-extension-admission-openstack
ENTRYPOINT ["/gardener-extension-admission-openstack"]
