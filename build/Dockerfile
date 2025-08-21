FROM quay.io/app-sre/boilerplate:image-v5.0.1 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN make go-build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM  registry.access.redhat.com/ubi8/ubi-minimal:8.10-1755695263
ENV USER_UID=1001 \
    USER_NAME=hypershift-logging-operator

COPY --from=builder /workspace/build/_output/bin/* /usr/local/bin/

COPY build/bin /usr/local/bin
RUN /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/hypershift-logging-operator"]

USER ${USER_UID}

LABEL io.openshift.managed.name="hypershift-logging-operator" \
      io.openshift.managed.description="hypershift logging Operator"
