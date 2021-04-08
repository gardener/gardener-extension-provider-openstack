############# builder
FROM eu.gcr.io/gardener-project/3rd/golang:1.16.2 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-openstack
COPY . .
RUN make install

############# base
FROM eu.gcr.io/gardener-project/3rd/alpine:3.13.2 AS base

############# gardener-extension-provider-openstack
FROM base AS gardener-extension-provider-openstack

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-openstack /gardener-extension-provider-openstack
ENTRYPOINT ["/gardener-extension-provider-openstack"]

############# gardener-extension-validator-openstack
FROM base AS gardener-extension-validator-openstack

COPY --from=builder /go/bin/gardener-extension-validator-openstack /gardener-extension-validator-openstack
ENTRYPOINT ["/gardener-extension-validator-openstack"]
