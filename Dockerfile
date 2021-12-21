############# builder
FROM golang:1.17.5 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-openstack
COPY . .
RUN make install

############# base
FROM alpine:3.13.7 AS base

############# gardener-extension-provider-openstack
FROM base AS gardener-extension-provider-openstack

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-openstack /gardener-extension-provider-openstack
ENTRYPOINT ["/gardener-extension-provider-openstack"]

############# gardener-extension-admission-openstack
FROM base as gardener-extension-admission-openstack

COPY --from=builder /go/bin/gardener-extension-admission-openstack /gardener-extension-admission-openstack
ENTRYPOINT ["/gardener-extension-admission-openstack"]
