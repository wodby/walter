FROM golang:1.27.1-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test -race ./... && go vet ./...
ARG VERSION=1.5.0
RUN VERSION=${VERSION} ./scripts/package
FROM scratch
COPY --from=build /src/dist/ /dist/
