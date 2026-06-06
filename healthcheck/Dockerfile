FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/healthcheck .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/healthcheck /healthcheck
EXPOSE 8080
ENTRYPOINT ["/healthcheck"]
