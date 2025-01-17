FROM registry.ci.openshift.org/open-cluster-management/builder:go1.16-linux AS builder
WORKDIR /go/src/github.com/open-cluster-management/multicloud-operators-foundation
COPY . .
ENV GO_PACKAGE github.com/open-cluster-management/multicloud-operators-foundation

RUN make build --warn-undefined-variables
RUN make build-e2e --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV USER_UID=10001 \
    USER_NAME=acm-foundation

COPY --from=builder /go/src/github.com/open-cluster-management/multicloud-operators-foundation/proxyserver /
COPY --from=builder /go/src/github.com/open-cluster-management/multicloud-operators-foundation/controller /
COPY --from=builder /go/src/github.com/open-cluster-management/multicloud-operators-foundation/webhook /
COPY --from=builder /go/src/github.com/open-cluster-management/multicloud-operators-foundation/agent /
COPY --from=builder /go/src/github.com/open-cluster-management/multicloud-operators-foundation/e2e.test /

RUN microdnf update && \
    microdnf clean all

USER ${USER_UID}
