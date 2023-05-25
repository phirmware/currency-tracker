# specify the base image to  be used for the application, alpine or ubuntu
FROM golang:1.17-alpine AS build

# create a working directory inside the image
WORKDIR /go/src/app

RUN apk add git

COPY . .


# Static build required so that we can safely copy the binary over.
# `-tags timetzdata` embeds zone info from the "time/tzdata" package.
RUN CGO_ENABLED=0 go install -ldflags '-extldflags "-static"' -tags timetzdata


# Final Build
FROM scratch


# the test program:
COPY --from=build /go/bin/currency-tracker /currency-tracker

# the tls certificates:
# NB: this pulls directly from the upstream image, which already has ca-certificates:
COPY --from=golang:1.17-alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["/currency-tracker"]
