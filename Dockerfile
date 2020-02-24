############# builder
FROM golang:1.13.8 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-openstack
COPY . .
RUN make install-requirements && make VERIFY=true all

############# gardener-extension-provider-openstack
FROM alpine:3.11.3 AS gardener-extension-provider-openstack

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-openstack /gardener-extension-provider-openstack
ENTRYPOINT ["/gardener-extension-provider-openstack"]
